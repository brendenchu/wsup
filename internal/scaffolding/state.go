// Package scaffolding validates managed template state before planning filesystem changes.
package scaffolding

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/brendenchu/wsup/internal/registry"
)

const (
	StatePath = ".wsup/template-state.json"
	// StateVersion identifies the managed-content schema.
	StateVersion = 2
)

var (
	collections = catalogue.CollectionNames()
	// defaultHomes are the home files v0.2.0 installed in every workspace and
	// the only paths an owner could record as omitted.
	defaultHomes  = []string{"apps/.gitkeep", "infrastructure/.gitkeep", "packages/README.md", "tools/README.md", "workbench/README.md"}
	digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	// retiredCollections maps v0.2.0 collection names to their replacement;
	// an empty replacement means the collection was removed. Converting such a
	// state is a migration, which this version does not perform.
	retiredCollections = map[string]string{"artifacts": "releases", "private": ""}
)

// State retains baseline hashes, not current content hashes. A customization
// must never be adopted merely by reading or re-encoding this state.
type State struct {
	Version          int               `json:"version"`
	TemplateVersion  string            `json:"template_version"`
	Template         string            `json:"template"`
	Files            map[string]string `json:"files"`
	Collections      []string          `json:"collections,omitempty"`
	OmittedHomeFiles []string          `json:"omitted_home_files,omitempty"`
}

// LoadedState carries exact original bytes for a later apply-time change check.
type LoadedState struct {
	State    State
	Original []byte
}

// LoadState validates only metadata and paths; missing or customized managed
// files are left for the planner to classify without changing any bytes.
func LoadState(root string) (LoadedState, error) {
	root, err := registry.Resolve(root)
	if err != nil {
		return LoadedState{}, err
	}
	filename := filepath.Join(root, filepath.FromSlash(StatePath))
	for _, name := range []string{filepath.Join(root, ".wsup"), filename} {
		info, err := os.Lstat(name)
		if err != nil {
			return LoadedState{}, fmt.Errorf("inspect template state: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return LoadedState{}, fmt.Errorf("template state refuses symlink destinations")
		}
	}
	body, err := os.ReadFile(filename)
	if err != nil {
		return LoadedState{}, err
	}
	state, err := ParseState(body)
	if err != nil {
		return LoadedState{}, err
	}
	return LoadedState{State: state, Original: body}, nil
}

// EncodeNativeState serializes only supported state without changing its schema.
func EncodeNativeState(state State) ([]byte, error) {
	if state.Version != StateVersion {
		return nil, fmt.Errorf("encoding template state requires version %d", StateVersion)
	}
	body, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	checked, err := ParseState(body)
	if err != nil {
		return nil, err
	}
	body, err = json.MarshalIndent(checked, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

// ParseState accepts only the current schema. It does not convert schemas,
// normalize ownership hashes, or inspect managed file contents.
func ParseState(body []byte) (State, error) {
	if !utf8.Valid(body) {
		return State{}, fmt.Errorf("template state must be UTF-8")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return State{}, fmt.Errorf("invalid template state JSON: %w", err)
	}
	if raw == nil {
		return State{}, fmt.Errorf("unsupported template state fields")
	}
	for name := range raw {
		if !slices.Contains([]string{"version", "template_version", "template", "files", "collections", "omitted_home_files"}, name) {
			return State{}, fmt.Errorf("unsupported template state field %s", name)
		}
	}
	var state State
	if err := json.Unmarshal(body, &state); err != nil {
		return State{}, fmt.Errorf("invalid template state fields: %w", err)
	}
	if state.Version != StateVersion {
		return State{}, fmt.Errorf("unsupported template state version %d; use a compatible wsup binary", state.Version)
	}
	if strings.TrimSpace(state.TemplateVersion) == "" ||
		(state.Template != "workspace" && state.Template != "project") || state.Files == nil {
		return State{}, fmt.Errorf("template state requires template_version, workspace/project template, and files")
	}
	for name, digest := range state.Files {
		if !portableFile(name) || name == StatePath || !digestPattern.MatchString(digest) {
			return State{}, fmt.Errorf("template state contains an invalid managed path or SHA-256 baseline")
		}
		if name == ".wsup" || strings.HasPrefix(name, ".wsup/") {
			return State{}, fmt.Errorf("native template state must not own runtime, catalogue, state, or reports")
		}
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if _, exists := state.Files[parent]; exists {
				return State{}, fmt.Errorf("template state managed paths contain a file/parent collision")
			}
		}
	}
	for _, field := range []struct {
		name    string
		values  *[]string
		allowed []string
	}{
		{"collections", &state.Collections, collections},
		{"omitted_home_files", &state.OmittedHomeFiles, defaultHomes},
	} {
		if value, exists := raw[field.name]; exists && string(value) == "null" {
			return State{}, fmt.Errorf("%s must be a list", field.name)
		}
		for _, value := range *field.values {
			if replacement, retired := retiredCollections[value]; retired && field.name == "collections" {
				if replacement == "" {
					return State{}, fmt.Errorf("collection %q was removed; this workspace needs a migration that this wsup does not provide", value)
				}
				return State{}, fmt.Errorf("collection %q is now %q; this workspace needs a migration that this wsup does not provide", value, replacement)
			}
			if !slices.Contains(field.allowed, value) {
				return State{}, fmt.Errorf("invalid %s selection", field.name)
			}
		}
		if len(*field.values) != 0 && state.Template != "workspace" {
			return State{}, fmt.Errorf("%s requires a workspace template", field.name)
		}
		slices.Sort(*field.values)
		*field.values = slices.Compact(*field.values)
	}
	return state, nil
}

// SelectedCollections returns the recorded collections plus every former
// default home this state still manages. Those homes moved from the workspace
// base into collections, so a state that predates the move selects them
// implicitly. Each home was a single file, so an omitted one means the owner
// removed the home and its collection stays unselected. The former
// infrastructure/ home has no same-named collection (it is now infra/) and is
// left to migration.
func (s State) SelectedCollections() []string {
	selected := slices.Clone(s.Collections)
	if s.Template != "workspace" {
		return selected
	}
	for _, name := range defaultHomes {
		home := strings.SplitN(name, "/", 2)[0]
		if _, managed := s.Files[name]; managed && slices.Contains(collections, home) {
			selected = append(selected, home)
		}
	}
	slices.Sort(selected)
	return slices.Compact(selected)
}

func portableFile(name string) bool {
	return utf8.ValidString(name) && name != "" && name != "." && path.Clean(name) == name &&
		!path.IsAbs(name) && !strings.ContainsAny(name, "\\:\x00") &&
		name != ".." && !strings.HasPrefix(name, "../")
}
