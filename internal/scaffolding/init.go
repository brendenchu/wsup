package scaffolding

import (
	"fmt"
	"maps"
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
	return InitialiseSelected(destination, version, kind, nil)
}

// InitialiseSelected creates only explicitly selected optional workspace homes.
func InitialiseSelected(destination, version, kind string, names []string) (string, error) {
	return InitialiseWith(destination, version, kind, catalogue.Selection{Collections: names})
}

// InitialiseWith creates a starter with explicit collections and resolved
// adapters (see catalogue.ComposeAdapters). Generators read the starter's own
// registry. Adapter links are created after the files and before state, so a
// failure never commits state for missing links.
func InitialiseWith(destination, version, kind string, selection catalogue.Selection) (string, error) {
	selected, err := catalogue.SelectCollections(selection.Collections)
	if err != nil {
		return "", err
	}
	selection.Collections, selection.Registry = selected, nil
	files, links, err := catalogue.Starter(kind, selection)
	if err != nil {
		return "", err
	}
	state := State{Collections: selected, Agents: selection.Agents, Editors: selection.Editors}
	return initialiseStarter(destination, version, kind, files, links, state, writeManaged)
}

func initialiseProject(destination, version string, files map[string][]byte, write func(*os.Root, string, []byte, bool) error) (string, error) {
	return initialise(destination, version, "project", files, write)
}

func initialise(destination, version, kind string, files map[string][]byte, write func(*os.Root, string, []byte, bool) error) (string, error) {
	return initialiseSelected(destination, version, kind, files, nil, write)
}

func initialiseSelected(destination, version, kind string, files map[string][]byte, selected []string, write func(*os.Root, string, []byte, bool) error) (string, error) {
	return initialiseStarter(destination, version, kind, files, nil, State{Collections: selected}, write)
}

// initialiseStarter writes files, then links, then state built from selected
// (its Collections, Agents, and Editors).
func initialiseStarter(destination, version, kind string, files map[string][]byte, links map[string]string, selected State, write func(*os.Root, string, []byte, bool) error) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("embedded starter catalogue is empty")
	}
	state := State{Version: StateVersion, TemplateVersion: version, Template: kind, Files: make(map[string]string, len(files)),
		Collections: selected.Collections, Agents: selected.Agents, Editors: selected.Editors}
	if len(links) != 0 {
		state.Links = links
	}
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
	for _, name := range slices.Sorted(maps.Keys(links)) {
		if err := createLink(root, name, links[name]); err != nil {
			return fail(err)
		}
	}
	if err := write(root, StatePath, body, true); err != nil {
		return fail(err)
	}
	return destination, nil
}
