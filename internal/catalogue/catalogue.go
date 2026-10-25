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

func Open() (fs.FS, error) {
	reader, err := zip.NewReader(bytes.NewReader(assets), int64(len(assets)))
	if err != nil {
		return nil, fmt.Errorf("open embedded starter catalogue: %w", err)
	}
	return reader, nil
}
