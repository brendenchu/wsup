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
	state, err := scaffolding.LoadState(destination)
	if err != nil || state.State.Template != "project" {
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
	first, err := scaffolding.LoadState(project)
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
	child, err := scaffolding.LoadState(filepath.Join(directory, "child"))
	if err != nil || !reflect.DeepEqual(first.State, child.State) {
		t.Fatalf("child depended on relocated parent/checkout: %+v %v", child.State, err)
	}
	run("--root", filepath.Join(directory, "unrelated"), "init", "workspace")
	workspace := filepath.Join(directory, "workspace")
	if output := run("--root", workspace, "validate"); !strings.Contains(string(output), "(0 projects)") {
		t.Fatalf("workspace validation failed: %s", output)
	}
	if output := run("--root", workspace, "guidance"); !strings.Contains(string(output), `"errors": []`) {
		t.Fatalf("workspace guidance failed: %s", output)
	}
	run("--root", workspace, "editor")
	workspaceState, err := scaffolding.LoadState(workspace)
	if err != nil || workspaceState.State.Template != "workspace" || workspaceState.State.Version != 2 {
		t.Fatalf("workspace state: %+v %v", workspaceState.State, err)
	}
	movedWorkspace := filepath.Join(directory, "relocated-workspace")
	if err := os.Rename(workspace, movedWorkspace); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(movedWorkspace, "README.md"), []byte("Local customization, not a child default"), 0600); err != nil {
		t.Fatal(err)
	}
	parentState := workspaceState.State
	parentState.OmittedHomeFiles = []string{"packages/README.md", "tools/README.md"}
	stateBody, err := scaffolding.EncodeNativeState(parentState)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(movedWorkspace, filepath.FromSlash(scaffolding.StatePath)), stateBody, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(movedWorkspace, "tools/README.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(movedWorkspace, "packages/README.md"), []byte("Owner omitted home"), 0600); err != nil {
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
	next, err := scaffolding.LoadState(filepath.Join(directory, "child-workspace"))
	if err != nil || !reflect.DeepEqual(workspaceState.State, next.State) {
		t.Fatalf("child workspace inherited local modifications: %+v %v", next.State, err)
	}
	for _, name := range parentState.OmittedHomeFiles {
		if _, err := os.Stat(filepath.Join(directory, "child-workspace", filepath.FromSlash(name))); err != nil {
			t.Fatalf("child inherited omitted home: %s", name)
		}
	}
	run("--root", movedWorkspace, "init", "embedded-project", "--template", "project")
	embeddedProject, err := scaffolding.LoadState(filepath.Join(directory, "embedded-project"))
	if err != nil || !reflect.DeepEqual(first.State, embeddedProject.State) {
		t.Fatalf("local catalogue overrode embedded project: %+v %v", embeddedProject.State, err)
	}
	if output := run("--root", filepath.Join(directory, "child-workspace"), "guidance"); !strings.Contains(string(output), `"errors": []`) {
		t.Fatalf("child workspace guidance failed: %s", output)
	}
	preview := run("--root", movedWorkspace, "update", "--keep", "README.md")
	if !strings.Contains(string(preview), `"keep-customization"`) || !strings.Contains(string(preview), `"applied": false`) {
		t.Fatalf("standalone update preview failed: %s", preview)
	}
	applied := run("--root", movedWorkspace, "update", "--keep", "README.md", "--apply")
	if !strings.Contains(string(applied), `"applied": true`) {
		t.Fatalf("standalone update apply failed: %s", applied)
	}
	owner, err := os.ReadFile(filepath.Join(movedWorkspace, "README.md"))
	if err != nil || string(owner) != "Local customization, not a child default" {
		t.Fatal("standalone update replaced kept owner bytes")
	}
	run("init", "selected-workspace", "--collection", "archive", "--collection", "journal")
	selected := filepath.Join(directory, "selected-workspace")
	previewCollections := run("--root", selected, "collections", "--add", "research")
	if !strings.Contains(string(previewCollections), `"applied": false`) {
		t.Fatalf("collection preview: %s", previewCollections)
	}
	run("--root", selected, "collections", "--add", "research", "--apply")
	run("--root", selected, "update", "--apply")
	run("--root", selected, "guidance")
	selectedState, err := scaffolding.LoadState(selected)
	if err != nil || !reflect.DeepEqual(selectedState.State.Collections, []string{"apps", "archive", "infra", "journal", "packages", "research", "tools"}) {
		t.Fatalf("standalone selections: %+v %v", selectedState.State, err)
	}
	run("--root", selected, "init", "unselected-child")
	childState, err := scaffolding.LoadState(filepath.Join(directory, "unselected-child"))
	if err != nil || !reflect.DeepEqual(childState.State.Collections, []string{"apps", "infra", "packages", "tools"}) {
		t.Fatalf("child inherited collection defaults: %+v %v", childState.State, err)
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
	for name, want := range map[string][]string{"all": {"claude", "gemini"}, "none": nil, "gemini": {"gemini"}, "project": {"claude"}} {
		loaded, err := scaffolding.LoadState(filepath.Join(parent, name))
		if err != nil || !reflect.DeepEqual(loaded.State.Agents, want) {
			t.Fatalf("%s agents: %v %v", name, loaded.State.Agents, err)
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
	loaded, err := scaffolding.LoadState(filepath.Join(parent, "project"))
	if err != nil || len(loaded.State.Editors) != 0 {
		t.Fatalf("project editors: %+v %v", loaded.State, err)
	}
	// wsup editor writes the same bytes the adapter generated.
	adapter, err := os.ReadFile(filepath.Join(parent, "default", "my-workspace.code-workspace"))
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--root", filepath.Join(parent, "default"), "editor", "--output", "copy.code-workspace"}, &stdout, &stderr); code != 0 {
		t.Fatalf("editor: %s", &stderr)
	}
	if copied, err := os.ReadFile(filepath.Join(parent, "default", "copy.code-workspace")); err != nil || !bytes.Equal(copied, adapter) {
		t.Fatalf("editor output differs from adapter: %s %v", copied, err)
	}
}
