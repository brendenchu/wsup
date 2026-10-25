package scaffolding

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/brendenchu/wsup/internal/registry"
)

// Action describes a proposed same-path change. Planning never applies it.
type Action struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

// UpdatePlan is a read-only native content preview, not a runtime migration.
type UpdatePlan struct {
	FromVersion string   `json:"from_version"`
	ToVersion   string   `json:"to_version"`
	Actions     []Action `json:"actions"`
	Conflicts   []string `json:"conflicts"`
	Applied     bool     `json:"applied"`
}

// PlanUpdate compares native state with an already selected template bundle.
// The caller supplies the matching template/collections; omissions are applied
// here. No registry, reports, unknown files, or orphan files are overwritten.
// Legacy state requires a separate explicit migration rather than this planner.
func PlanUpdate(root, version string, files map[string][]byte, keep []string) (UpdatePlan, error) {
	return planUpdate(root, version, files, keep, nil)
}

func planUpdate(root, version string, files map[string][]byte, keep []string, prepared *preparedUpdate) (UpdatePlan, error) {
	if files == nil {
		return UpdatePlan{}, fmt.Errorf("native template bundle is unavailable")
	}
	loaded, err := LoadState(root)
	if err != nil {
		return UpdatePlan{}, err
	}
	state := loaded.State
	if state.Version != StateVersion {
		return UpdatePlan{}, fmt.Errorf("Python template state requires explicit native migration, not a content update")
	}
	proposed := State{
		Version: StateVersion, TemplateVersion: version, Template: state.Template,
		Files:       make(map[string]string, len(files)),
		Collections: state.Collections, OmittedHomeFiles: state.OmittedHomeFiles,
	}
	for name, body := range files {
		if !portableFile(name) {
			return UpdatePlan{}, fmt.Errorf("invalid template bundle path")
		}
		proposed.Files[name] = contentDigest(body)
	}
	if _, err := EncodeNativeState(proposed); err != nil {
		return UpdatePlan{}, fmt.Errorf("invalid native template bundle: %w", err)
	}
	for _, name := range state.OmittedHomeFiles {
		delete(proposed.Files, name)
	}
	root, err = registry.Resolve(root)
	if err != nil {
		return UpdatePlan{}, err
	}
	handle, err := os.OpenRoot(root)
	if err != nil {
		return UpdatePlan{}, err
	}
	defer handle.Close()

	kept := make(map[string]bool, len(keep))
	for _, name := range keep {
		if _, exists := state.Files[name]; !exists || !portableFile(name) {
			return UpdatePlan{}, fmt.Errorf("--keep requires an existing same-path managed file: %s", name)
		}
		kept[name] = true
	}
	names := make([]string, 0, len(state.Files)+len(proposed.Files))
	for name := range state.Files {
		names = append(names, name)
	}
	for name := range proposed.Files {
		names = append(names, name)
	}
	slices.Sort(names)
	names = slices.Compact(names)
	result := UpdatePlan{
		FromVersion: state.TemplateVersion, ToVersion: version,
		Actions: []Action{}, Conflicts: []string{},
	}
	observed := make(map[string]observation, len(names))
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
		if name == "workspace.json" {
			// The registry belongs to the workspace even if its original starter
			// hash is present. Missing registries are not silently recreated.
			continue
		}
		action := ""
		baseline, managed := state.Files[name]
		target, supplied := proposed.Files[name]
		switch {
		case kept[name]:
			action = "keep-customization"
		case !supplied:
			action = "retain-orphan"
		case current.regular && current.digest == target:
			continue
		case !current.exists && !managed:
			action = "write"
		case current.regular && managed && current.digest == baseline:
			action = "write"
		default:
			result.Conflicts = append(result.Conflicts, name)
			continue
		}
		result.Actions = append(result.Actions, Action{Path: name, Action: action})
	}
	if prepared != nil {
		// Registry ownership and kept baseline hashes survive state replacement.
		delete(proposed.Files, "workspace.json")
		if digest, exists := state.Files["workspace.json"]; exists {
			proposed.Files["workspace.json"] = digest
		}
		for name := range kept {
			if _, exists := proposed.Files[name]; exists {
				proposed.Files[name] = state.Files[name]
			}
		}
		body, err := EncodeNativeState(proposed)
		if err != nil {
			return UpdatePlan{}, err
		}
		bodies := make(map[string][]byte)
		for _, action := range result.Actions {
			if action.Action == "write" {
				bodies[action.Path] = bytes.Clone(files[action.Path])
			}
		}
		*prepared = preparedUpdate{
			root: root, plan: result, observed: observed,
			stateBefore: loaded.Original, stateAfter: body, files: bodies,
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
	return observation{exists: true, regular: true, digest: fmt.Sprintf("%x", hash.Sum(nil))}, nil
}

func contentDigest(body []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(body))
}
