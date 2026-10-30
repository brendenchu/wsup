package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/scaffolding"
)

func TestProjectInitCLIAndRejectedArguments(t *testing.T) {
	parent := t.TempDir()
	for _, args := range [][]string{
		{"init"}, {"init", "a", "b"},
		{"init", filepath.Join(parent, "workspace"), "--collection", "unknown"},
		{"init", filepath.Join(parent, "bad"), "--template", "unknown"},
		{"init", filepath.Join(parent, "bad"), "--template", "project", "--collection", "archive"},
		{"init", filepath.Join(parent, "bad"), "--template", "project", "--invalid"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Fatalf("invalid init accepted: %v stdout=%s stderr=%s code=%d", args, &stdout, &stderr, code)
		}
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid arguments created destination")
	}
	destination := filepath.Join(parent, "project")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--root", filepath.Join(parent, "nonexistent-root"), "init", destination, "--template", "project"}, &stdout, &stderr); code != 0 {
		t.Fatalf("project init exit %d: %s", code, &stderr)
	}
	if stdout.String() != "Created "+destination+"\n" || stderr.Len() != 0 {
		t.Fatalf("init output: %s %s", &stdout, &stderr)
	}
	state, err := scaffolding.LoadLock(destination)
	if err != nil || state.Lock.Starter != "project" {
		t.Fatalf("CLI state: %+v %v", state, err)
	}
}

