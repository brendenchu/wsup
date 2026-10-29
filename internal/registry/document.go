package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/brendenchu/wsup/internal/catalogue"
)

// DocumentVersion is the wsup.json schema version that Decode accepts.
const DocumentVersion = 3

// projectKinds is the closed set of project kinds (contract amendment 1).
var projectKinds = []string{"app", "package", "tool", "infra", "sandbox", "template", "fork", "research"}

// Document is a decoded wsup.json: the workspace's module selection, shared
// settings, projects, and VS Code passthrough. Decode validates its shape only;
// paths are checked against the filesystem when a workspace is loaded.
type Document struct {
	Schema      string
	Name        string
	Collections []string
	Agents      []string
	Editors     []string
	// Settings holds shared settings by key; configuration validates values.
	Settings map[string]any
	Projects []ProjectEntry
	VSCode   *VSCode
}

// ProjectEntry is one project as declared in wsup.json.
type ProjectEntry struct {
	ID           string
	Path         string
	Kind         string
	Repository   string
	Git          string
	Purpose      string
	Owner        string
	Dependencies []string
	// Tasks keep their source order, which is also check run order.
	Tasks []Task
}

// Task is a named project command. Check carries the command and the policy
// that applies when the task runs under wsup check (IsCheck).
type Task struct {
	Name    string
	IsCheck bool
	// Cwd is relative to the project; empty means the project root.
	Cwd   string
	Check Check
}

// VSCode is passed through to the generated .code-workspace file unchanged.
type VSCode struct {
	Settings   map[string]any
	Extensions []string
	Tasks      []map[string]any
	Launch     []map[string]any
}

