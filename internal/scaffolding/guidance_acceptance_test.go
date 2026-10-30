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
	contract := "knows only itself and its descendants."
	links := regexp.MustCompile(`\]\(([^)]+)\)`)
	upward := regexp.MustCompile(`(?i)\bparent(?:'s)?\s+workspace\b`)
	for _, root := range []string{workspace, baseOnly, project} {
		inspection, err := guidance.Inspect(root)
		if err != nil || len(inspection.Errors) != 0 {
			t.Fatalf("guidance: %+v %v", inspection, err)
		}
		agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if err != nil || !bytes.Contains(agents, []byte(contract)) {
			t.Fatalf("missing independence contract: %s", root)
		}
		for _, retired := range []string{"AI.md", "guidance", "WSUP-LICENSE"} {
			if _, err := os.Lstat(filepath.Join(root, retired)); !os.IsNotExist(err) {
				t.Fatalf("retired guidance file present: %s/%s", root, retired)
			}
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
	// A development workspace keeps its 12 skills (contract amendment 4).
	if len(inventory(workspace)) != 12 ||
		!slices.Equal(inventory(baseOnly), baseSkills) || !slices.Equal(inventory(project), projectSkills) {
		t.Fatalf("skill inventory: %v %v %v", inventory(workspace), inventory(baseOnly), inventory(project))
	}
	// Each AGENTS.md skills table links exactly the skills that are installed.
	for _, root := range []string{workspace, baseOnly, project} {
		agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		for _, skill := range skills {
			name := skill.Name()
			route := []byte("](.agents/skills/" + name + "/SKILL.md)")
			if slices.Contains(inventory(root), name) != bytes.Contains(agents, route) {
				t.Fatalf("skills table mismatch in %s: %s", root, name)
			}
		}
	}
	for _, name := range inventory(workspace) {
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
	// Each selected collection has its folder rules.
	for _, collection := range []string{"apps", "infra", "packages", "tools"} {
		if _, err := os.Stat(filepath.Join(workspace, collection, "AGENTS.md")); err != nil {
			t.Fatalf("missing folder rules: %s", collection)
		}
	}
}
