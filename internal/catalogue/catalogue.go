// Package catalogue exposes the embedded starter snapshot used during the Go port.
package catalogue

import (
	"archive/zip"
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"strings"
)

//go:generate go run ./generate -source ../../src/templates -output assets.zip

// The archive is temporary migration output; src/templates remains authoritative.
//
//go:embed assets.zip
var assets []byte

// Native-only installation guidance coexists with the unchanged Python starter
// until cutover. Shared rules, skills, and other assets still come from the ZIP.
//
//go:embed all:native
var native embed.FS

// Bundle returns starter bytes, applying explicit native installation overrides.
// Neither workspace customizations nor checkout files are read at runtime.
func Bundle(kind string) (map[string][]byte, error) {
	if kind != "workspace" && kind != "project" {
		return nil, fmt.Errorf("template must be workspace or project")
	}
	tree, err := Open()
	if err != nil {
		return nil, err
	}
	files := make(map[string][]byte)
	read := func(tree fs.FS, base string) error {
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
			files[strings.TrimPrefix(name, base+"/")] = body
			return nil
		})
	}
	if err := read(tree, kind); err != nil {
		return nil, err
	}
	if err := read(native, "native/"+kind); err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("embedded starter catalogue is empty")
	}
	return files, nil
}

// Open returns a read-only filesystem without accessing the checkout or extracting files.
// Asset inventory and byte parity with authoritative sources are verified by tests.
func Open() (fs.FS, error) {
	reader, err := zip.NewReader(bytes.NewReader(assets), int64(len(assets)))
	if err != nil {
		return nil, fmt.Errorf("open embedded starter catalogue: %w", err)
	}
	return reader, nil
}
