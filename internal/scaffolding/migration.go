package scaffolding

import (
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/brendenchu/wsup/internal/registry"
)

// MigrationPlan is a read-only proposal, never permission to remove files.
// A future apply must revalidate original state, content and tree inventories.
type MigrationPlan struct {
	FromVersion string            `json:"from_version"`
	ToVersion   string            `json:"to_version"`
	FromSchema  int               `json:"from_schema"`
	ToSchema    int               `json:"to_schema"`
	Actions     []MigrationAction `json:"actions"`
	Conflicts   []string          `json:"conflicts"`
	Applied     bool              `json:"applied"`
}

// PlanMigration explicitly previews Python-to-native ownership changes.
// Runtime retirement is restricted to exact manifest-owned unchanged files.
// Unknown files in retiring trees block migration rather than being swept up.
func PlanMigration(root, version string, keep []string) (MigrationPlan, error) {
	loaded, err := LoadState(root)
	if err != nil {
		return MigrationPlan{}, err
	}
	if loaded.State.Version != 1 {
		return MigrationPlan{}, fmt.Errorf("native migration requires Python schema-1 state; use ordinary update for native state")
	}
	root, err = registry.Resolve(root)
	if err != nil {
		return MigrationPlan{}, err
	}
	handle, err := os.OpenRoot(root)
	if err != nil {
		return MigrationPlan{}, err
	}
	defer handle.Close()
	result := MigrationPlan{
		FromVersion: loaded.State.TemplateVersion, ToVersion: version, FromSchema: 1, ToSchema: StateVersion,
		Actions: []MigrationAction{}, Conflicts: []string{},
	}
	files, err := catalogue.BundleSelected(loaded.State.Template, loaded.State.Collections)
	if err != nil {
		return MigrationPlan{}, err
	}
	native := loaded.State
	native.Version = StateVersion
	native.Files = map[string]string{}
	names := make([]string, 0, len(loaded.State.Files))
	for name := range loaded.State.Files {
		names = append(names, name)
	}
	slices.Sort(names)
	kept := map[string]bool{}
	for _, name := range keep {
		kept[name] = true
	}
	for _, name := range names {
		destination, retire := migrationTarget(name)
		if kept[name] && (retire || destination != name) {
			return MigrationPlan{}, fmt.Errorf("--keep cannot bypass runtime retirement or relocation: %s", name)
		}
		if !retire && destination == name {
			native.Files[name] = loaded.State.Files[name]
			continue
		}
		current, err := observeFile(handle, name)
		if err != nil {
			return MigrationPlan{}, err
		}
		if !current.regular || current.digest != loaded.State.Files[name] {
			result.Conflicts = append(result.Conflicts, name)
			continue
		}
		if retire {
			result.Actions = append(result.Actions, MigrationAction{Path: name, Action: "retire-runtime"})
			continue
		}
		target, err := observeFile(handle, destination)
		if err != nil {
			return MigrationPlan{}, err
		}
		_, managed := loaded.State.Files[destination]
		if _, supplied := files[destination]; !supplied || target.exists || managed {
			result.Conflicts = append(result.Conflicts, destination)
			continue
		}
		result.Actions = append(result.Actions, MigrationAction{Path: destination, Action: "relocate", From: name})
	}
	// Walk only exact retiring installation/legacy guidance trees, never tools/
	// as a whole. Symlinks are rejected without traversing their targets.
	for _, prefix := range []string{
		".wsup/wsup_core", ".wsup/templates", "tools/wsup_core",
		"tools/wsup_templates", "templates/project", ".ai/rules", ".ai/skills",
	} {
		info, err := handle.Lstat(prefix)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return MigrationPlan{}, err
		}
		if !info.IsDir() {
			return MigrationPlan{}, fmt.Errorf("migration tree must be a real directory: %s", prefix)
		}
		err = fs.WalkDir(handle.FS(), prefix, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("migration refuses symlink: %s", name)
			}
			if entry.IsDir() {
				return nil
			}
			if _, exists := loaded.State.Files[name]; !exists {
				result.Conflicts = append(result.Conflicts, name)
			}
			return nil
		})
		if err != nil {
			return MigrationPlan{}, err
		}
	}
	for _, name := range []string{".wsup/wsup.py", "tools/wsup.py"} {
		current, err := observeFile(handle, name)
		if err != nil {
			return MigrationPlan{}, err
		}
		if _, managed := loaded.State.Files[name]; current.exists && !managed {
			result.Conflicts = append(result.Conflicts, name)
		}
	}
	// Legacy report relocation/settings need their own byte-preserving planning
	// and apply checks. Block explicitly until that surface is implemented.
	if info, err := handle.Lstat(".work/quality"); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return MigrationPlan{}, fmt.Errorf("migration refuses symlinked legacy reports")
		}
		result.Conflicts = append(result.Conflicts, ".work/quality")
	} else if !os.IsNotExist(err) {
		return MigrationPlan{}, err
	}
	for _, name := range []string{"wsup.json", "wsup.local.json"} {
		current, err := observeFile(handle, name)
		if err != nil {
			return MigrationPlan{}, err
		}
		// Config stays owner-owned. Until legacy report-policy interpretation is
		// implemented, require review instead of promising a usable migration.
		if current.exists {
			result.Conflicts = append(result.Conflicts, name)
		}
	}
	preview, err := planLoadedUpdate(root, version, files, keep, LoadedState{State: native, Original: loaded.Original}, nil)
	if err != nil {
		return MigrationPlan{}, err
	}
	for _, action := range preview.Actions {
		relocated := slices.ContainsFunc(result.Actions, func(item MigrationAction) bool { return item.Action == "relocate" && item.Path == action.Path })
		if !relocated {
			result.Actions = append(result.Actions, MigrationAction{Path: action.Path, Action: action.Action})
		}
	}
	result.Conflicts = append(result.Conflicts, preview.Conflicts...)
	slices.Sort(result.Conflicts)
	result.Conflicts = slices.Compact(result.Conflicts)
	slices.SortFunc(result.Actions, func(a, b MigrationAction) int { return strings.Compare(a.Path, b.Path) })
	// Recheck exact source state before returning a schema-selection-based plan.
	current, err := LoadState(root)
	if err != nil {
		return MigrationPlan{}, err
	}
	if string(current.Original) != string(loaded.Original) {
		return MigrationPlan{}, fmt.Errorf("template state changed during migration planning")
	}
	return result, nil
}

func migrationTarget(name string) (string, bool) {
	for _, prefix := range []string{".wsup/wsup_core/", ".wsup/templates/", "tools/wsup_core/", "tools/wsup_templates/", "templates/project/"} {
		if strings.HasPrefix(name, prefix) {
			return "", true
		}
	}
	if name == ".wsup/wsup.py" || name == "tools/wsup.py" {
		return "", true
	}
	for _, mapping := range []struct{ old, next string }{{".ai/rules/", "guidance/rules/"}, {".ai/skills/", ".agents/skills/"}} {
		if strings.HasPrefix(name, mapping.old) {
			return mapping.next + strings.TrimPrefix(name, mapping.old), false
		}
	}
	return name, false
}
