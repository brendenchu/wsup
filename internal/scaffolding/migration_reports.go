package scaffolding

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/brendenchu/wsup/internal/configuration"
)

// Report history is evidence, not template content. Moves retain the original
// bytes/provenance and can never be kept away or merged with a destination.
func planMigrationReports(root string, handle *os.Root, managed map[string]string, bundle map[string][]byte, result *MigrationPlan, observed map[string]observation) error {
	if _, err := configuration.ReportDirectory(root, ".wsup/reports"); err != nil {
		return err
	}
	for _, name := range []string{"wsup.json", "wsup.local.json"} {
		current, err := observeFile(handle, name)
		if err != nil {
			return err
		}
		observed[name] = current
		if !current.exists {
			continue
		}
		if !current.regular {
			result.Conflicts = append(result.Conflicts, name)
			continue
		}
		body, err := handle.ReadFile(name)
		if err != nil {
			return err
		}
		if contentDigest(body) != current.digest {
			return fmt.Errorf("configuration changed during migration planning: %s", name)
		}
		settings, err := configuration.ParseSettings(name, body)
		if err != nil {
			return err
		}
		if report, exists := settings["report_dir"]; exists {
			directory, ok := report.(string)
			if !ok {
				return fmt.Errorf("%s: report_dir must be a relative path", name)
			}
			if strings.HasPrefix(directory, ".work") {
				// Inspect both files separately: a local override must not mask
				// a legacy shared policy that can become active later.
				result.Conflicts = append(result.Conflicts, name)
				continue
			}
			if _, err := configuration.ReportDirectory(root, directory); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
	}
	// Do not read ambient overrides: migration describes persistent local policy,
	// not whether this process's environment happens to hide an unsafe setting.
	configConflict := false
	for _, name := range result.Conflicts {
		if name == "wsup.json" || name == "wsup.local.json" {
			configConflict = true
		}
	}
	if !configConflict {
		if _, err := configuration.Load(root, nil, func(string) (string, bool) { return "", false }); err != nil {
			return err
		}
	}
	const prefix = ".work/quality"
	if _, err := observeFile(handle, prefix); err != nil {
		return err
	}
	info, err := handle.Lstat(prefix)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("legacy report root must be a real directory")
		}
		err = fs.WalkDir(handle.FS(), prefix, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("migration refuses symlinked report: %s", name)
			}
			if entry.IsDir() {
				return nil
			}
			source, err := observeFile(handle, name)
			if err != nil {
				return err
			}
			if !source.regular {
				return fmt.Errorf("legacy report must be a regular file: %s", name)
			}
			destination := ".wsup/reports/" + strings.TrimPrefix(name, prefix+"/")
			target, err := observeFile(handle, destination)
			if err != nil {
				return err
			}
			observed[name], observed[destination] = source, target
			_, owned := managed[name]
			_, destinationOwned := managed[destination]
			_, supplied := bundle[destination]
			if target.exists || owned || destinationOwned || supplied {
				result.Conflicts = append(result.Conflicts, destination)
			} else {
				result.Actions = append(result.Actions, MigrationAction{Path: destination, Action: "move-report", From: name})
			}
			return nil
		})
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	for name, expected := range observed {
		current, err := observeFile(handle, name)
		if err != nil {
			return err
		}
		if current != expected {
			return fmt.Errorf("report or configuration changed during migration planning: %s", name)
		}
	}
	return nil
}
