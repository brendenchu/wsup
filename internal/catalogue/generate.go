package catalogue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/brendenchu/wsup/internal/jsoncompat"
)

// Workspace is the fixed, read-only input generators see: the workspace name
// and its registered projects, decoded from the workspace registry.
type Workspace struct {
	Name     string
	Projects []Project
}

// Project is one registered project as generators see it.
type Project struct {
	ID   string
	Path string
}

// generator builds one file from the workspace and returns its path. Only
// wsup defines generators; modules name them in their manifest's "generate".
type generator struct {
	build func(Workspace) (string, []byte, error)
	// owns reports whether a path has this generator's output shape. Its path
	// can follow workspace data (such as the name), so a managed file it no
	// longer writes is replaced on update rather than left as an orphan.
	owns func(string) bool
}

var generators = map[string]generator{
	"code-workspace": {
		build: func(workspace Workspace) (string, []byte, error) {
			body, err := CodeWorkspace(workspace)
			return Slug(workspace.Name) + ".code-workspace", body, err
		},
		owns: func(name string) bool {
			return !strings.Contains(name, "/") && strings.HasSuffix(name, ".code-workspace")
		},
	},
}

// Generated reports whether a workspace-relative path is one a built-in
// generator writes. Module files never use these paths.
func Generated(name string) bool {
	for _, generator := range generators {
		if generator.owns(name) {
			return true
		}
	}
	return false
}

// ParseWorkspace decodes a registry's name and project ids and paths for
// generators. It checks only the shape generators need; registry.Load owns
// full validation.
func ParseWorkspace(body []byte) (Workspace, error) {
	var document struct {
		Name     string `json:"name"`
		Projects []struct {
			ID   string `json:"id"`
			Path string `json:"path"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return Workspace{}, fmt.Errorf("read workspace name and projects: %w", err)
	}
	workspace := Workspace{Name: document.Name, Projects: []Project{}}
	for _, project := range document.Projects {
		if project.ID == "" || project.Path == "" {
			return Workspace{}, fmt.Errorf("read workspace projects: every project needs an id and path")
		}
		workspace.Projects = append(workspace.Projects, Project{ID: project.ID, Path: project.Path})
	}
	return workspace, nil
}

// CodeWorkspace returns a VS Code multi-root workspace file: the root folder
// plus one folder per project, with ASCII escapes so bytes are stable.
func CodeWorkspace(workspace Workspace) ([]byte, error) {
	type folder struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	document := struct {
		Folders  []folder       `json:"folders"`
		Settings map[string]any `json:"settings"`
	}{Folders: []folder{{Name: "Workspace", Path: "."}}, Settings: map[string]any{}}
	for _, project := range workspace.Projects {
		if project.Path != "." {
			document.Folders = append(document.Folders, folder{Name: project.ID, Path: project.Path})
		}
	}
	encoded, err := jsoncompat.Marshal(document, "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

// Slug lowercases a name and joins its letters and digits with hyphens, for
// generated filenames. An empty result becomes "workspace".
func Slug(name string) string {
	var slug strings.Builder
	for _, word := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !(r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)))
	}) {
		if slug.Len() != 0 {
			slug.WriteByte('-')
		}
		slug.WriteString(word)
	}
	if slug.Len() == 0 {
		return "workspace"
	}
	return slug.String()
}

// Generate runs the resolved modules' generators against workspace.
func (c *Catalogue) Generate(ids []string, workspace Workspace) (map[string][]byte, error) {
	files := map[string][]byte{}
	for _, id := range ids {
		for _, name := range c.modules[id].Generate {
			path, body, err := generators[name].build(workspace)
			if err != nil {
				return nil, fmt.Errorf("module %q: generate %s: %w", id, name, err)
			}
			if _, exists := files[path]; exists || !portable(path) || strings.Contains(path, "/") || !generators[name].owns(path) {
				return nil, fmt.Errorf("module %q: generator %s wrote an invalid or duplicate path %s", id, name, path)
			}
			files[path] = bytes.Clone(body)
		}
	}
	return files, nil
}
