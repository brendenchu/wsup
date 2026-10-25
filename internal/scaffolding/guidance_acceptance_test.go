package scaffolding

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/guidance"
)

func TestNativeGuidanceIsSelfContainedAndRoutesEveryCanonicalSkill(t *testing.T) {
	source, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	workspace := collectionWorkspace(t)
	project := filepath.Join(t.TempDir(), "project")
	if _, err := InitialiseProject(project, "0.1.0"); err != nil {
		t.Fatal(err)
	}
	contract := "Each workspace knows only itself and its descendants."
	links := regexp.MustCompile(`\]\(([^)]+)\)`)
	upward := regexp.MustCompile(`(?i)\bparent(?:'s)?\s+workspace\b`)
	for _, root := range []string{source, workspace, project} {
		inspection, err := guidance.Inspect(root)
		if err != nil || len(inspection.Errors) != 0 {
			t.Fatalf("guidance: %+v %v", inspection, err)
		}
		ai, err := os.ReadFile(filepath.Join(root, "AI.md"))
		if err != nil || !bytes.Contains(ai, []byte(contract)) {
			t.Fatalf("missing independence contract: %s", root)
		}
		if root == source {
			continue
		}
		err = filepath.WalkDir(root, func(name string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(name, ".md") {
				return err
			}
			body, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			if upward.Match(body) || bytes.Contains(body, []byte(source)) {
				t.Errorf("nonlocal guidance: %s", name)
			}
			for _, match := range links.FindAllSubmatch(body, -1) {
				target := string(match[1])
				if strings.Contains(target, "://") || strings.HasPrefix(target, "#") {
					continue
				}
				resolved := filepath.Join(filepath.Dir(name), strings.SplitN(target, "#", 2)[0])
				relative, err := filepath.Rel(root, resolved)
				if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
					t.Fatalf("escaping starter link: %s -> %s", name, target)
				}
				info, err := os.Stat(resolved)
				if err != nil || !info.Mode().IsRegular() {
					t.Fatalf("unresolved starter link: %s -> %s", name, target)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		boundary, err := os.ReadFile(filepath.Join(root, "guidance/rules/boundaries.md"))
		if err != nil || !bytes.Contains(boundary, []byte(contract)) {
			t.Fatal("starter boundary contract missing")
		}
	}
	index, err := os.ReadFile(filepath.Join(workspace, "guidance/rules/index.md"))
	if err != nil {
		t.Fatal(err)
	}
	skills, err := os.ReadDir(filepath.Join(source, ".agents/skills"))
	if err != nil {
		t.Fatal(err)
	}
	inspection, err := guidance.Inspect(workspace)
	if err != nil || len(skills) != 12 || len(inspection.Skills) != 12 {
		t.Fatalf("skill inventory: %+v %v", inspection, err)
	}
	for _, skill := range skills {
		name := skill.Name()
		if !slices.ContainsFunc(inspection.Skills, func(s guidance.Skill) bool { return s.Name == name }) ||
			!bytes.Contains(index, []byte("](../../.agents/skills/"+name+"/SKILL.md)")) {
			t.Fatalf("missing canonical skill or route: %s", name)
		}
	}
	rules, err := os.ReadDir(filepath.Join(source, "guidance/rules"))
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range rules {
		if !strings.HasSuffix(rule.Name(), ".md") {
			continue
		}
		want, err := os.ReadFile(filepath.Join(source, "guidance/rules", rule.Name()))
		if err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(filepath.Join(workspace, "guidance/rules", rule.Name()))
		if err != nil || !bytes.Equal(body, want) {
			t.Fatalf("canonical rule differs: %s", rule.Name())
		}
	}
}
