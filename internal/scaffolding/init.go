package scaffolding

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/brendenchu/wsup/internal/catalogue"
)

// InitialiseProject creates a project from the executable's embedded catalogue,
// independent of the selected workspace, checkout, or local template edits.
// Existing destinations, including empty directories and dangling links, fail.
func InitialiseProject(destination, version string) (string, error) {
	return Initialise(destination, version, "project")
}

// Initialise creates a native workspace or project using embedded templates.
func Initialise(destination, version, kind string) (string, error) {
	files, err := catalogue.Bundle(kind)
	if err != nil {
		return "", err
	}
	return initialise(destination, version, kind, files, writeManaged)
}

func initialiseProject(destination, version string, files map[string][]byte, write func(*os.Root, string, []byte, bool) error) (string, error) {
	return initialise(destination, version, "project", files, write)
}

func initialise(destination, version, kind string, files map[string][]byte, write func(*os.Root, string, []byte, bool) error) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("embedded starter catalogue is empty")
	}
	state := State{Version: StateVersion, TemplateVersion: version, Template: kind, Files: make(map[string]string, len(files))}
	names := make([]string, 0, len(files))
	for name, body := range files {
		if kind == "project" && name == "workspace.json" {
			return "", fmt.Errorf("project starter must not contain a workspace registry")
		}
		state.Files[name] = contentDigest(body)
		names = append(names, name)
	}
	body, err := EncodeNativeState(state)
	if err != nil {
		return "", err
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	if _, err := os.Lstat(destination); err == nil {
		return "", fmt.Errorf("destination must not exist; existing work is never overwritten")
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return "", err
	}
	if err := os.Mkdir(destination, 0755); err != nil {
		return "", fmt.Errorf("create exclusive %s destination: %w", kind, err)
	}
	fail := func(err error) (string, error) {
		// Never recursively remove a destination another process could now use.
		return "", fmt.Errorf("%s initialization incomplete at %s; partial files/directories may remain; inspect before retrying: %w", kind, destination, err)
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		return fail(err)
	}
	defer root.Close()
	slices.Sort(names)
	for _, name := range names {
		if err := write(root, name, files[name], true); err != nil {
			return fail(err)
		}
	}
	if err := write(root, StatePath, body, true); err != nil {
		return fail(err)
	}
	return destination, nil
}
