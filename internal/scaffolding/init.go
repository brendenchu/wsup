package scaffolding

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/brendenchu/wsup/internal/registry"
)

// InitialiseProject creates a project from the executable's embedded catalogue,
// independent of the selected workspace, checkout, or local template edits.
// Existing destinations, including empty directories and dangling links, fail.
func InitialiseProject(destination, version string) (string, error) {
	return Initialise(destination, version, "project")
}

// Initialise creates a native workspace or project using embedded templates.
func Initialise(destination, version, kind string) (string, error) {
	return InitialiseSelected(destination, version, kind, nil)
}

// InitialiseSelected creates only explicitly selected optional workspace homes.
func InitialiseSelected(destination, version, kind string, names []string) (string, error) {
	return InitialiseWith(destination, version, kind, catalogue.Selection{Collections: names})
}

// InitialiseWith creates a starter with explicit collections and resolved
// adapters (see catalogue.ComposeAdapters). A workspace also gets wsup.json
// recording that selection, which generators read. Adapter links are created
// after the files and before wsup.lock, so a failure never records missing links.
func InitialiseWith(destination, version, kind string, selection catalogue.Selection) (string, error) {
	selected, err := catalogue.SelectCollections(selection.Collections)
	if err != nil {
		return "", err
	}
	selection.Collections, selection.Registry = selected, nil
	owned := map[string][]byte{}
	if kind == "workspace" {
		document, err := registry.Encode(registry.Document{
			Schema: registry.SchemaURL, Name: catalogue.DefaultWorkspaceName,
			Collections: selected, Agents: selection.Agents, Editors: selection.Editors,
		})
		if err != nil {
			return "", err
		}
		selection.Registry = document
		owned[registry.DocumentPath] = document
	}
	files, links, err := catalogue.Starter(kind, selection)
	if err != nil {
		return "", err
	}
	modules, err := catalogue.Installed(kind, selection)
	if err != nil {
		return "", err
	}
	return initialiseStarter(destination, kind, files, owned, newLock(version, kind, modules, files, links), writeManaged)
}

func initialiseProject(destination, version string, files map[string][]byte, write func(*os.Root, string, []byte, bool) error) (string, error) {
	return initialise(destination, version, "project", files, write)
}

func initialise(destination, version, kind string, files map[string][]byte, write func(*os.Root, string, []byte, bool) error) (string, error) {
	return initialiseSelected(destination, version, kind, files, nil, write)
}

// initialiseSelected writes injected files for tests, locking the modules the
// selected collections resolve to.
func initialiseSelected(destination, version, kind string, files map[string][]byte, selected []string, write func(*os.Root, string, []byte, bool) error) (string, error) {
	modules, err := catalogue.Installed(kind, catalogue.Selection{Collections: selected})
	if err != nil {
		return "", err
	}
	return initialiseStarter(destination, kind, files, nil, newLock(version, kind, modules, files, nil), write)
}

// initialiseStarter writes the managed files, the owner's files (wsup.json),
// then the lock's links, then wsup.lock.
func initialiseStarter(destination, kind string, files, owned map[string][]byte, lock Lock, write func(*os.Root, string, []byte, bool) error) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("embedded starter catalogue is empty")
	}
	if _, exists := files[registry.DocumentPath]; exists {
		return "", fmt.Errorf("wsup.json belongs to the owner and is never a managed file")
	}
	if _, exists := owned[registry.DocumentPath]; exists && kind == "project" {
		return "", fmt.Errorf("project starter must not contain a workspace registry")
	}
	body, err := EncodeLock(lock)
	if err != nil {
		return "", err
	}
	all := maps.Clone(files)
	maps.Copy(all, owned)
	links := lock.Links
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
	for _, name := range slices.Sorted(maps.Keys(all)) {
		if err := write(root, name, all[name], true); err != nil {
			return fail(err)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(links)) {
		if err := createLink(root, name, links[name]); err != nil {
			return fail(err)
		}
	}
	if err := write(root, LockPath, body, true); err != nil {
		return fail(err)
	}
	return destination, nil
}
