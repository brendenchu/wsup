package cli

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNativeSourceBuildAndGenerationAreSelfContained(t *testing.T) {
	source, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	checkout := t.TempDir()
	for _, subtree := range []string{
		"cmd", "internal", "templates", "go.mod", "go.sum",
		".gitignore", "LICENSE", "AI.md", "AGENTS.md", "README.md", "CONTRIBUTING.md", "CHANGELOG.md",
		".agents", "guidance", "docs", "workspace.json",
	} {
		err := filepath.WalkDir(filepath.Join(source, subtree), func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(source, name)
			if err != nil {
				return err
			}
			target := filepath.Join(checkout, relative)
			if entry.IsDir() {
				return os.MkdirAll(target, 0755)
			}
			if !entry.Type().IsRegular() {
				t.Fatalf("nonregular native build input: %s", relative)
			}
			if strings.HasSuffix(name, "_test.go") {
				return nil
			}
			body, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			return os.WriteFile(target, body, 0600)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"src", ".wsup", "internal/catalogue/assets.zip", "internal/catalogue/native"} {
		if _, err := os.Lstat(filepath.Join(checkout, filepath.FromSlash(name))); !os.IsNotExist(err) {
			t.Fatalf("native source build retained reference/overlay input: %s", name)
		}
	}
	binary := filepath.Join(t.TempDir(), "wsup")
	build := exec.Command("go", "build", "-o", binary, "./cmd/wsup")
	build.Dir = checkout
	build.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("offline native-only source build: %s %v", output, err)
	}
	run := func(args ...string) {
		t.Helper()
		command := exec.Command(binary, args...)
		command.Dir = t.TempDir()
		command.Env = []string{"PATH="}
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("native-only binary %v: %s %v", args, output, err)
		}
	}
	workspace, child, project := filepath.Join(t.TempDir(), "workspace"), filepath.Join(t.TempDir(), "child"), filepath.Join(t.TempDir(), "project")
	run("init", workspace, "--collection", "journal")
	run("--root", workspace, "validate")
	run("--root", workspace, "guidance")
	run("--root", workspace, "collections", "--add", "archive", "--apply")
	run("--root", workspace, "update", "--apply")
	run("--root", workspace, "init", child)
	run("--root", child, "guidance")
	run("--root", workspace, "init", project, "--template", "project")
	run("--root", project, "guidance")
}
