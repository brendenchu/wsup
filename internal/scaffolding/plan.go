package scaffolding

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/brendenchu/wsup/internal/registry"
)

// Action describes a proposed same-path change. Planning never applies it.
type Action struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

// UpdatePlan is a read-only managed-content preview.
type UpdatePlan struct {
	FromVersion string   `json:"from_version"`
	ToVersion   string   `json:"to_version"`
	Actions     []Action `json:"actions"`
	Conflicts   []string `json:"conflicts"`
	Applied     bool     `json:"applied"`
}

// PlanUpdate compares the lock with an already selected template bundle and
// the installed modules. No wsup.json, reports, unknown files, or orphan files
// are overwritten.
func PlanUpdate(root, version string, files map[string][]byte, keep []string) (UpdatePlan, error) {
	return planUpdate(root, version, files, nil, nil, keep, nil)
}

// planUpdate plans an update to files and links. A nil links or modules keeps
// the lock's. prepared, when given, receives what apply needs.
func planUpdate(root, version string, files map[string][]byte, links, modules map[string]string, keep []string, prepared *preparedUpdate) (UpdatePlan, error) {
	return planScoped(root, version, files, links, modules, keep, nil, prepared)
}

// planScoped is planUpdate limited to the paths scope accepts; a nil scope
// accepts every path. A scoped plan leaves every other path, the links, the
// modules, and the recorded wsup version as the lock has them.
func planScoped(root, version string, files map[string][]byte, links, modules map[string]string, keep []string, scope func(string) bool, prepared *preparedUpdate) (UpdatePlan, error) {
	if files == nil {
		return UpdatePlan{}, fmt.Errorf("native template bundle is unavailable")
	}
	loaded, err := LoadLock(root)
	if err != nil {
		return UpdatePlan{}, err
	}
	lock := loaded.Lock
	if links == nil {
		links = lock.Links
	}
	if modules == nil {
		modules = lock.Modules
	}
	for name := range files {
		if !portableFile(name) {
			return UpdatePlan{}, fmt.Errorf("invalid template bundle path")
		}
	}
	if scope != nil {
		version, links, modules = lock.WsupVersion, lock.Links, lock.Modules
	}
	proposed := newLock(version, lock.Starter, modules, files, links)
	if scope != nil {
		for name := range proposed.Files {
			if !scope(name) {
				delete(proposed.Files, name)
			}
		}
		for name, digest := range lock.Files {
			if !scope(name) {
				proposed.Files[name] = digest
			}
		}
		for key := range proposed.Blocks {
			if file, _, _ := strings.Cut(key, "#"); !scope(file) {
				delete(proposed.Blocks, key)
			}
		}
		for key, digest := range lock.Blocks {
			if file, _, _ := strings.Cut(key, "#"); !scope(file) {
				proposed.Blocks[key] = digest
			}
		}
	}
	if _, err := EncodeLock(proposed); err != nil {
		return UpdatePlan{}, fmt.Errorf("invalid native template bundle: %w", err)
	}
	resolved, err := registry.Resolve(root)
	if err != nil {
		return UpdatePlan{}, err
	}
	root = resolved
	handle, err := os.OpenRoot(root)
	if err != nil {
		return UpdatePlan{}, err
	}
	defer handle.Close()

	kept := make(map[string]bool, len(keep))
	for _, name := range keep {
		if _, exists := lock.Files[name]; !exists || !portableFile(name) {
			return UpdatePlan{}, fmt.Errorf("--keep requires an existing same-path managed file: %s", name)
		}
		kept[name] = true
	}
	names := slices.Concat(slices.Collect(maps.Keys(lock.Files)), slices.Collect(maps.Keys(proposed.Files)))
	slices.Sort(names)
	names = slices.Compact(names)
	if scope != nil {
		names = slices.DeleteFunc(names, func(name string) bool { return !scope(name) })
	}
	result := UpdatePlan{
		FromVersion: lock.WsupVersion, ToVersion: version,
		Actions: []Action{}, Conflicts: []string{},
	}
	observed := make(map[string]observation, len(names))
	merged := map[string][]byte{}
	for _, name := range names {
		current, err := observeFile(handle, name)
		if err != nil {
			return UpdatePlan{}, err
		}
		observed[name] = current
		if kept[name] && !current.regular {
			return UpdatePlan{}, fmt.Errorf("--keep requires an existing same-path managed file: %s", name)
		}
		if current.exists && !current.regular {
			result.Conflicts = append(result.Conflicts, name)
			continue
		}
		action := ""
		baseline, managed := lock.Files[name]
		target, supplied := proposed.Files[name]
		switch {
		case kept[name]:
			action = "keep-customization"
		case !supplied && managed && catalogue.Generated(name):
			// A generated path that followed workspace data (a rename) is
			// replaced: removed while unedited, a conflict once edited.
			switch {
			case !current.exists:
				continue
			case current.digest == baseline:
				action = "remove"
			default:
				result.Conflicts = append(result.Conflicts, name)
				continue
			}
		case !supplied:
			action = "retain-orphan"
		case current.regular && current.digest == target:
			continue
		case !current.exists && !managed:
			action = "write"
		case current.regular && managed && current.digest == baseline:
			action = "write"
		case current.regular && managed:
			// An edited file whose generated sections are as wsup.lock recorded
			// them takes the new sections and keeps every owner byte.
			body, err := handle.ReadFile(name)
			if err != nil {
				return UpdatePlan{}, err
			}
			spliced, ok := spliceRecorded(body, files[name], recordedBlocks(lock, name))
			switch {
			case !ok || lockDigest(body) != current.digest:
				result.Conflicts = append(result.Conflicts, name)
				continue
			case bytes.Equal(spliced, body):
				continue
			}
			action = "merge"
			merged[name] = spliced
		default:
			result.Conflicts = append(result.Conflicts, name)
			continue
		}
		result.Actions = append(result.Actions, Action{Path: name, Action: action})
	}
	// Links are re-created when missing. Anything else at a link path (a real
	// directory, or a link pointing elsewhere) is reported and left alone. A
	// scoped plan leaves links to update.
	for _, name := range slices.Sorted(maps.Keys(proposed.Links)) {
		if scope != nil {
			break
		}
		status, err := linkStatus(handle, name, proposed.Links[name])
		if err != nil {
			return UpdatePlan{}, err
		}
		switch status {
		case linkOK:
		case linkMissing:
			result.Actions = append(result.Actions, Action{Path: name, Action: "link"})
		default:
			result.Conflicts = append(result.Conflicts, name)
		}
	}
	if prepared != nil {
		// A merged file keeps its baseline: its owner bytes are still edits.
		for name := range merged {
			proposed.Files[name] = lock.Files[name]
		}
		// A kept file keeps its baseline and its blocks' baselines.
		for name := range kept {
			if _, exists := proposed.Files[name]; !exists {
				continue
			}
			proposed.Files[name] = lock.Files[name]
			for key := range proposed.Blocks {
				if strings.HasPrefix(key, name+"#") {
					delete(proposed.Blocks, key)
				}
			}
			for key, digest := range lock.Blocks {
				if strings.HasPrefix(key, name+"#") {
					proposed.Blocks[key] = digest
				}
			}
		}
		body, err := EncodeLock(proposed)
		if err != nil {
			return UpdatePlan{}, err
		}
		bodies := make(map[string][]byte)
		for _, action := range result.Actions {
			switch action.Action {
			case "write":
				bodies[action.Path] = bytes.Clone(files[action.Path])
			case "merge":
				bodies[action.Path] = merged[action.Path]
			}
		}
		*prepared = preparedUpdate{
			root: root, plan: result, observed: observed,
			lockBefore: loaded.Original, lockAfter: body, files: bodies, links: proposed.Links,
		}
	}
	return result, nil
}