// Decode validates one wsup.json document without reading the filesystem:
// strict fields, version 3, unique module and project ids, project kinds and
// paths, task definitions, and dependency references and cycles.
func Decode(body []byte) (Document, error) {
	const filename = "wsup.json"
	object, err := ParseObject(filename, body)
	if err != nil {
		return Document{}, err
	}
	if !Fields(object, "$schema", "version", "name", "collections", "agents", "editors", "settings", "projects", "vscode") {
		return Document{}, fmt.Errorf("wsup.json: unknown top-level field")
	}
	if version, ok := Integer(object["version"]); !ok || version != DocumentVersion {
		return Document{}, fmt.Errorf("wsup.json: requires version %d", DocumentVersion)
	}
	document := Document{}
	if raw, exists := object["$schema"]; exists {
		if document.Schema, _ = raw.(string); strings.TrimSpace(document.Schema) == "" {
			return Document{}, fmt.Errorf("wsup.json: $schema must be a nonempty string")
		}
	}
	if document.Name, _ = object["name"].(string); strings.TrimSpace(document.Name) == "" {
		return Document{}, fmt.Errorf("wsup.json: name must be a nonempty string")
	}
	for _, list := range []struct {
		key    string
		target *[]string
	}{{"collections", &document.Collections}, {"agents", &document.Agents}, {"editors", &document.Editors}} {
		*list.target = []string{}
		raw, exists := object[list.key]
		if !exists {
			continue
		}
		ids, ok := stringsValue(raw)
		if !ok {
			return Document{}, fmt.Errorf("wsup.json: %s must be a list of module ids", list.key)
		}
		for i, id := range ids {
			if !catalogue.ValidID(id) || slices.Contains(ids[:i], id) {
				return Document{}, fmt.Errorf("wsup.json: %s: %q is not a unique module id", list.key, id)
			}
		}
		*list.target = ids
	}
	if raw, exists := object["settings"]; exists {
		settings, ok := raw.(map[string]any)
		if !ok || !Fields(settings, "timeout_seconds", "output_limit_bytes", "report_dir") {
			return Document{}, fmt.Errorf("wsup.json: settings may contain only timeout_seconds, output_limit_bytes, and report_dir")
		}
		document.Settings = settings
	}
	if document.VSCode, err = decodeVSCode(object["vscode"]); err != nil {
		return Document{}, err
	}
	entries, ok := object["projects"].([]any)
	if !ok {
		return Document{}, fmt.Errorf("wsup.json: projects must be a list")
	}
	// Task order is only visible in the raw bytes.
	var ordered struct {
		Projects []struct {
			Tasks json.RawMessage `json:"tasks"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(body, &ordered); err != nil {
		return Document{}, fmt.Errorf("wsup.json: %w", err)
	}
	document.Projects = make([]ProjectEntry, 0, len(entries))
	dependencies := map[string][]string{}
	order := []string{}
	paths := map[string]string{}
	for index, entry := range entries {
		project, err := decodeProject(entry, ordered.Projects[index].Tasks)
		if err != nil {
			return Document{}, err
		}
		if _, duplicate := dependencies[project.ID]; duplicate {
			return Document{}, fmt.Errorf("wsup.json: project id %s is not unique", project.ID)
		}
		if other, duplicate := paths[project.Path]; duplicate {
			return Document{}, fmt.Errorf("wsup.json: projects %s and %s share path %s", other, project.ID, project.Path)
		}
		paths[project.Path] = project.ID
		dependencies[project.ID] = project.Dependencies
		order = append(order, project.ID)
		document.Projects = append(document.Projects, project)
	}
	if err := checkDependencies(order, dependencies); err != nil {
		return Document{}, fmt.Errorf("wsup.json: %w", err)
	}
	return document, nil
}

func decodeProject(entry any, rawTasks json.RawMessage) (ProjectEntry, error) {
	raw, ok := entry.(map[string]any)
	if !ok || !Fields(raw, "id", "path", "kind", "repository", "git", "purpose", "owner", "dependencies", "tasks") {
		return ProjectEntry{}, fmt.Errorf("wsup.json: each project is an object with supported fields")
	}
	project := ProjectEntry{Dependencies: []string{}, Tasks: []Task{}}
	if project.ID, _ = raw["id"].(string); !identifier.MatchString(project.ID) {
		return ProjectEntry{}, fmt.Errorf("wsup.json: project ids use letters, digits, dots, underscores, and hyphens")
	}
	fail := func(format string, args ...any) (ProjectEntry, error) {
		return ProjectEntry{}, fmt.Errorf("wsup.json: project %s: "+format, append([]any{project.ID}, args...)...)
	}
	for _, field := range []struct {
		key    string
		target *string
	}{{"path", &project.Path}, {"purpose", &project.Purpose}, {"owner", &project.Owner}} {
		if *field.target, _ = raw[field.key].(string); strings.TrimSpace(*field.target) == "" {
			return fail("missing %s", field.key)
		}
	}
	if !portablePath(project.Path) {
		return fail("path must be a relative forward-slash path inside the workspace")
	}
	if project.Kind, _ = raw["kind"].(string); !slices.Contains(projectKinds, project.Kind) {
		return fail("kind must be one of %s", strings.Join(projectKinds, ", "))
	}
	if project.Repository, _ = raw["repository"].(string); project.Repository != "workspace" && project.Repository != "independent" {
		return fail("repository must be workspace or independent")
	}
	if value, exists := raw["git"]; exists {
		if project.Git, _ = value.(string); strings.TrimSpace(project.Git) == "" ||
			strings.ContainsFunc(project.Git, func(r rune) bool { return r < ' ' }) {
			return fail("git must be a nonempty single-line URL")
		}
	}
	if value, exists := raw["dependencies"]; exists {
		if project.Dependencies, ok = stringsValue(value); !ok {
			return fail("dependencies must be project ids")
		}
	}
	value, exists := raw["tasks"]
	if !exists {
		return project, nil
	}
	tasks, ok := value.(map[string]any)
	if !ok {
		return fail("tasks must be named definitions")
	}
	names, err := objectKeys(rawTasks)
	if err != nil {
		return ProjectEntry{}, fmt.Errorf("wsup.json: %w", err)
	}
	for _, name := range names {
		if !identifier.MatchString(name) {
			return fail("invalid task name %q", name)
		}
		task, err := decodeTask(name, tasks[name])
		if err != nil {
			return fail("task %s: %v", name, err)
		}
		project.Tasks = append(project.Tasks, task)
	}
	return project, nil
}

// decodeTask accepts a bare argument list or an object with the check fields
// plus check and cwd.
func decodeTask(name string, raw any) (Task, error) {
	task := Task{Name: name}
	if object, ok := raw.(map[string]any); ok {
		policy := make(map[string]any, len(object))
		for key, value := range object {
			switch key {
			case "check":
				if task.IsCheck, ok = value.(bool); !ok {
					return Task{}, fmt.Errorf("check must be boolean")
				}
			case "cwd":
				if task.Cwd, _ = value.(string); !portablePath(task.Cwd) {
					return Task{}, fmt.Errorf("cwd must be a relative forward-slash path inside the project")
				}
			default:
				policy[key] = value
			}
		}
		raw = policy
	}
	check, err := ParseCheck(raw)
	if err != nil {
		return Task{}, err
	}
	task.Check = check
	return task, nil
}

func decodeVSCode(raw any) (*VSCode, error) {
	if raw == nil {
		return nil, nil
	}
	object, ok := raw.(map[string]any)
	if !ok || !Fields(object, "settings", "extensions", "tasks", "launch") {
		return nil, fmt.Errorf("wsup.json: vscode may contain only settings, extensions, tasks, and launch")
	}
	vscode := &VSCode{Settings: map[string]any{}, Extensions: []string{}, Tasks: []map[string]any{}, Launch: []map[string]any{}}
	if value, exists := object["settings"]; exists {
		if vscode.Settings, ok = value.(map[string]any); !ok {
			return nil, fmt.Errorf("wsup.json: vscode.settings must be an object")
		}
	}
	if value, exists := object["extensions"]; exists {
		if vscode.Extensions, ok = stringsValue(value); !ok {
			return nil, fmt.Errorf("wsup.json: vscode.extensions must be a list of extension ids")
		}
	}
	for _, list := range []struct {
		key    string
		target *[]map[string]any
	}{{"tasks", &vscode.Tasks}, {"launch", &vscode.Launch}} {
		value, exists := object[list.key]
		if !exists {
			continue
		}
		items, ok := value.([]any)
		if !ok {
			return nil, fmt.Errorf("wsup.json: vscode.%s must be a list of objects", list.key)
		}
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("wsup.json: vscode.%s must be a list of objects", list.key)
			}
			*list.target = append(*list.target, entry)
		}
	}
	return vscode, nil
}

// portablePath accepts "." or a clean relative forward-slash path that stays
// inside its base lexically. Load separately checks resolved containment.
func portablePath(name string) bool {
	return name != "" && path.Clean(name) == name && !path.IsAbs(name) &&
		name != ".." && !strings.HasPrefix(name, "../") && !strings.ContainsAny(name, "\\:\x00")
}

// Encode writes a document in contract key order with tasks in their order,
// two-space indentation, and a trailing newline. Only non-default task policy
// fields are written, so Decode(Encode(d)) returns d.
func Encode(document Document) ([]byte, error) {
	type project struct {
		ID           string   `json:"id"`
		Path         string   `json:"path"`
		Kind         string   `json:"kind"`
		Repository   string   `json:"repository"`
		Git          string   `json:"git,omitempty"`
		Purpose      string   `json:"purpose"`
		Owner        string   `json:"owner"`
		Dependencies []string `json:"dependencies"`
		Tasks        taskList `json:"tasks,omitempty"`
	}
	type vscode struct {
		Settings   map[string]any   `json:"settings"`
		Extensions []string         `json:"extensions"`
		Tasks      []map[string]any `json:"tasks"`
		Launch     []map[string]any `json:"launch"`
	}
	output := struct {
		Schema      string         `json:"$schema,omitempty"`
		Version     int            `json:"version"`
		Name        string         `json:"name"`
		Collections []string       `json:"collections"`
		Agents      []string       `json:"agents"`
		Editors     []string       `json:"editors"`
		Settings    map[string]any `json:"settings,omitempty"`
		Projects    []project      `json:"projects"`
		VSCode      *vscode        `json:"vscode,omitempty"`
	}{
		Schema: document.Schema, Version: DocumentVersion, Name: document.Name,
		Collections: nonNil(document.Collections), Agents: nonNil(document.Agents), Editors: nonNil(document.Editors),
		Settings: document.Settings, Projects: []project{},
	}
	for _, entry := range document.Projects {
		output.Projects = append(output.Projects, project{
			ID: entry.ID, Path: entry.Path, Kind: entry.Kind, Repository: entry.Repository, Git: entry.Git,
			Purpose: entry.Purpose, Owner: entry.Owner, Dependencies: nonNil(entry.Dependencies), Tasks: entry.Tasks,
		})
	}
	if document.VSCode != nil {
		output.VSCode = &vscode{
			Settings: document.VSCode.Settings, Extensions: nonNil(document.VSCode.Extensions),
			Tasks: document.VSCode.Tasks, Launch: document.VSCode.Launch,
		}
		if output.VSCode.Settings == nil {
			output.VSCode.Settings = map[string]any{}
		}
		if output.VSCode.Tasks == nil {
			output.VSCode.Tasks = []map[string]any{}
		}
		if output.VSCode.Launch == nil {
			output.VSCode.Launch = []map[string]any{}
		}
	}
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(output); err != nil {
		return nil, err
	}
	// Encoding is checked by decoding, so an invalid document is never written.
	if _, err := Decode(encoded.Bytes()); err != nil {
		return nil, fmt.Errorf("encode wsup.json: %w", err)
	}
	return encoded.Bytes(), nil
}

// taskList encodes tasks as one object in their order.
type taskList []Task

func (tasks taskList) MarshalJSON() ([]byte, error) {
	var encoded bytes.Buffer
	encoded.WriteByte('{')
	for index, task := range tasks {
		if index != 0 {
			encoded.WriteByte(',')
		}
		name, err := json.Marshal(task.Name)
		if err != nil {
			return nil, err
		}
		body, err := json.Marshal(task.fields())
		if err != nil {
			return nil, err
		}
		encoded.Write(name)
		encoded.WriteByte(':')
		encoded.Write(body)
	}
	encoded.WriteByte('}')
	return encoded.Bytes(), nil
}

// fields returns the task's command and only its non-default settings.
func (task Task) fields() any {
	check := task.Check
	fields := struct {
		Command        []string `json:"command"`
		Check          bool     `json:"check,omitempty"`
		Cwd            string   `json:"cwd,omitempty"`
		TimeoutSeconds *int     `json:"timeout_seconds,omitempty"`
		Mode           string   `json:"mode,omitempty"`
		Enabled        *bool    `json:"enabled,omitempty"`
		Reason         string   `json:"reason,omitempty"`
		// A pointer keeps an explicit empty list, which differs from the default.
		FindingExitCodes *[]int `json:"finding_exit_codes,omitempty"`
	}{Command: check.Command, Check: task.IsCheck, Cwd: task.Cwd, TimeoutSeconds: check.TimeoutSeconds, Reason: check.Reason}
	if check.Mode != "required" {
		fields.Mode = check.Mode
	}
	if !check.Enabled {
		fields.Enabled = &check.Enabled
	}
	if !slices.Equal(check.FindingExitCodes, []int{1}) {
		codes := slices.Clone(check.FindingExitCodes)
		if codes == nil {
			codes = []int{}
		}
		fields.FindingExitCodes = &codes
	}
	return fields
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
