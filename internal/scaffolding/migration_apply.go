package scaffolding

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"reflect"
	"slices"
)

type preparedMigration struct {
	plan    MigrationPlan
	content preparedUpdate
	trees   map[string]observation
}

// ApplyMigration writes all required content and native state before retiring
// any exact unchanged legacy source. It is not a multi-file transaction. Errors
// explicitly distinguish uncommitted state from committed native state with
// incomplete retirement. Restore matched files/state from backup on failure.
func ApplyMigration(root, version string, keep []string) (MigrationPlan, error) {
	var prepared preparedMigration
	plan, err := planMigration(root, version, keep, &prepared)
	if err != nil {
		return plan, err
	}
	return prepared.applyWith(writeManaged, func(root *os.Root, name string) error { return root.Remove(name) })
}

func (p preparedMigration) applyWith(write func(*os.Root, string, []byte, bool) error, remove func(*os.Root, string) error) (MigrationPlan, error) {
	result := p.plan
	handle, err := os.OpenRoot(p.content.root)
	if err != nil {
		return result, err
	}
	defer handle.Close()
	if err := checkMigrationTrees(handle, p.trees); err != nil {
		return result, err
	}
	_, err = p.content.applyWith(func(root *os.Root, name string, body []byte, exclusive bool) error {
		if name == StatePath {
			if err := checkMigrationTrees(root, p.trees); err != nil {
				return err
			}
			// Recheck all inputs not intentionally replaced by this operation,
			// including standalone runtimes, kept files, config and registry.
			for name := range p.content.observed {
				if _, replaced := p.content.files[name]; replaced {
					continue
				}
				if err := p.content.recheck(root, name); err != nil {
					return err
				}
			}
		}
		return write(root, name, body, exclusive)
	})
	if err != nil {
		return result, err
	}
	fail := func(err error) (MigrationPlan, error) {
		return result, fmt.Errorf("native migration state committed but legacy retirement incomplete; native content is installed and some legacy sources may remain; restore matched files/state from backup before retrying: %w", err)
	}
	if err := checkMigrationTrees(handle, p.trees); err != nil {
		return fail(err)
	}
	// Verify committed state and every required output before deleting sources.
	current, err := observeFile(handle, StatePath)
	if err != nil {
		return fail(err)
	}
	if !current.regular || current.digest != contentDigest(p.content.stateAfter) {
		return fail(fmt.Errorf("native state changed before retirement"))
	}
	for name, body := range p.content.files {
		current, err := observeFile(handle, name)
		if err != nil {
			return fail(err)
		}
		if !current.regular || current.digest != contentDigest(body) {
			return fail(fmt.Errorf("native output changed before retirement: %s", name))
		}
	}
	removed := []string{}
	for _, action := range result.Actions {
		source := ""
		switch action.Action {
		case "retire-runtime":
			source = action.Path
		case "relocate", "move-report":
			source = action.From
		default:
			continue
		}
		state, err := observeFile(handle, StatePath)
		if err != nil {
			return fail(err)
		}
		if !state.regular || state.digest != contentDigest(p.content.stateAfter) {
			return fail(fmt.Errorf("native state changed during retirement"))
		}
		if action.Action == "move-report" || action.Action == "relocate" {
			output, err := observeFile(handle, action.Path)
			if err != nil {
				return fail(err)
			}
			if !output.regular || output.digest != contentDigest(p.content.files[action.Path]) {
				return fail(fmt.Errorf("migration destination changed during retirement: %s", action.Path))
			}
		}
		if err := p.content.recheck(handle, source); err != nil {
			return fail(err)
		}
		if err := remove(handle, source); err != nil {
			return fail(fmt.Errorf("retire %s: %w", source, err))
		}
		removed = append(removed, source)
	}
	// Remove only empty ancestors of exact removed sources. Never sweep trees
	// or remove a directory containing unknown/concurrently added owner files.
	for _, source := range removed {
		for directory := path.Dir(source); directory != "."; directory = path.Dir(directory) {
			info, err := handle.Lstat(directory)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return fail(err)
			}
			if !info.IsDir() {
				return fail(fmt.Errorf("retired ancestor changed: %s", directory))
			}
			child, err := handle.Open(directory)
			if err != nil {
				return fail(err)
			}
			entries, readErr := child.ReadDir(1)
			closeErr := child.Close()
			if closeErr != nil {
				return fail(closeErr)
			}
			if len(entries) != 0 {
				break
			}
			if readErr != nil && readErr != io.EOF {
				return fail(readErr)
			}
			if err := handle.Remove(directory); err != nil {
				return fail(err)
			}
		}
	}
	result.Applied = true
	return result, nil
}

func migrationTrees(root *os.Root) (map[string]observation, error) {
	trees := map[string]observation{}
	for _, prefix := range []string{
		".wsup/wsup_core", ".wsup/templates", "tools/wsup_core", "tools/wsup_templates",
		"templates/project", ".ai/rules", ".ai/skills", ".work/quality",
	} {
		if _, err := observeFile(root, prefix); err != nil {
			return nil, err
		}
		info, err := root.Lstat(prefix)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("migration tree must be a real directory: %s", prefix)
		}
		err = fs.WalkDir(root.FS(), prefix, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				trees[name] = observation{exists: true}
				return nil
			}
			current, err := observeFile(root, name)
			if err != nil {
				return err
			}
			if !current.regular {
				return fmt.Errorf("migration tree requires regular files: %s", name)
			}
			trees[name] = current
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return trees, nil
}

func checkMigrationTrees(root *os.Root, expected map[string]observation) error {
	current, err := migrationTrees(root)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current, expected) {
		names := make([]string, 0, len(current)+len(expected))
		for name := range current {
			names = append(names, name)
		}
		for name := range expected {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			if current[name] != expected[name] {
				return fmt.Errorf("migration tree changed during planning: %s", name)
			}
		}
		return fmt.Errorf("migration tree inventory changed during planning")
	}
	return nil
}