type observation struct {
	exists  bool
	regular bool
	digest  string
}

// Reject symlink components even when they resolve inside the workspace.
// Rooted reads also prevent an exchanged ancestor from escaping confinement;
// this is point-in-time planning, not an apply-time concurrency guarantee.
func observeFile(root *os.Root, name string) (observation, error) {
	parts := strings.Split(name, "/")
	for i := range parts {
		component := path.Join(parts[:i+1]...)
		info, err := root.Lstat(component)
		if os.IsNotExist(err) {
			return observation{}, nil
		}
		if err != nil {
			return observation{}, fmt.Errorf("inspect template path %s: %w", name, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return observation{}, fmt.Errorf("template operation refuses symlink destination: %s", name)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return observation{}, fmt.Errorf("template parent is not a directory: %s", name)
		}
		if i == len(parts)-1 && !info.Mode().IsRegular() {
			return observation{exists: true}, nil
		}
	}
	file, err := root.Open(name)
	if err != nil {
		return observation{}, fmt.Errorf("read template file %s: %w", name, err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return observation{}, fmt.Errorf("hash template file %s: %w", name, err)
	}
	return observation{exists: true, regular: true, digest: fmt.Sprintf("sha256:%x", hash.Sum(nil))}, nil
}

// recordedBlocks returns wsup.lock's section digests for one file, keyed by
// section name.
func recordedBlocks(lock Lock, name string) map[string]string {
	recorded := map[string]string{}
	for key, digest := range lock.Blocks {
		if section, ok := strings.CutPrefix(key, name+"#"); ok {
			recorded[section] = digest
		}
	}
	return recorded
}
