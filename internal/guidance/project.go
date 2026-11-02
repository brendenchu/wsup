package guidance

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/wavykit/wsup/internal/registry"
)

// OwnedSkill preserves provenance when owners have skills with the same name.
type OwnedSkill struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Owner string `json:"owner"`
}

// ProjectResult contains presence metadata, not client-discovery guarantees.
type ProjectResult struct {
	Instructions []string
	Skills       []OwnedSkill
}

// InspectProject inspects only the ancestor chain and the workspace/project
// skill roots. It does not read instruction contents or follow their references.
func InspectProject(root, projectPath string) (ProjectResult, error) {
	root, err := registry.Resolve(root)
	if err != nil {
		return ProjectResult{}, err
	}
	project, err := registry.RelativeTarget(root, projectPath)
	if err != nil {
		return ProjectResult{}, err
	}
	owner, err := registry.Resolve(project)
	if err != nil {
		return ProjectResult{}, err
	}
	directories := ancestorChain(root, project)
	for _, directory := range directories {
		resolved, err := registry.Resolve(directory)
		if err != nil {
			return ProjectResult{}, err
		}
		if !registry.Within(root, resolved) {
			return ProjectResult{}, fmt.Errorf("%s: ancestor symlink escapes the workspace", relativePath(root, directory))
		}
		// Check physical ancestors too: an in-root symlink must not hide a
		// nested workspace that is outside the lexical ancestor chain.
		for _, candidate := range ancestorChain(root, resolved)[1:] {
			_, err := os.Lstat(filepath.Join(candidate, registry.DocumentPath))
			if err == nil {
				return ProjectResult{}, fmt.Errorf("%s: nested workspace; select that workspace explicitly with --root", relativePath(root, candidate))
			}
			if !os.IsNotExist(err) {
				return ProjectResult{}, err
			}
		}
	}
	result := ProjectResult{Instructions: []string{}, Skills: []OwnedSkill{}}
	for _, directory := range directories {
		boundary := root
		if directory == project {
			boundary = owner
		}
		name := filepath.Join(directory, "AGENTS.md")
		exists, err := guidanceCandidate(boundary, name, false)
		if err != nil {
			return ProjectResult{}, fmt.Errorf("%s: %w", relativePath(root, name), err)
		}
		if exists {
			result.Instructions = append(result.Instructions, relativePath(root, name))
		}
	}
	owners := []string{root}
	if project != root {
		owners = append(owners, project)
	}
	for _, directory := range owners {
		skills, err := projectSkills(root, directory)
		if err != nil {
			return ProjectResult{}, err
		}
		result.Skills = append(result.Skills, skills...)
	}
	sort.Slice(result.Skills, func(i, j int) bool {
		a, b := result.Skills[i], result.Skills[j]
		if a.Owner != b.Owner {
			return a.Owner < b.Owner
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Path < b.Path
	})
	return result, nil
}

func ancestorChain(root, target string) []string {
	relative, _ := filepath.Rel(root, target)
	result := []string{root}
	if relative != "." {
		for _, part := range strings.Split(relative, string(filepath.Separator)) {
			result = append(result, filepath.Join(result[len(result)-1], part))
		}
	}
	return result
}

func relativePath(root, name string) string {
	relative, _ := filepath.Rel(root, name)
	return filepath.ToSlash(relative)
}

// guidanceCandidate distinguishes absence from broken links and validates
// containment before any directory enumeration or file-content read.
func guidanceCandidate(owner, name string, directory bool) (bool, error) {
	if _, err := os.Lstat(name); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	resolved, err := registry.Resolve(name)
	if err != nil {
		return false, err
	}
	if !registry.Within(owner, resolved) {
		return false, fmt.Errorf("guidance path or symlink escapes owner")
	}
	info, err := os.Stat(name)
	if err != nil {
		return false, err
	}
	if directory && !info.IsDir() {
		return false, fmt.Errorf("expected a directory")
	}
	if !directory && !info.Mode().IsRegular() {
		return false, fmt.Errorf("expected a regular file")
	}
	return true, nil
}

func projectSkills(root, ownerPath string) ([]OwnedSkill, error) {
	owner, err := registry.Resolve(ownerPath)
	if err != nil {
		return nil, err
	}
	directory := ownerPath
	for _, part := range []string{".agents", "skills"} {
		directory = filepath.Join(directory, part)
		exists, err := guidanceCandidate(owner, directory, true)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", relativePath(root, directory), err)
		}
		if !exists {
			return []OwnedSkill{}, nil
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	result := []OwnedSkill{}
	for _, entry := range entries {
		// Only direct skill directories are candidates, not arbitrary Markdown.
		if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			continue
		}
		skillDirectory := filepath.Join(directory, entry.Name())
		if _, err := guidanceCandidate(owner, skillDirectory, true); err != nil {
			return nil, fmt.Errorf("%s: %w", relativePath(root, skillDirectory), err)
		}
		name := filepath.Join(skillDirectory, "SKILL.md")
		exists, err := guidanceCandidate(owner, name, false)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", relativePath(root, name), err)
		}
		if !exists {
			return nil, fmt.Errorf("%s: missing SKILL.md", relativePath(root, skillDirectory))
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return nil, err
		}
		text := strings.ReplaceAll(strings.ReplaceAll(string(body), "\r\n", "\n"), "\r", "\n")
		skill, valid := parseSkill(text, entry.Name())
		if !utf8.Valid(body) || !valid {
			return nil, fmt.Errorf("%s: expected simple name/description frontmatter and body", relativePath(root, name))
		}
		result = append(result, OwnedSkill{Name: skill, Path: relativePath(root, name), Owner: relativePath(root, ownerPath)})
	}
	return result, nil
}
