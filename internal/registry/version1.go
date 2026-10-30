package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Version1Project is one project from a v0.2.0 workspace.json, with its checks
// in source order and its original JSON shape.
type Version1Project struct {
	ID           string
	Path         string
	Dependencies []string
	Checks       map[string]Check
	CheckNames   []string
	Raw          map[string]any
}

// Version1Registry is a validated v0.2.0 workspace.json.
type Version1Registry struct {
	Name     string
	Projects []Version1Project
}

// LoadVersion1 validates a v0.2.0 workspace.json: version 1 fields, project
// paths/README files, check policies, and dependency references/cycles. It is
// read only as migration input; workspaces are loaded from wsup.json by Load.
func LoadVersion1(root string) (Version1Registry, error) {
	filename := filepath.Join(root, "workspace.json")
	body, err := os.ReadFile(filename)
	if err != nil {
		return Version1Registry{}, err
	}
	document, err := readObject(filename, body)
	if err != nil {
		return Version1Registry{}, err
	}
	version, valid := Integer(document["version"])
	if !Fields(document, "version", "name", "projects") || !valid || version != 1 {
		return Version1Registry{}, fmt.Errorf("workspace.json requires supported version 1 fields")
	}
	name, validName := document["name"].(string)
	entries, validEntries := document["projects"].([]any)
	if !validName || strings.TrimSpace(name) == "" || !validEntries {
		return Version1Registry{}, fmt.Errorf("Registry needs a name and projects list")
	}
	result := Version1Registry{Name: name, Projects: make([]Version1Project, 0, len(entries))}
	ids, paths := map[string]bool{}, map[string]bool{}
	for _, entry := range entries {
		raw, ok := entry.(map[string]any)
		if !ok || !Fields(raw, "id", "purpose", "owner", "path", "kind", "repository", "dependencies", "checks") {
			return Version1Registry{}, fmt.Errorf("Invalid project fields")
		}
		id, ok := raw["id"].(string)
		if !ok || !identifier.MatchString(id) || ids[id] {
			return Version1Registry{}, fmt.Errorf("Project IDs must be unique letters/digits/dots/underscores/hyphens")
		}
		for _, field := range []string{"purpose", "owner", "path"} {
			text, ok := raw[field].(string)
			if !ok || strings.TrimSpace(text) == "" {
				return Version1Registry{}, fmt.Errorf("%s: missing %s", id, field)
			}
		}
		kind, _ := raw["kind"].(string)
		repository, _ := raw["repository"].(string)
		if !map[string]bool{"app": true, "package": true, "tool": true, "infrastructure": true, "research": true, "template": true}[kind] ||
			(repository != "workspace" && repository != "independent") {
			return Version1Registry{}, fmt.Errorf("%s: invalid kind/repository", id)
		}
		target, err := RelativeTarget(root, raw["path"])
		if err != nil {
			return Version1Registry{}, err
		}
		resolved, err := Resolve(target)
		if err != nil {
			return Version1Registry{}, err
		}
		info, statErr := os.Stat(target)
		readme, readmeErr := os.Stat(filepath.Join(target, "README.md"))
		if statErr != nil || !info.IsDir() || paths[resolved] || readmeErr != nil || !readme.Mode().IsRegular() {
			return Version1Registry{}, fmt.Errorf("%s: path must be unique/existing and contain README.md", id)
		}
		project := Version1Project{ID: id, Path: raw["path"].(string), Raw: raw, Checks: map[string]Check{}}
		if deps, exists := raw["dependencies"]; exists {
			project.Dependencies, ok = stringsValue(deps)
			if !ok {
				return Version1Registry{}, fmt.Errorf("%s: dependencies must be project IDs", id)
			}
		}
		if checks, exists := raw["checks"]; exists {
			values, ok := checks.(map[string]any)
			if !ok {
				return Version1Registry{}, fmt.Errorf("%s: checks must be named definitions", id)
			}
			// Decode order separately to preserve configured check order.
			project.CheckNames, err = checkOrder(body, len(result.Projects))
			if err != nil {
				return Version1Registry{}, err
			}
			for _, name := range project.CheckNames {
				if !identifier.MatchString(name) {
					return Version1Registry{}, fmt.Errorf("%s: invalid check name", id)
				}
				check, err := ParseCheck(values[name])
				if err != nil {
					return Version1Registry{}, err
				}
				project.Checks[name] = check
			}
		}
		ids[id], paths[resolved] = true, true
		result.Projects = append(result.Projects, project)
	}
	dependencies := make(map[string][]string, len(result.Projects))
	order := make([]string, 0, len(result.Projects))
	for _, project := range result.Projects {
		dependencies[project.ID] = project.Dependencies
		order = append(order, project.ID)
	}
	if err := checkDependencies(order, dependencies); err != nil {
		return Version1Registry{}, err
	}
	return result, nil
}
