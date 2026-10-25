package scaffolding

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/brendenchu/wsup/internal/catalogue"
)

// InitialiseProject creates a project from the executable's embedded catalogue,
// independent of the selected workspace, checkout, or local template edits.
// Existing destinations, including empty directories and dangling links, fail.
func InitialiseProject(destination, version string) (string, error) {
	tree, err := catalogue.Open()
	if err != nil {
		return "", err
	}
	files := make(map[string][]byte)
	err = fs.WalkDir(tree, "project", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("project catalogue must contain only regular files")
		}
		body, err := fs.ReadFile(tree, name)
		if err != nil {
			return err
		}
		files[name[len("project/"):]] = body
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("read embedded project catalogue: %w", err)
	}
	if len(files) == 0 {
		return "", fmt.Errorf("embedded project catalogue is empty")
	}
	return initialiseProject(destination, version, files, writeManaged)
}

func initialiseProject(destination, version string, files map[string][]byte, write func(*os.Root, string, []byte, bool) error) (string, error) {
	state := State{Version: StateVersion, TemplateVersion: version, Template: "project", Files: make(map[string]string, len(files))}
	names := make([]string, 0, len(files))
	for name, body := range files {
		if name == "workspace.json" {
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
		return "", fmt.Errorf("create exclusive project destination: %w", err)
	}
	fail := func(err error) (string, error) {
		// Never recursively remove a destination another process could now use.
		return "", fmt.Errorf("project initialization incomplete at %s; partial files/directories may remain; inspect before retrying: %w", destination, err)
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
