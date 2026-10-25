package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDeterministicGenerationAndSymlinkRejection(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	if err := os.Mkdir(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".hidden"), []byte("exact bytes\n"), 0600); err != nil {
		t.Fatal(err)
	}
	first, second := filepath.Join(directory, "first.zip"), filepath.Join(directory, "second.zip")
	for _, output := range []string{first, second} {
		if err := generate(source, output); err != nil {
			t.Fatal(err)
		}
	}
	a, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("generated archive is not deterministic")
	}
	if err := os.Symlink(filepath.Join(source, ".hidden"), filepath.Join(source, "link")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if err := generate(source, second); err == nil {
		t.Fatal("symlink template accepted")
	}
}
