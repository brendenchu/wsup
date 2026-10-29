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

// CollectionNames returns a fresh ordered list of supported optional homes.
// The embedded catalogue is validated by tests, so a load failure here is a
// build defect and panics rather than silently offering no collections.
func CollectionNames() []string {
	c, err := Load()
	if err != nil {
		panic("embedded module catalogue is invalid: " + err.Error())
	}
	return c.IDs(KindCollection)
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
