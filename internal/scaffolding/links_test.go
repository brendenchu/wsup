package scaffolding

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func agentWorkspace(t *testing.T, agents ...string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workspace")
	if _, err := InitialiseWith(root, "0.1.0", "workspace", nil, agents); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestAgentAdaptersAndTheSkillsLink(t *testing.T) {
	root := agentWorkspace(t, "claude", "gemini")
	for name, want := range map[string]string{"CLAUDE.md": "@AGENTS.md\n", "GEMINI.md": "@AGENTS.md\n"} {
		if body, err := os.ReadFile(filepath.Join(root, name)); err != nil || string(body) != want {
			t.Fatalf("%s: %q %v", name, body, err)
		}
	}
	if target, err := os.Readlink(filepath.Join(root, ".claude/skills")); err != nil || target != "../.agents/skills" {
		t.Fatalf("link: %q %v", target, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/skills/check-workspace/SKILL.md")); err != nil {
		t.Fatalf("link does not resolve to the skills: %v", err)
	}
	loaded, err := LoadState(root)
	if err != nil || !reflect.DeepEqual(loaded.State.Agents, []string{"claude", "gemini"}) ||
		!reflect.DeepEqual(loaded.State.Links, map[string]string{".claude/skills": "../.agents/skills"}) {
		t.Fatalf("state: %+v %v", loaded.State, err)
	}
	if _, managed := loaded.State.Files[".claude/skills"]; managed {
		t.Fatal("link recorded as a file")
	}
	if findings, err := CheckLinks(root); err != nil || len(findings) != 0 {
		t.Fatalf("fresh links: %v %v", findings, err)
	}
	if plan, err := Update(root, "0.1.0", nil, false); err != nil || len(plan.Actions) != 0 || len(plan.Conflicts) != 0 {
		t.Fatalf("fresh workspace does not settle: %+v %v", plan, err)
	}

	// A missing link is reported, then re-created by update.
	if err := os.Remove(filepath.Join(root, ".claude/skills")); err != nil {
		t.Fatal(err)
	}
	if findings, _ := CheckLinks(root); !reflect.DeepEqual(findings, []string{"Link .claude/skills is missing"}) {
		t.Fatalf("missing link findings: %v", findings)
	}
	plan, err := Update(root, "0.1.0", nil, true)
	if err != nil || !plan.Applied || !reflect.DeepEqual(plan.Actions, []Action{{".claude/skills", "link"}}) {
		t.Fatalf("re-create link: %+v %v", plan, err)
	}
	if findings, err := CheckLinks(root); err != nil || len(findings) != 0 {
		t.Fatalf("re-created link: %v %v", findings, err)
	}

	// A link pointing elsewhere or a real directory is a conflict, left alone.
	for name, occupy := range map[string]func(string) error{
		"elsewhere": func(link string) error { return os.Symlink("../docs", link) },
		"directory": func(link string) error { return os.Mkdir(link, 0755) },
	} {
		link := filepath.Join(root, ".claude/skills")
		if err := os.RemoveAll(link); err != nil {
			t.Fatal(err)
		}
		if err := occupy(link); err != nil {
			t.Fatal(err)
		}
		before := snapshot(t, root)
		plan, err := Update(root, "0.1.0", nil, true)
		if err == nil || plan.Applied || !reflect.DeepEqual(plan.Conflicts, []string{".claude/skills"}) || !reflect.DeepEqual(before, snapshot(t, root)) {
			t.Fatalf("%s: %+v %v", name, plan, err)
		}
		if findings, _ := CheckLinks(root); len(findings) != 1 || !strings.HasPrefix(findings[0], "Link .claude/skills ") {
			t.Fatalf("%s findings: %v", name, findings)
		}
	}
	if err := os.RemoveAll(filepath.Join(root, ".claude/skills")); err != nil {
		t.Fatal(err)
	}
	if _, err := Update(root, "0.1.0", nil, true); err != nil {
		t.Fatal(err)
	}

	// The relative link survives relocation.
	moved := filepath.Join(t.TempDir(), "moved")
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	if findings, err := CheckLinks(moved); err != nil || len(findings) != 0 {
		t.Fatalf("relocated link: %v %v", findings, err)
	}
	if _, err := os.Stat(filepath.Join(moved, ".claude/skills/check-workspace/SKILL.md")); err != nil {
		t.Fatalf("relocated link does not resolve: %v", err)
	}
}

func TestAgentsAreNeverAddedSilently(t *testing.T) {
	root := agentWorkspace(t)
	for _, name := range []string{"CLAUDE.md", "GEMINI.md", ".claude"} {
		if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("no-agent workspace has %s", name)
		}
	}
	loaded, err := LoadState(root)
	if err != nil || len(loaded.State.Agents) != 0 || loaded.State.Links != nil {
		t.Fatalf("no-agent state: %+v %v", loaded.State, err)
	}
	// A state without agents (including every earlier workspace) stays that way.
	if plan, err := Update(root, "0.1.0", nil, false); err != nil || len(plan.Actions) != 0 {
		t.Fatalf("update added adapters: %+v %v", plan, err)
	}
	project := filepath.Join(t.TempDir(), "project")
	if _, err := InitialiseWith(project, "0.1.0", "project", nil, []string{"claude"}); err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(filepath.Join(project, ".claude/skills")); err != nil || target != "../.agents/skills" {
		t.Fatalf("project link: %q %v", target, err)
	}
	if _, err := InitialiseWith(filepath.Join(t.TempDir(), "bad"), "0.1.0", "workspace", nil, []string{"unknown"}); err == nil {
		t.Fatal("unknown agent accepted")
	}
}
