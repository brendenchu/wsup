// Package guidance inspects local instructions and skills without invoking an AI client.
package guidance

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/brendenchu/wsup/internal/registry"
)

// Skill identifies valid metadata found at an owner-relative path.
type Skill struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Result separates structural findings from operational errors returned by Inspect.
type Result struct {
	Files  []string `json:"files"`
	Skills []Skill  `json:"skills"`
	Errors []string `json:"errors"`
}

var (
	linkPattern   = regexp.MustCompile(`\]\(([^)]+)\)`)
	schemePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
	skillName     = regexp.MustCompile(`^[a-z0-9-]{1,63}$`)
	// importPattern matches a whole-line @path import (CLAUDE.md, GEMINI.md).
	importPattern = regexp.MustCompile(`(?m)^@([^\s@]+\.md)\s*$`)
	markerPattern = regexp.MustCompile(`<!-- wsup:(begin|end) ([a-z][a-z0-9-]*) -->`)
)

func regular(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.Mode().IsRegular()
}

func markdown(root string) ([]string, error) {
	files := []string{}
	info, err := os.Stat(root)
	if os.IsNotExist(err) {
		return files, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return files, nil
	}
	// rglob traverses a symlink used as its root, but not symlink descendants.
	// Keep lexical paths so owner checks and output still describe the link.
	root += string(filepath.Separator)
	err = filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, name)
		}
		return nil
	})
	return files, err
}

// Inspect checks the required AGENTS.md, local file references in the root and
// top-level folder AGENTS.md files and skills, simple skill metadata, and
// generated-section markers. It neither fetches external links nor establishes
// that a client discovers or follows the instructions.
func Inspect(root string) (Result, error) {
	resolvedRoot, err := registry.Resolve(root)
	if err != nil {
		return Result{}, err
	}
	root = resolvedRoot
	result := Result{Files: []string{}, Skills: []Skill{}, Errors: []string{}}
	if !regular(filepath.Join(root, "AGENTS.md")) {
		result.Errors = append(result.Errors, "Missing AGENTS.md")
	}
	// Agent adapter files point at AGENTS.md; inspect them when present.
	candidates := []string{filepath.Join(root, "AGENTS.md"), filepath.Join(root, "CLAUDE.md"), filepath.Join(root, "GEMINI.md")}
	// Folder rules live one level down (collections, docs, workbench). Deeper
	// AGENTS.md files belong to projects, inspected with --project.
	entries, err := os.ReadDir(root)
	if err != nil {
		return Result{}, err
	}
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			candidates = append(candidates, filepath.Join(root, entry.Name(), "AGENTS.md"))
		}
	}
	files, err := markdown(filepath.Join(root, ".agents/skills"))
	if err != nil {
		return Result{}, err
	}
	candidates = append(candidates, files...)
	seen := map[string]bool{}
	for _, name := range candidates {
		if !regular(name) {
			continue
		}
		resolved, err := registry.Resolve(name)
		if err != nil {
			return Result{}, err
		}
		if !registry.Within(root, resolved) {
			result.Errors = append(result.Errors, "Guidance symlink escapes owner")
			continue
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return Result{}, err
		}
		relative = filepath.ToSlash(relative)
		if !seen[relative] {
			result.Files = append(result.Files, relative)
			seen[relative] = true
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return Result{}, err
		}
		if !utf8.Valid(body) {
			return Result{}, fmt.Errorf("%s: guidance must be UTF-8", relative)
		}
		text := strings.ReplaceAll(string(body), "\r\n", "\n")
		text = strings.ReplaceAll(text, "\r", "\n")
		for _, match := range linkPattern.FindAllStringSubmatch(text, -1) {
			link := match[1]
			if schemePattern.MatchString(link) || strings.HasPrefix(link, "#") {
				continue
			}
			target := strings.SplitN(link, "#", 2)[0]
			if !filepath.IsAbs(target) {
				target = filepath.Dir(name) + string(filepath.Separator) + target
			}
			target, err = registry.Resolve(target)
			if err != nil {
				return Result{}, err
			}
			if !registry.Within(root, target) || !regular(target) {
				result.Errors = append(result.Errors, relative+": unresolved/nonlocal reference")
			}
		}
		for _, match := range importPattern.FindAllStringSubmatch(text, -1) {
			target, err := registry.Resolve(filepath.Join(filepath.Dir(name), match[1]))
			if err != nil {
				return Result{}, err
			}
			if !registry.Within(root, target) || !regular(target) {
				result.Errors = append(result.Errors, relative+": unresolved @ import")
			}
		}
		if filepath.Base(name) == "AGENTS.md" {
			if problem := markerProblem(text); problem != "" {
				result.Errors = append(result.Errors, relative+": "+problem)
			}
		}
		if filepath.Base(name) == "SKILL.md" {
			skill, ok := parseSkill(text, filepath.Base(filepath.Dir(name)))
			if !ok {
				result.Errors = append(result.Errors, relative+": expected simple name/description frontmatter and body")
			} else {
				result.Skills = append(result.Skills, Skill{Name: skill, Path: relative})
			}
		}
	}
	sort.Strings(result.Files)
	// Native findings have a stable order independent of filesystem enumeration.
	sort.Strings(result.Errors)
	sort.SliceStable(result.Skills, func(i, j int) bool { return result.Skills[i].Name < result.Skills[j].Name })
	return result, nil
}

// markerProblem reports generated-section markers that do not pair up: every
// begin needs a matching end, sections never nest, and each name appears once.
func markerProblem(text string) string {
	open, seen := "", map[string]bool{}
	for _, match := range markerPattern.FindAllStringSubmatch(text, -1) {
		kind, name := match[1], match[2]
		switch {
		case kind == "begin" && open != "":
			return "wsup section " + name + " starts inside " + open
		case kind == "begin" && seen[name]:
			return "wsup section " + name + " appears twice"
		case kind == "begin":
			open, seen[name] = name, true
		case open != name:
			return "wsup section end " + name + " has no matching begin"
		default:
			open = ""
		}
	}
	if open != "" {
		return "wsup section " + open + " is not closed"
	}
	return ""
}

// Preserve the baseline's simple frontmatter contract, not general YAML parsing.
func parseSkill(text, directory string) (string, bool) {
	if !strings.HasPrefix(text, "---\n") {
		return "", false
	}
	parts := strings.SplitN(text[4:], "\n---\n", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return "", false
	}
	fields := map[string]string{}
	for _, line := range strings.Split(parts[0], "\n") {
		pair := strings.SplitN(line, ": ", 2)
		if len(pair) != 2 {
			return "", false
		}
		fields[pair[0]] = pair[1]
	}
	name := fields["name"]
	return name, name == directory && skillName.MatchString(name) &&
		strings.TrimSpace(fields["description"]) != ""
}
