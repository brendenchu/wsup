package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	source := flag.String("source", "", "Authoritative template directory")
	output := flag.String("output", "", "Generated archive destination")
	flag.Parse()
	if err := generate(*source, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(source, output string) error {
	if source == "" || output == "" {
		return fmt.Errorf("source and output are required")
	}
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("template symlinks are not allowed: %s", path)
		}
		if entry.IsDir() {
			if entry.Name() == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".pyc") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("template must be a regular file: %s", path)
		}
		name, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		header := &zip.FileHeader{Name: filepath.ToSlash(name), Method: zip.Deflate}
		header.SetMode(0644)
		destination, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = destination.Write(body)
		return err
	})
	if err != nil {
		return fmt.Errorf("generate catalogue: %w", err)
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return os.WriteFile(output, data.Bytes(), 0644)
}
