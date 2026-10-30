// Package registry validates workspace ownership and check definitions without executing them.
package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// checkFields are the fields of a check object (and of a task, with taskFields).
var checkFields = []string{"command", "mode", "enabled", "reason", "timeout_seconds", "finding_exit_codes"}

// Project is a loaded wsup.json project whose paths were checked against the
// filesystem.
type Project struct {
	ID           string
	Path         string
	Dependencies []string
	// Tasks keep source order; check tasks run in this order.
	Tasks []Task
	// Entry is the project as declared, for listings.
	Entry ProjectEntry
}

// Check is a validated literal argument array plus execution policy.
// A nil TimeoutSeconds means to use the workspace timeout.
type Check struct {
	Command          []string
	Mode             string
	Enabled          bool
	Reason           string
	TimeoutSeconds   *int
	FindingExitCodes []int
}

// Registry preserves project order; dependencies describe relationships, not execution order.
type Registry struct {
	Name     string
	Projects []Project
}

// Selection pairs one selected project check task with its validated policy.
type Selection struct {
	Project Project
	Name    string
	Check   Check
	// Cwd is the task's directory relative to the project; empty is its root.
	Cwd string
}

// Select returns check tasks (tasks with "check": true) in request order,
// deduplicated, validating the entire selection first. all selects every
// project that has check tasks instead of requested; empty names selects every
// check task. Errors return no partial selection, and dependencies are never
// selected implicitly.
func (r Registry) Select(requested, names []string, all bool) ([]Selection, error) {
	if all {
		requested = []string{}
		for _, project := range r.Projects {
			if slices.ContainsFunc(project.Tasks, func(task Task) bool {
				return task.IsCheck && (len(names) == 0 || slices.Contains(names, task.Name))
			}) {
				requested = append(requested, project.ID)
			}
		}
		if len(requested) == 0 {
			return nil, fmt.Errorf("no project has the selected check tasks")
		}
	}
	selected := []Selection{}
	seenProjects := map[string]bool{}
	for _, id := range requested {
		if seenProjects[id] {
			continue
		}
		seenProjects[id] = true
		index := slices.IndexFunc(r.Projects, func(project Project) bool { return project.ID == id })
		if index < 0 {
			return nil, fmt.Errorf("Unknown project: %s", id)
		}
		project := r.Projects[index]
		chosen := names
		if len(chosen) == 0 || all {
			chosen = []string{}
			for _, task := range project.Tasks {
				if task.IsCheck && (len(names) == 0 || slices.Contains(names, task.Name)) {
					chosen = append(chosen, task.Name)
				}
			}
		}
		if len(chosen) == 0 {
			return nil, fmt.Errorf("%s: no check tasks configured", id)
		}
		seenTasks := map[string]bool{}
		for _, name := range chosen {
			if seenTasks[name] {
				continue
			}
			seenTasks[name] = true
			task := slices.IndexFunc(project.Tasks, func(task Task) bool { return task.Name == name })
			if task < 0 {
				return nil, fmt.Errorf("%s: unknown task %s", id, name)
			}
			if !project.Tasks[task].IsCheck {
				return nil, fmt.Errorf("%s: task %s is not a check task", id, name)
			}
			selected = append(selected, Selection{Project: project, Name: name, Check: project.Tasks[task].Check, Cwd: project.Tasks[task].Cwd})
		}
	}
	return selected, nil
}

// Task returns one task of one project, check or not.
func (r Registry) Task(projectID, name string) (Project, Task, error) {
	index := slices.IndexFunc(r.Projects, func(project Project) bool { return project.ID == projectID })
	if index < 0 {
		return Project{}, Task{}, fmt.Errorf("Unknown project: %s", projectID)
	}
	project := r.Projects[index]
	task := slices.IndexFunc(project.Tasks, func(task Task) bool { return task.Name == name })
	if task < 0 {
		names := []string{}
		for _, task := range project.Tasks {
			names = append(names, task.Name)
		}
		if len(names) == 0 {
			return Project{}, Task{}, fmt.Errorf("%s: unknown task %s; the project has no tasks", projectID, name)
		}
		return Project{}, Task{}, fmt.Errorf("%s: unknown task %s; tasks: %s", projectID, name, strings.Join(names, ", "))
	}
	return project, project.Tasks[task], nil
}

// Load reads wsup.json at root with Decode, then checks every project path
// against the filesystem: it must resolve inside root, be unique, and contain
// README.md, and task directories must stay inside their project. Repository
// ownership remains declarative: no Git repositories are created or inspected.
func Load(root string) (Registry, error) {
	_, registry, err := LoadDocument(root)
	return registry, err
}