func TestStandaloneProjectInitWithoutToolchainOrCheckout(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "wsup")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/wsup")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build native executable: %s %v", output, err)
	}
	directory := t.TempDir()
	run := func(args ...string) []byte {
		t.Helper()
		command := exec.Command(binary, args...)
		command.Dir = directory
		command.Env = []string{"PATH="}
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("standalone %v: %s %v", args, output, err)
		}
		return output
	}
	run("--root", filepath.Join(directory, "unrelated"), "init", "project", "--template", "project")
	project := filepath.Join(directory, "project")
	first, err := scaffolding.LoadLock(project)
	if err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(directory, "moved")
	if err := os.Rename(project, moved); err != nil {
		t.Fatal(err)
	}
	if output := run("--root", moved, "guidance"); !strings.Contains(string(output), `"errors": []`) {
		t.Fatalf("relocated project guidance failed: %s", output)
	}
	run("--root", moved, "init", "child", "--template", "project")
	child, err := scaffolding.LoadLock(filepath.Join(directory, "child"))
	if err != nil || !reflect.DeepEqual(first.Lock, child.Lock) {
		t.Fatalf("child depended on relocated parent/checkout: %+v %v", child.Lock, err)
	}
	run("--root", filepath.Join(directory, "unrelated"), "init", "workspace")
	workspace := filepath.Join(directory, "workspace")
	if output := run("--root", workspace, "validate"); !strings.Contains(string(output), "(0 projects)") {
		t.Fatalf("workspace validation failed: %s", output)
	}
	if output := run("--root", workspace, "guidance"); !strings.Contains(string(output), `"errors": []`) {
		t.Fatalf("workspace guidance failed: %s", output)
	}
	run("--root", workspace, "vscode")
	workspaceState, err := scaffolding.LoadLock(workspace)
	if err != nil || workspaceState.Lock.Starter != "workspace" || workspaceState.Lock.Version != scaffolding.LockVersion {
		t.Fatalf("workspace state: %+v %v", workspaceState.Lock, err)
	}
	movedWorkspace := filepath.Join(directory, "relocated-workspace")
	if err := os.Rename(workspace, movedWorkspace); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(movedWorkspace, "README.md"), []byte("Local customization, not a child default"), 0600); err != nil {
		t.Fatal(err)
	}
	localChanges := []string{"packages/README.md"}
	if err := os.WriteFile(filepath.Join(movedWorkspace, "packages/README.md"), []byte("Owner home"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(movedWorkspace, "templates/project"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(movedWorkspace, "templates/project/README.md"), []byte("Misleading local catalogue"), 0600); err != nil {
		t.Fatal(err)
	}
	run("--root", movedWorkspace, "validate")
	if output := run("--root", movedWorkspace, "guidance"); !strings.Contains(string(output), `"errors": []`) {
		t.Fatalf("relocated workspace guidance failed: %s", output)
	}
	run("--root", movedWorkspace, "init", "child-workspace")
	next, err := scaffolding.LoadLock(filepath.Join(directory, "child-workspace"))
	if err != nil || !reflect.DeepEqual(workspaceState.Lock, next.Lock) {
		t.Fatalf("child workspace inherited local modifications: %+v %v", next.Lock, err)
	}
	for _, name := range localChanges {
		if _, err := os.Stat(filepath.Join(directory, "child-workspace", filepath.FromSlash(name))); err != nil {
			t.Fatalf("child inherited a local change: %s", name)
		}
	}
	run("--root", movedWorkspace, "init", "embedded-project", "--template", "project")
	embeddedProject, err := scaffolding.LoadLock(filepath.Join(directory, "embedded-project"))
	if err != nil || !reflect.DeepEqual(first.Lock, embeddedProject.Lock) {
		t.Fatalf("local catalogue overrode embedded project: %+v %v", embeddedProject.Lock, err)
	}
	if output := run("--root", filepath.Join(directory, "child-workspace"), "guidance"); !strings.Contains(string(output), `"errors": []`) {
		t.Fatalf("child workspace guidance failed: %s", output)
	}
	preview := run("--root", movedWorkspace, "update", "--keep", "README.md", "--keep", "packages/README.md")
	if !strings.Contains(string(preview), `"keep-customization"`) || !strings.Contains(string(preview), `"applied": false`) {
		t.Fatalf("standalone update preview failed: %s", preview)
	}
	applied := run("--root", movedWorkspace, "update", "--keep", "README.md", "--keep", "packages/README.md", "--apply")
	if !strings.Contains(string(applied), `"applied": true`) {
		t.Fatalf("standalone update apply failed: %s", applied)
	}
	owner, err := os.ReadFile(filepath.Join(movedWorkspace, "README.md"))
	if err != nil || string(owner) != "Local customization, not a child default" {
		t.Fatal("standalone update replaced kept owner bytes")
	}
	run("init", "selected-workspace", "--collection", "archive", "--collection", "journal")
	selected := filepath.Join(directory, "selected-workspace")
	previewCollections := run("--root", selected, "add", "research")
	if !strings.Contains(string(previewCollections), `"applied": false`) {
		t.Fatalf("collection preview: %s", previewCollections)
	}
	run("--root", selected, "add", "research", "--apply")
	run("--root", selected, "update", "--apply")
	run("--root", selected, "guidance")
	selectedState, err := scaffolding.LoadLock(selected)
	if err != nil || !reflect.DeepEqual(lockSelection(t, selectedState.Lock).Collections, []string{"apps", "archive", "infra", "journal", "packages", "research", "tools"}) {
		t.Fatalf("standalone selections: %+v %v", selectedState.Lock, err)
	}
	run("--root", selected, "init", "unselected-child")
	childState, err := scaffolding.LoadLock(filepath.Join(directory, "unselected-child"))
	if err != nil || !reflect.DeepEqual(lockSelection(t, childState.Lock).Collections, []string{"apps", "infra", "packages", "tools"}) {
		t.Fatalf("child inherited collection defaults: %+v %v", childState.Lock, err)
	}
}

func TestInitAgentSelection(t *testing.T) {
	parent := t.TempDir()
	run := func(want int, args ...string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := Run(append([]string{"init"}, args...), &stdout, &stderr); code != want {
			t.Fatalf("%v: %d %s", args, code, &stderr)
		}
	}
	run(0, filepath.Join(parent, "all"))
	run(0, filepath.Join(parent, "none"), "--agent", "none")
	run(0, filepath.Join(parent, "gemini"), "--agent", "gemini", "--agent", "gemini")
	run(0, filepath.Join(parent, "project"), "--template", "project", "--agent", "claude")
	for _, args := range [][]string{
		{filepath.Join(parent, "bad"), "--agent", "unknown"},
		{filepath.Join(parent, "bad"), "--agent", "none", "--agent", "claude"},
	} {
		run(2, args...)
	}
	if _, err := os.Lstat(filepath.Join(parent, "bad")); !os.IsNotExist(err) {
		t.Fatal("invalid agent selection wrote output")
	}
	for name, want := range map[string][]string{"all": {"claude", "gemini"}, "none": {}, "gemini": {"gemini"}, "project": {"claude"}} {
		loaded, err := scaffolding.LoadLock(filepath.Join(parent, name))
		if err != nil || !reflect.DeepEqual(lockSelection(t, loaded.Lock).Agents, want) {
			t.Fatalf("%s agents: %v %v", name, lockSelection(t, loaded.Lock).Agents, err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--root", filepath.Join(parent, "all"), "validate"}, &stdout, &stderr); code != 0 {
		t.Fatalf("validate: %s", &stderr)
	}
	if err := os.Remove(filepath.Join(parent, "all/.claude/skills")); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"--root", filepath.Join(parent, "all"), "validate"}, &stdout, &stderr); code != 2 ||
		!strings.Contains(stderr.String(), "Link .claude/skills is missing") {
		t.Fatalf("validate missing link: %d %s", code, &stderr)
	}
	stdout.Reset()
	if code := Run([]string{"--root", filepath.Join(parent, "all"), "guidance"}, &stdout, &stderr); code != 1 ||
		!strings.Contains(stdout.String(), "Link .claude/skills is missing") {
		t.Fatalf("guidance missing link: %d %s", code, &stdout)
	}
}

func TestInitEditorSelection(t *testing.T) {
	parent := t.TempDir()
	run := func(want int, args ...string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := Run(append([]string{"init"}, args...), &stdout, &stderr); code != want {
			t.Fatalf("%v: %d %s", args, code, &stderr)
		}
	}
	run(0, filepath.Join(parent, "default"))
	run(0, filepath.Join(parent, "none"), "--editor", "none")
	run(0, filepath.Join(parent, "project"), "--template", "project")
	run(2, filepath.Join(parent, "bad"), "--template", "project", "--editor", "vscode")
	run(2, filepath.Join(parent, "bad"), "--editor", "unknown")
	if _, err := os.Lstat(filepath.Join(parent, "bad")); !os.IsNotExist(err) {
		t.Fatal("invalid editor selection wrote output")
	}
	for name, want := range map[string]bool{"default": true, "none": false} {
		_, err := os.Stat(filepath.Join(parent, name, "my-workspace.code-workspace"))
		if (err == nil) != want {
			t.Fatalf("%s editor file present=%v", name, err == nil)
		}
	}
	loaded, err := scaffolding.LoadLock(filepath.Join(parent, "project"))
	if err != nil || len(lockSelection(t, loaded.Lock).Editors) != 0 {
		t.Fatalf("project editors: %+v %v", loaded.Lock, err)
	}
	// wsup vscode finds the adapter's file already current.
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--root", filepath.Join(parent, "default"), "vscode"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), `"actions": []`) {
		t.Fatalf("vscode: %s %s", &stdout, &stderr)
	}
}
