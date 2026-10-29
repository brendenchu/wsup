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
	templateSource := filepath.Join(source, "templates", "modules", "workspace-base", "files")
	skillSource := filepath.Join(source, "templates", "skills")
	workspace := collectionWorkspace(t, "apps", "infra", "packages", "tools")
	baseOnly := collectionWorkspace(t)
	project := filepath.Join(t.TempDir(), "project")
	if _, err := InitialiseProject(project, "0.1.0"); err != nil {
		t.Fatal(err)
	}
	contract := "Each workspace knows only itself and its descendants."
	links := regexp.MustCompile(`\]\(([^)]+)\)`)
	upward := regexp.MustCompile(`(?i)\bparent(?:'s)?\s+workspace\b`)
	for _, root := range []string{workspace, baseOnly, project} {
		inspection, err := guidance.Inspect(root)
		if err != nil || len(inspection.Errors) != 0 {
			t.Fatalf("guidance: %+v %v", inspection, err)
		}
		ai, err := os.ReadFile(filepath.Join(root, "AI.md"))
		if err != nil || !bytes.Contains(ai, []byte(contract)) {
			t.Fatalf("missing independence contract: %s", root)
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
	skills, err := os.ReadDir(skillSource)
	if err != nil {
		t.Fatal(err)
	}
	baseSkills := []string{"check-workspace", "maintain-agents-md", "onboard-project", "plan-work", "recap-work", "run-checks", "update-workspace"}
	projectSkills := []string{"diagnose-issue", "infer-conventions", "make-change", "plan-work", "review-change", "write-commit"}
	inventory := func(root string) []string {
		t.Helper()
		inspection, err := guidance.Inspect(root)
		if err != nil {
			t.Fatal(err)
		}
		names := []string{}
		for _, skill := range inspection.Skills {
			names = append(names, skill.Name)
		}
		return names
	}
	if len(skills) != 12 || len(inventory(workspace)) != 12 ||
		!slices.Equal(inventory(baseOnly), baseSkills) || !slices.Equal(inventory(project), projectSkills) {
		t.Fatalf("skill inventory: %v %v %v", inventory(workspace), inventory(baseOnly), inventory(project))
	}
	projectIndex, err := os.ReadFile(filepath.Join(project, "guidance/rules/index.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, skill := range skills {
		name := skill.Name()
		// Base skills are linked; collection skills are named, since a workspace
		// may not have installed them.
		route := []byte("](../../.agents/skills/" + name + "/SKILL.md)")
		if !slices.Contains(inventory(workspace), name) ||
			slices.Contains(baseSkills, name) != bytes.Contains(index, route) ||
			(!slices.Contains(baseSkills, name) && !bytes.Contains(index, []byte("`"+name+"`"))) {
			t.Fatalf("missing canonical skill or route: %s", name)
		}
		if slices.Contains(projectSkills, name) != bytes.Contains(projectIndex, route) {
			t.Fatalf("project route mismatch: %s", name)
		}
		want, err := os.ReadFile(filepath.Join(skillSource, name, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(filepath.Join(workspace, ".agents/skills", name, "SKILL.md"))
		if err != nil || !bytes.Equal(body, want) {
			t.Fatalf("canonical native skill differs: %s", name)
		}
	}
	wantModel, err := os.ReadFile(filepath.Join(templateSource, "docs/workspace-model.md"))
	if err != nil {
		t.Fatal(err)
	}
	model, err := os.ReadFile(filepath.Join(workspace, "docs/workspace-model.md"))
	if err != nil || !bytes.Equal(model, wantModel) {
		t.Fatal("canonical native workspace model differs")
	}
	rules, err := os.ReadDir(filepath.Join(templateSource, "guidance/rules"))
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range rules {
		if !strings.HasSuffix(rule.Name(), ".md") {
			continue
		}
		want, err := os.ReadFile(filepath.Join(templateSource, "guidance/rules", rule.Name()))
		if err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(filepath.Join(workspace, "guidance/rules", rule.Name()))
		if err != nil || !bytes.Equal(body, want) {
			t.Fatalf("canonical rule differs: %s", rule.Name())
		}
	}
}