// LoadDocument is Load that also returns the decoded document.
func LoadDocument(root string) (Document, Registry, error) {
	filename := filepath.Join(root, DocumentPath)
	info, err := os.Lstat(filename)
	if err != nil {
		return Document{}, Registry{}, err
	}
	if !info.Mode().IsRegular() {
		return Document{}, Registry{}, fmt.Errorf("wsup.json must be a regular file")
	}
	body, err := os.ReadFile(filename)
	if err != nil {
		return Document{}, Registry{}, err
	}
	document, err := Decode(body)
	if err != nil {
		return Document{}, Registry{}, err
	}
	result := Registry{Name: document.Name, Projects: make([]Project, 0, len(document.Projects))}
	resolvedPaths := map[string]bool{}
	for _, entry := range document.Projects {
		target, err := RelativeTarget(root, entry.Path)
		if err != nil {
			return Document{}, Registry{}, fmt.Errorf("wsup.json: project %s: %w", entry.ID, err)
		}
		resolved, err := Resolve(target)
		if err != nil {
			return Document{}, Registry{}, err
		}
		info, statErr := os.Stat(target)
		readme, readmeErr := os.Stat(filepath.Join(target, "README.md"))
		if statErr != nil || !info.IsDir() || resolvedPaths[resolved] || readmeErr != nil || !readme.Mode().IsRegular() {
			return Document{}, Registry{}, fmt.Errorf("%s: path must be unique/existing and contain README.md", entry.ID)
		}
		resolvedPaths[resolved] = true
		for _, task := range entry.Tasks {
			if task.Cwd == "" {
				continue
			}
			if _, err := RelativeTarget(target, task.Cwd); err != nil {
				return Document{}, Registry{}, fmt.Errorf("wsup.json: project %s: task %s: %w", entry.ID, task.Name, err)
			}
		}
		result.Projects = append(result.Projects, Project{
			ID: entry.ID, Path: entry.Path, Dependencies: entry.Dependencies, Tasks: entry.Tasks, Entry: entry,
		})
	}
	return document, result, nil
}

// ReadObject requires a single JSON object and retains numbers as json.Number
// so validation can distinguish integer tokens from decimals and booleans.
func ReadObject(filename string) (map[string]any, error) {
	body, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return readObject(filename, body)
}

func readObject(filename string, body []byte) (map[string]any, error) {
	return ParseObject(filename, body)
}

// ParseObject validates one UTF-8 JSON object from already-read bytes.
func ParseObject(filename string, body []byte) (map[string]any, error) {
	if !utf8.Valid(body) {
		return nil, fmt.Errorf("%s: JSON must be UTF-8", filepath.Base(filename))
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("%s: invalid JSON: %w", filepath.Base(filename), err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("%s: unexpected trailing JSON", filepath.Base(filename))
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must contain an object", filepath.Base(filename))
	}
	return object, nil
}

// Fields rejects unknown keys; callers separately validate required fields and values.
func Fields(object map[string]any, names ...string) bool {
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	for name := range object {
		if !allowed[name] {
			return false
		}
	}
	return true
}

// Integer accepts only decoded integer tokens representable as a Go int.
func Integer(value any) (int, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	integer, err := number.Int64()
	if err != nil || int64(int(integer)) != integer {
		return 0, false
	}
	return int(integer), true
}

// Resolve follows existing symlink components even when the final path is absent.
// It processes ".." after preceding symlinks, unlike lexical filepath.Clean.
func Resolve(name string) (string, error) {
	if !filepath.IsAbs(name) {
		current, err := os.Getwd()
		if err != nil {
			return "", err
		}
		name = current + string(filepath.Separator) + name
	}
	return resolve(name, 0)
}

func resolve(name string, links int) (string, error) {
	if links > 255 {
		return "", fmt.Errorf("too many symlinks resolving %s", name)
	}
	parts := strings.Split(strings.TrimPrefix(name, filepath.VolumeName(name)), string(filepath.Separator))
	current := filepath.VolumeName(name) + string(filepath.Separator)
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			current = filepath.Dir(current)
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(current)
			if err != nil {
				return "", err
			}
			if !filepath.IsAbs(target) {
				target = filepath.Dir(current) + string(filepath.Separator) + target
			}
			current, err = resolve(target, links+1)
			if err != nil {
				return "", err
			}
		}
	}
	return filepath.Clean(current), nil
}

