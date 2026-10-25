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
		{"init"}, {"init", "a", "b"}, {"init", filepath.Join(parent, "workspace")},
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
}
