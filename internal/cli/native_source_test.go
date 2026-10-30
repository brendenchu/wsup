package cli

import (
	"bytes"
	"debug/buildinfo"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeSourceBuildAndGenerationAreSelfContained(t *testing.T) {
	source, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	checkout := t.TempDir()
	for _, subtree := range []string{
		"cmd", "internal", "schema", "templates", "go.mod", "go.sum",
		".gitignore", "LICENSE", "AGENTS.md", "CLAUDE.md", "README.md", "CONTRIBUTING.md", "CHANGELOG.md",
		"docs",
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
	for _, name := range []string{".wsup", ".git"} {
		if _, err := os.Lstat(filepath.Join(checkout, filepath.FromSlash(name))); !os.IsNotExist(err) {
			t.Fatalf("native source build retained workspace/repository state: %s", name)
		}
	}
	binary := filepath.Join(t.TempDir(), "wsup")
	build := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", binary, "./cmd/wsup")
	build.Dir = checkout
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("offline native-only source build: %s %v", output, err)
	}
	second := filepath.Join(t.TempDir(), "wsup")
	rebuild := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", second, "./cmd/wsup")
	rebuild.Dir, rebuild.Env = checkout, build.Env
	if output, err := rebuild.CombinedOutput(); err != nil {
		t.Fatalf("repeat release-profile build: %s %v", output, err)
	}
	firstBody, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	secondBody, err := os.ReadFile(second)
	if err != nil || !bytes.Equal(firstBody, secondBody) {
		t.Fatalf("release-profile binary changed between output directories: %v", err)
	}
	info, err := buildinfo.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	settings := map[string]string{}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
		if strings.HasPrefix(setting.Key, "vcs") {
			t.Fatalf("release-profile binary contains implicit VCS metadata: %s", setting.Key)
		}
	}
	if settings["CGO_ENABLED"] != "0" || settings["-trimpath"] != "true" ||
		bytes.Contains(firstBody, []byte(checkout)) {
		t.Fatalf("release-profile binary retained cgo or source path: %v", settings)
	}
	tools := t.TempDir()
	for _, name := range []string{"go", "git"} {
		executable, err := exec.LookPath(name)
		if err != nil {
			t.Fatalf("required development tool %s: %v", name, err)
		}
		if err := os.Symlink(executable, filepath.Join(tools, name)); err != nil {
			t.Fatal(err)
		}
	}
	test := exec.Command(filepath.Join(tools, "go"), "test", "-count=1", "-skip", "^TestNativeSourceBuildAndGenerationAreSelfContained$", "./...")
	test.Dir = checkout
	test.Env = append(os.Environ(), "PATH="+tools, "CGO_ENABLED=0", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local")
	if output, err := test.CombinedOutput(); err != nil {
		t.Fatalf("isolated native suite with only Go/Git on PATH: %s %v", output, err)
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
	run("--root", workspace, "add", "archive", "--apply")
	run("--root", workspace, "update", "--apply")
	run("--root", workspace, "init", child)
	run("--root", child, "guidance")
	run("--root", workspace, "init", project, "--template", "project")
	run("--root", project, "guidance")
}
