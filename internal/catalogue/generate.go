package catalogue

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"

	"github.com/brendenchu/wsup/internal/jsoncompat"
	"github.com/brendenchu/wsup/internal/registry"
)

// Workspace is the fixed, read-only input generators see: the decoded
// wsup.json.
type Workspace = registry.Document

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

// ParseWorkspace decodes and validates wsup.json for generators, so an invalid
// file never produces generated output.
func ParseWorkspace(body []byte) (Workspace, error) {
	return registry.Decode(body)
}

// rootFolder names the workspace root in the generated file; tasks address
// project directories through it.
const rootFolder = "Workspace"

// CodeWorkspace returns a VS Code multi-root workspace file: the root folder
// plus one folder per project, the vscode passthrough, and one VS Code task per
// project task (check tasks in the test group) followed by the passthrough
// tasks. ASCII escapes keep the bytes stable.
func CodeWorkspace(workspace Workspace) ([]byte, error) {
	type folder struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	type options struct {
		Cwd string `json:"cwd"`
	}
	type task struct {
		Label          string   `json:"label"`
		Type           string   `json:"type"`
		Command        string   `json:"command"`
		Args           []string `json:"args,omitempty"`
		Options        options  `json:"options"`
		Group          string   `json:"group,omitempty"`
		ProblemMatcher []any    `json:"problemMatcher"`
	}
	type versioned struct {
		Version string `json:"version"`
		Tasks   []any  `json:"tasks,omitempty"`
		// Configurations is used by launch.
		Configurations []any `json:"configurations,omitempty"`
	}
	type recommendations struct {
		Recommendations []string `json:"recommendations"`
	}
	document := struct {
		Folders    []folder         `json:"folders"`
		Settings   map[string]any   `json:"settings"`
		Extensions *recommendations `json:"extensions,omitempty"`
		Tasks      *versioned       `json:"tasks,omitempty"`
		Launch     *versioned       `json:"launch,omitempty"`
	}{Folders: []folder{{Name: rootFolder, Path: "."}}, Settings: map[string]any{}}
	tasks := []any{}
	for _, project := range workspace.Projects {
		if project.Path != "." {
			document.Folders = append(document.Folders, folder{Name: project.ID, Path: project.Path})
		}
		for _, entry := range project.Tasks {
			cwd := "${workspaceFolder:" + rootFolder + "}"
			for _, part := range []string{project.Path, entry.Cwd} {
				if part != "" && part != "." {
					cwd += "/" + part
				}
			}
			generated := task{
				Label: project.ID + ": " + entry.Name, Type: "process",
				Command: entry.Check.Command[0], Args: entry.Check.Command[1:],
				Options: options{Cwd: cwd}, ProblemMatcher: []any{},
			}
			if entry.IsCheck {
				generated.Group = "test"
			}
			tasks = append(tasks, generated)
		}
	}
	if vscode := workspace.VSCode; vscode != nil {
		if len(vscode.Settings) != 0 {
			document.Settings = vscode.Settings
		}
		if len(vscode.Extensions) != 0 {
			document.Extensions = &recommendations{Recommendations: vscode.Extensions}
		}
		for _, entry := range vscode.Tasks {
			tasks = append(tasks, entry)
		}
		if len(vscode.Launch) != 0 {
			document.Launch = &versioned{Version: "0.2.0"}
			for _, entry := range vscode.Launch {
				document.Launch.Configurations = append(document.Launch.Configurations, entry)
			}
		}
	}
	if len(tasks) != 0 {
		document.Tasks = &versioned{Version: "2.0.0", Tasks: tasks}
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