// Within checks lexical containment, including equality. Resolve both inputs first
// when containment must account for symlinks.
func Within(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// RelativeTarget validates the portable forward-slash path contract and resolved
// containment, returning the joined path rather than the resolved destination.
// This is a point-in-time check, not protection against concurrent filesystem changes.
func RelativeTarget(root string, value any) (string, error) {
	name, ok := value.(string)
	if !ok || name == "" || strings.ContainsAny(name, `\:`) {
		return "", fmt.Errorf("Use a nonempty relative forward-slash path")
	}
	if path.IsAbs(name) {
		return "", fmt.Errorf("Paths must stay inside the workspace")
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return "", fmt.Errorf("Paths must stay inside the workspace")
		}
	}
	target := filepath.Join(root, filepath.FromSlash(name))
	resolved, err := Resolve(target)
	if err != nil {
		return "", err
	}
	resolvedRoot, err := Resolve(root)
	if err != nil {
		return "", err
	}
	if !Within(resolvedRoot, resolved) {
		return "", fmt.Errorf("Path or symlink escapes the workspace")
	}
	return target, nil
}

func stringsValue(value any) ([]string, bool) {
	items, ok := value.([]any)
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, false
		}
		result = append(result, text)
	}
	return result, true
}

// ParseCheck accepts an argument array or policy object and applies
// defaults. Missing and null timeout both defer to workspace settings; other
// explicitly null policy fields are rejected.
func ParseCheck(raw any) (Check, error) {
	value, ok := raw.(map[string]any)
	if _, list := raw.([]any); list {
		value, ok = map[string]any{"command": raw}, true
	}
	if !ok || !Fields(value, checkFields...) {
		return Check{}, fmt.Errorf("Check must be an argument list or supported check object")
	}
	command, ok := stringsValue(value["command"])
	if !ok || len(command) == 0 || command[0] == "" {
		return Check{}, fmt.Errorf("Check command must be a nonempty list of literal string arguments")
	}
	for _, arg := range command {
		if strings.ContainsRune(arg, '\x00') {
			return Check{}, fmt.Errorf("Check command must be a nonempty list of literal string arguments")
		}
	}
	check := Check{Command: command, Mode: "required", Enabled: true, FindingExitCodes: []int{1}}
	if mode, exists := value["mode"]; exists {
		check.Mode, ok = mode.(string)
		if !ok {
			return Check{}, fmt.Errorf("Check mode must be required/report and enabled must be boolean")
		}
	}
	if enabled, exists := value["enabled"]; exists {
		check.Enabled, ok = enabled.(bool)
		if !ok {
			return Check{}, fmt.Errorf("Check mode must be required/report and enabled must be boolean")
		}
	}
	if check.Mode != "required" && check.Mode != "report" {
		return Check{}, fmt.Errorf("Check mode must be required/report and enabled must be boolean")
	}
	if reason, exists := value["reason"]; exists {
		check.Reason, ok = reason.(string)
		if !ok {
			return Check{}, fmt.Errorf("Check reason must be a string")
		}
	}
	if !check.Enabled && strings.TrimSpace(check.Reason) == "" {
		return Check{}, fmt.Errorf("Disabled checks need a reason")
	}
	if codes, exists := value["finding_exit_codes"]; exists {
		items, ok := codes.([]any)
		if !ok {
			return Check{}, fmt.Errorf("finding_exit_codes must contain exit codes 1..255")
		}
		check.FindingExitCodes = make([]int, 0, len(items))
		for _, item := range items {
			code, ok := Integer(item)
			if !ok || code < 1 || code > 255 {
				return Check{}, fmt.Errorf("finding_exit_codes must contain exit codes 1..255")
			}
			check.FindingExitCodes = append(check.FindingExitCodes, code)
		}
	}
	if rawTimeout := value["timeout_seconds"]; rawTimeout != nil {
		timeout, ok := Integer(rawTimeout)
		if !ok || timeout < 1 || timeout > 86400 {
			return Check{}, fmt.Errorf("Check timeout_seconds must be 1..86400")
		}
		check.TimeoutSeconds = &timeout
	}
	return check, nil
}

// checkDependencies requires every dependency to name a project and the graph
// to be acyclic, visiting projects in order so errors are deterministic.
func checkDependencies(order []string, dependencies map[string][]string) error {
	done, active := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		edges, exists := dependencies[id]
		if !exists {
			return fmt.Errorf("Unknown dependency: %s", id)
		}
		if active[id] {
			return fmt.Errorf("Dependency cycle at %s", id)
		}
		if done[id] {
			return nil
		}
		active[id] = true
		for _, dependency := range edges {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		delete(active, id)
		done[id] = true
		return nil
	}
	for _, id := range order {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func checkOrder(body []byte, index int) ([]string, error) {
	var document struct {
		Projects []struct {
			Checks json.RawMessage `json:"checks"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	return objectKeys(document.Projects[index].Checks)
}

// objectKeys returns a JSON object's distinct keys in source order, which Go
// maps do not keep.
func objectKeys(raw json.RawMessage) ([]string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	names := []string{}
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		name := token.(string)
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
	}
	return names, nil
}
