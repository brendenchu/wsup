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
	"strings"
	"unicode/utf8"
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// Project retains validated fields and the original JSON shape for project listings.
type Project struct {
	ID           string
	Path         string
	Dependencies []string
	Checks       map[string]Check
	CheckNames   []string // Source order, independent of Go map iteration.
	Raw          map[string]any
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

// Selection pairs one explicitly selected project/check with its validated policy.
type Selection struct {
	Project Project
	Name    string
	Check   Check
}

// Select deduplicates selections in request order and validates the entire selection.
// Empty names selects all configured checks; errors return no partial selection.
// Dependencies are never selected implicitly.
func (r Registry) Select(requested, names []string) ([]Selection, error) {
	selected := []Selection{}
	seenProjects := map[string]bool{}
	for _, id := range requested {
		if seenProjects[id] {
			continue
		}
		seenProjects[id] = true
		var project *Project
		for i := range r.Projects {
			if r.Projects[i].ID == id {
				project = &r.Projects[i]
				break
			}
		}
		if project == nil {
			return nil, fmt.Errorf("Unknown project: %s", id)
		}
		chosen := names
		if len(chosen) == 0 {
			chosen = project.CheckNames
		}
		if len(chosen) == 0 {
			return nil, fmt.Errorf("%s: no checks configured", id)
		}
		seenChecks := map[string]bool{}
		for _, name := range chosen {
			if seenChecks[name] {
				continue
			}
			seenChecks[name] = true
			check, exists := project.Checks[name]
			if !exists {
				return nil, fmt.Errorf("%s: unknown check %s", id, name)
			}
			selected = append(selected, Selection{Project: *project, Name: name, Check: check})
		}
	}
	return selected, nil
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

// ParseCheck accepts an argument array or policy object and applies Python-compatible
// defaults. Missing and null timeout both defer to workspace settings; other
// explicitly null policy fields are rejected.
func ParseCheck(raw any) (Check, error) {
	value, ok := raw.(map[string]any)
	if _, list := raw.([]any); list {
		value, ok = map[string]any{"command": raw}, true
	}
	if !ok || !Fields(value, "command", "mode", "enabled", "reason", "timeout_seconds", "finding_exit_codes") {
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

// Load validates version 1 fields, project paths/README files, check policies,
// and dependency references/cycles. Repository ownership remains declarative:
// no Git repositories are created or inspected.
func Load(root string) (Registry, error) {
	filename := filepath.Join(root, "workspace.json")
	body, err := os.ReadFile(filename)
	if err != nil {
		return Registry{}, err
	}
	document, err := readObject(filename, body)
	if err != nil {
		return Registry{}, err
	}
	version, valid := Integer(document["version"])
	if !Fields(document, "version", "name", "projects") || !valid || version != 1 {
		return Registry{}, fmt.Errorf("workspace.json requires supported version 1 fields")
	}
	name, validName := document["name"].(string)
	entries, validEntries := document["projects"].([]any)
	if !validName || strings.TrimSpace(name) == "" || !validEntries {
		return Registry{}, fmt.Errorf("Registry needs a name and projects list")
	}
	result := Registry{Name: name, Projects: make([]Project, 0, len(entries))}
	ids, paths := map[string]bool{}, map[string]bool{}
	for _, entry := range entries {
		raw, ok := entry.(map[string]any)
		if !ok || !Fields(raw, "id", "purpose", "owner", "path", "kind", "repository", "dependencies", "checks") {
			return Registry{}, fmt.Errorf("Invalid project fields")
		}
		id, ok := raw["id"].(string)
		if !ok || !identifier.MatchString(id) || ids[id] {
			return Registry{}, fmt.Errorf("Project IDs must be unique letters/digits/dots/underscores/hyphens")
		}
		for _, field := range []string{"purpose", "owner", "path"} {
			text, ok := raw[field].(string)
			if !ok || strings.TrimSpace(text) == "" {
				return Registry{}, fmt.Errorf("%s: missing %s", id, field)
			}
		}
		kind, _ := raw["kind"].(string)
		repository, _ := raw["repository"].(string)
		if !map[string]bool{"app": true, "package": true, "tool": true, "infrastructure": true, "research": true, "template": true}[kind] ||
			(repository != "workspace" && repository != "independent") {
			return Registry{}, fmt.Errorf("%s: invalid kind/repository", id)
		}
		target, err := RelativeTarget(root, raw["path"])
		if err != nil {
			return Registry{}, err
		}
		resolved, err := Resolve(target)
		if err != nil {
			return Registry{}, err
		}
		info, statErr := os.Stat(target)
		readme, readmeErr := os.Stat(filepath.Join(target, "README.md"))
		if statErr != nil || !info.IsDir() || paths[resolved] || readmeErr != nil || !readme.Mode().IsRegular() {
			return Registry{}, fmt.Errorf("%s: path must be unique/existing and contain README.md", id)
		}
		project := Project{ID: id, Path: raw["path"].(string), Raw: raw, Checks: map[string]Check{}}
		if deps, exists := raw["dependencies"]; exists {
			project.Dependencies, ok = stringsValue(deps)
			if !ok {
				return Registry{}, fmt.Errorf("%s: dependencies must be project IDs", id)
			}
		}
		if checks, exists := raw["checks"]; exists {
			values, ok := checks.(map[string]any)
			if !ok {
				return Registry{}, fmt.Errorf("%s: checks must be named definitions", id)
			}
			// Decode order separately: JSON object iteration in Python defines check order.
			project.CheckNames, err = checkOrder(body, len(result.Projects))
			if err != nil {
				return Registry{}, err
			}
			for _, name := range project.CheckNames {
				if !identifier.MatchString(name) {
					return Registry{}, fmt.Errorf("%s: invalid check name", id)
				}
				check, err := ParseCheck(values[name])
				if err != nil {
					return Registry{}, err
				}
				project.Checks[name] = check
			}
		}
		ids[id], paths[resolved] = true, true
		result.Projects = append(result.Projects, project)
	}
	index := map[string]Project{}
	for _, project := range result.Projects {
		index[project.ID] = project
	}
	done, active := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		project, exists := index[id]
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
		for _, dependency := range project.Dependencies {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		delete(active, id)
		done[id] = true
		return nil
	}
	for _, project := range result.Projects {
		if err := visit(project.ID); err != nil {
			return Registry{}, err
		}
	}
	return result, nil
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
	decoder := json.NewDecoder(bytes.NewReader(document.Projects[index].Checks))
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
