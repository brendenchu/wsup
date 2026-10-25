// Package catalogue exposes the embedded starter snapshot used during the Go port.
package catalogue

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"io/fs"
)

//go:generate go run ./generate -source ../../src/templates -output assets.zip

// The archive is temporary migration output; src/templates remains authoritative.
//
//go:embed assets.zip
var assets []byte

// Open returns a read-only filesystem without accessing the checkout or extracting files.
// Asset inventory and byte parity with authoritative sources are verified by tests.
func Open() (fs.FS, error) {
	reader, err := zip.NewReader(bytes.NewReader(assets), int64(len(assets)))
	if err != nil {
		return nil, fmt.Errorf("open embedded starter catalogue: %w", err)
	}
	return reader, nil
}
