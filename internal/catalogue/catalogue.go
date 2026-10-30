// Package catalogue selects complete embedded native starters from the
// built-in module catalogue.
package catalogue

import (
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"

	"github.com/brendenchu/wsup/templates"
)

const (
	// DefaultPreset is the preset init applies when nothing else is selected.
	DefaultPreset = "development"
	// NoPreset selects the base plus explicit collections only. It is reserved
	// and can never be a module id.
	NoPreset = "none"
	// DefaultWorkspaceName names a new workspace in its wsup.json.
	DefaultWorkspaceName = "My workspace"
)

// mustLoad returns the embedded catalogue. It is validated by tests, so a load
// failure is a build defect and panics rather than silently offering nothing.
func mustLoad() *Catalogue {
	c, err := Load()
	if err != nil {
		panic("embedded module catalogue is invalid: " + err.Error())
	}
	return c
}

// CollectionNames returns a fresh ordered list of supported optional homes.
func CollectionNames() []string {
	return mustLoad().IDs(KindCollection)
}

// AgentNames returns a fresh ordered list of built-in agent adapters.
func AgentNames() []string {
	return mustLoad().IDs(KindAgent)
}

// EditorNames returns a fresh ordered list of built-in editor adapters.
func EditorNames() []string {
	return mustLoad().IDs(KindEditor)
}

// ComposeAdapters resolves an --agent or --editor selection for the adapter
// kind: no names means every adapter, a lone NoPreset ("none") means none,
// and anything else is validated, sorted, and deduplicated.
func ComposeAdapters(kind string, names []string) ([]string, error) {
	available := mustLoad().IDs(kind)
	if len(names) == 0 {
		return available, nil
	}
	if slices.Contains(names, NoPreset) {
		if slices.ContainsFunc(names, func(name string) bool { return name != NoPreset }) {
			return nil, fmt.Errorf("--%s %s cannot be combined with other %ss", kind, NoPreset, kind)
		}
		return []string{}, nil
	}
	selected := slices.Sorted(slices.Values(names))
	selected = slices.Compact(selected)
	for _, name := range selected {
		if !slices.Contains(available, name) {
			return nil, fmt.Errorf("unknown %s %q; select from %s, or %s", kind, name, strings.Join(available, ", "), NoPreset)
		}
	}
	return selected, nil
}

// PresetNames returns a fresh ordered list of built-in presets.
func PresetNames() []string {
	return mustLoad().IDs(KindPreset)
}

// ComposeCollections expands a preset (or NoPreset) and adds explicit
// collections, returning a validated, sorted, deduplicated selection.
func ComposeCollections(preset string, collections []string) ([]string, error) {
	members := []string{}
	if preset != NoPreset {
		module, ok := mustLoad().Module(preset)
		if !ok || module.Kind != KindPreset {
			return nil, fmt.Errorf("unknown preset %q; select from %s", preset, strings.Join(append(PresetNames(), NoPreset), ", "))
		}
		members = module.Modules
	}
	return SelectCollections(append(slices.Clone(members), collections...))
}

// SelectCollections validates, sorts, and deduplicates explicit selections.
func SelectCollections(names []string) ([]string, error) {
	selected := append([]string{}, names...)
	slices.Sort(selected)
	selected = slices.Compact(selected)
	for _, name := range selected {
		if !slices.Contains(CollectionNames(), name) {
			return nil, fmt.Errorf("unknown collection %q; select from %s", name, strings.Join(CollectionNames(), ", "))
		}
	}
	return selected, nil
}

// Bundle returns starter bytes from the native template authority.
// Neither workspace customizations nor checkout files are read at runtime.
func Bundle(kind string) (map[string][]byte, error) {
	return BundleSelected(kind, nil)
}

// BundleSelected includes only explicitly selected collection scaffolds.
func BundleSelected(kind string, collections []string) (map[string][]byte, error) {
	files, _, err := Starter(kind, Selection{Collections: collections})
	return files, err
}

// Selection names everything a starter installs beyond its base. Adapters are
// already resolved (see ComposeAdapters).
type Selection struct {
	Collections []string
	Agents      []string
	Editors     []string
	// Registry is the wsup.json generators read; nil means a new workspace
	// with no projects.
	Registry []byte
}

// Modules returns every built-in module, sorted by id. The result is shared
// and must be treated as read-only.
func Modules() []Module {
	c := mustLoad()
	modules := make([]Module, 0, len(c.modules))
	for _, id := range slices.Sorted(maps.Keys(c.modules)) {
		modules = append(modules, c.modules[id])
	}
	return modules
}

// KindOf returns a built-in module's kind.
func KindOf(id string) (string, bool) {
	module, ok := mustLoad().Module(id)
	return module.Kind, ok
}

// resolve validates a selection for a starter and returns the catalogue and
// the resolved module ids, base first.
func resolve(kind string, selection Selection) (*Catalogue, []string, error) {
	if kind != "workspace" && kind != "project" {
		return nil, nil, fmt.Errorf("template must be workspace or project")
	}
	if kind != "workspace" && len(selection.Collections) != 0 {
		return nil, nil, fmt.Errorf("optional collections require a workspace template")
	}
	selected, err := SelectCollections(selection.Collections)
	if err != nil {
		return nil, nil, err
	}
	for _, adapter := range []struct {
		kind  string
		names []string
	}{{KindAgent, selection.Agents}, {KindEditor, selection.Editors}} {
		for _, name := range adapter.names {
			if !slices.Contains(mustLoad().IDs(adapter.kind), name) {
				return nil, nil, fmt.Errorf("unknown %s %q", adapter.kind, name)
			}
		}
	}
	c, err := Load()
	if err != nil {
		return nil, nil, err
	}
	ids, err := c.Resolve(kind, slices.Concat(selected, selection.Agents, selection.Editors))
	if err != nil {
		return nil, nil, err
	}
	return c, ids, nil
}

// Installed returns the version of every module a starter with the given
// selection installs, including its base and required modules.
func Installed(kind string, selection Selection) (map[string]string, error) {
	c, ids, err := resolve(kind, selection)
	if err != nil {
		return nil, err
	}
	versions := make(map[string]string, len(ids))
	for _, id := range ids {
		versions[id] = c.modules[id].Version
	}
	return versions, nil
}

// Starter returns the files, including generated ones, and links for a
// starter with the given selection.
func Starter(kind string, selection Selection) (map[string][]byte, map[string]string, error) {
	c, ids, err := resolve(kind, selection)
	if err != nil {
		return nil, nil, err
	}
	files, err := c.Files(ids)
	if err != nil {
		return nil, nil, err
	}
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("embedded starter catalogue is empty")
	}
	// The registry is read only when a resolved module has a generator.
	if slices.ContainsFunc(ids, func(id string) bool { return len(c.modules[id].Generate) != 0 }) {
		workspace := Workspace{Name: DefaultWorkspaceName}
		if selection.Registry != nil {
			if workspace, err = ParseWorkspace(selection.Registry); err != nil {
				return nil, nil, err
			}
		}
		generated, err := c.Generate(ids, workspace)
		if err != nil {
			return nil, nil, err
		}
		for name, body := range generated {
			if _, clash := files[name]; clash {
				return nil, nil, fmt.Errorf("generated %s collides with a starter file", name)
			}
			files[name] = body
		}
	}
	links, err := c.Links(ids)
	if err != nil {
		return nil, nil, err
	}
	return files, links, nil
}

// Open returns a read-only filesystem without accessing the checkout or extracting files.
// Asset inventory and byte parity with authoritative sources are verified by tests.
func Open() fs.FS {
	return templates.Files
}
