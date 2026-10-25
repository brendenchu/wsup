// Package catalogue selects complete embedded native starters.
package catalogue

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/brendenchu/wsup/templates"
)

// CollectionNames returns a fresh ordered list of supported optional homes.
func CollectionNames() []string {
	return []string{"archive", "artifacts", "journal", "learning", "private", "reference", "research"}
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
	tree := Open()
	files := make(map[string][]byte)
	read := func(tree fs.FS, base, prefix string) error {
		return fs.WalkDir(tree, base, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if !entry.Type().IsRegular() {
				return fmt.Errorf("starter catalogue must contain only regular files")
			}
			body, err := fs.ReadFile(tree, name)
			if err != nil {
				return err
			}
			files[prefix+strings.TrimPrefix(name, base+"/")] = body
			return nil
		})
	}
	if err := read(tree, kind, ""); err != nil {
		return nil, err
	}
	for _, name := range selected {
		if err := read(tree, "collections/"+name, name+"/"); err != nil {
			return nil, err
		}
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
