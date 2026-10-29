// Package catalogue selects complete embedded native starters from the
// built-in module catalogue.
package catalogue

import (
	"fmt"
	"io/fs"
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
	if kind != "workspace" && kind != "project" {
		return nil, fmt.Errorf("template must be workspace or project")
	}
	if kind != "workspace" && len(collections) != 0 {
		return nil, fmt.Errorf("optional collections require a workspace template")
	}
	selected, err := SelectCollections(collections)
	if err != nil {
		return nil, err
	}
	c, err := Load()
	if err != nil {
		return nil, err
	}
	ids, err := c.Resolve(kind, selected)
	if err != nil {
		return nil, err
	}
	files, err := c.Files(ids)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("embedded starter catalogue is empty")
	}
	return files, nil
}

// Open returns a read-only filesystem without accessing the checkout or extracting files.
// Asset inventory and byte parity with authoritative sources are verified by tests.
func Open() fs.FS {
	return templates.Files
}
