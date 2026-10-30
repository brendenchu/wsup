package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/brendenchu/wsup/internal/scaffolding"
)

func TestAddCLIPreviewApplyAndErrors(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace")
	invoke := func(want int, args ...string) map[string]json.RawMessage {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := Run(append([]string{"--root", root}, args...), &stdout, &stderr)
		if code != want {
			t.Fatalf("%v: %d stdout=%s stderr=%s", args, code, &stdout, &stderr)
		}
		if want == 2 {
			if stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "Error: ") {
				t.Fatalf("invalid failure shape: %s %s", &stdout, &stderr)
			}
			return nil
		}
		var result map[string]json.RawMessage
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || stderr.Len() != 0 {
			t.Fatalf("JSON output: %s %s %v", &stdout, &stderr, err)
		}
		return result
	}
	invoke(2, "add")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"init", root, "--collection", "research", "--collection", "archive", "--collection", "research"}, &stdout, &stderr); code != 0 {
		t.Fatalf("selected init: %d %s", code, &stderr)
	}
	loaded, err := scaffolding.LoadLock(root)
	// The default preset combines with explicit collections.
	if err != nil || !reflect.DeepEqual(lockSelection(t, loaded.Lock).Collections, []string{"apps", "archive", "infra", "packages", "research", "tools"}) {
		t.Fatalf("selected init state: %+v %v", loaded, err)
	}
	before := workspaceBytes(t, root)
	preview := invoke(0, "add", "journal", "journal")
	if len(preview) != 5 || string(preview["applied"]) != "false" || !strings.Contains(string(preview["installs"]), "journal") {
		t.Fatalf("addition shape: %v", preview)
	}
	for _, args := range [][]string{
		{"add", "unknown", "--apply"}, {"add", "archive,journal"}, {"add", "development"},
		{"add", "workspace-base"}, {"add", "--invalid", "journal"}, {"collections"},
		{"init", filepath.Join(root, "bad"), "--template", "project", "--collection", "archive"},
		{"init", filepath.Join(root, "bad"), "--template", "project", "--preset", "none"},
		{"init", filepath.Join(root, "bad"), "--preset", "unknown"},
		{"init", filepath.Join(root, "bad"), "--preset", "none", "--collection", "unknown"},
	} {
		invoke(2, args...)
	}
	if !reflect.DeepEqual(before, workspaceBytes(t, root)) {
		t.Fatal("preview/invalid input changed bytes")
	}
	applied := invoke(0, "add", "journal", "--apply")
	if string(applied["applied"]) != "true" {
		t.Fatal("apply not reported")
	}
	loaded, err = scaffolding.LoadLock(root)
	if err != nil || !reflect.DeepEqual(lockSelection(t, loaded.Lock).Collections, []string{"apps", "archive", "infra", "journal", "packages", "research", "tools"}) {
		t.Fatalf("addition lost selections: %+v %v", loaded, err)
	}
	if err := os.MkdirAll(filepath.Join(root, "releases"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "releases/README.md"), []byte("owner"), 0600); err != nil {
		t.Fatal(err)
	}
	before = workspaceBytes(t, root)
	invoke(1, "add", "releases", "research")
	invoke(2, "add", "releases", "research", "--apply")
	if !reflect.DeepEqual(before, workspaceBytes(t, root)) {
		t.Fatal("conflicting addition wrote bytes")
	}
}

func TestModulesCLIListsAndShowsModules(t *testing.T) {
	run := func(want int, args ...string) []byte {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != want {
			t.Fatalf("%v: %d %s %s", args, code, &stdout, &stderr)
		}
		return stdout.Bytes()
	}
	var all, collections []map[string]string
	if err := json.Unmarshal(run(0, "modules"), &all); err != nil || len(all) != len(catalogue.Modules()) {
		t.Fatalf("all modules: %v %v", len(all), err)
	}
	if err := json.Unmarshal(run(0, "modules", "--kind", "collection"), &collections); err != nil || len(collections) != len(catalogue.CollectionNames()) {
		t.Fatalf("collections: %v %v", collections, err)
	}
	var detail struct {
		ID     string            `json:"id"`
		Skills []string          `json:"skills"`
		Files  []string          `json:"files"`
		Links  map[string]string `json:"links"`
	}
	if err := json.Unmarshal(run(0, "modules", "apps"), &detail); err != nil || detail.ID != "apps" ||
		!reflect.DeepEqual(detail.Files, []string{"apps/AGENTS.md", "apps/README.md"}) || len(detail.Skills) != 5 {
		t.Fatalf("apps detail: %+v %v", detail, err)
	}
	if err := json.Unmarshal(run(0, "modules", "claude"), &detail); err != nil || detail.Links[".claude/skills"] != "../.agents/skills" {
		t.Fatalf("claude detail: %+v %v", detail, err)
	}
	run(2, "modules", "unknown")
	run(2, "modules", "--kind", "plugin")
	run(2, "modules", "apps", "tools")
}

func TestAddCLIInstallsAdaptersInWorkspacesAndProjects(t *testing.T) {
	parent := t.TempDir()
	for _, tc := range []struct {
		args   []string
		add    []string
		exists []string
	}{
		{[]string{"--yes", "--agent", "none", "--editor", "none"}, []string{"claude", "vscode"}, []string{"CLAUDE.md", ".claude/skills/plan-work/SKILL.md", "my-workspace.code-workspace"}},
		{[]string{"--yes", "--agent", "none", "--template", "project"}, []string{"gemini"}, []string{"GEMINI.md"}},
	} {
		root := filepath.Join(parent, tc.add[0])
		var stdout, stderr bytes.Buffer
		if code := Run(append([]string{"init", root}, tc.args...), &stdout, &stderr); code != 0 {
			t.Fatalf("init: %s", &stderr)
		}
		stdout.Reset()
		if code := Run(append([]string{"--root", root, "add", "--apply"}, tc.add...), &stdout, &stderr); code != 0 {
			t.Fatalf("add %v: %s %s", tc.add, &stdout, &stderr)
		}
		for _, name := range tc.exists {
			if _, err := os.Stat(filepath.Join(root, name)); err != nil {
				t.Fatalf("%v: missing %s", tc.add, name)
			}
		}
		stdout.Reset()
		if code := Run([]string{"--root", root, "update"}, &stdout, &stderr); code != 0 || strings.Contains(stdout.String(), `"path"`) {
			t.Fatalf("addition did not settle: %s %s", &stdout, &stderr)
		}
	}
}
