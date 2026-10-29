package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/scaffolding"
)

func fixtureGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-c", "core.excludesFile=/dev/null"}, args...)...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s %v", args, output, err)
	}
	return string(output)
}

func TestNativeStartersGitIgnoreBaselinesAndIndependentProjectLocalFiles(t *testing.T) {
	ignored := []string{
		"workbench/.local/note", "apps/example/workbench/.local/note",
		".work/note", "apps/example/.work/note", ".wsup/reports/check.json",
		"wsup.local.json", ".env", ".env.local", "apps/example/.env",
		".DS_Store", "apps/example/.DS_Store", "nested/.work/log",
	}
	trackable := []string{
		".env.example", "apps/example/.env.example", ".wsup/template-state.json",
		"workspace.json", "workbench/draft.md", "apps/example/README.md",
		"decisions/README.md", "WSUP-LICENSE", "README.md",
		"wsup", "wsup.exe", "wsup_0.2.0_darwin_arm64.tar.gz",
		"wsup_0.2.0_windows_amd64.zip", "SHA256SUMS",
	}
	license, err := os.ReadFile("../../LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"workspace", "project"} {
		t.Run(kind, func(t *testing.T) {
			sourceIgnore, err := os.ReadFile(filepath.Join("../../templates/modules", kind+"-base", "files", ".gitignore"))
			if err != nil {
				t.Fatal(err)
			}
			root := filepath.Join(t.TempDir(), kind)
			if _, err := scaffolding.Initialise(root, Version, kind); err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(filepath.Join(root, ".gitignore"))
			if err != nil || !bytes.Equal(body, sourceIgnore) {
				t.Fatal("starter ignore baseline changed")
			}
			body, err = os.ReadFile(filepath.Join(root, "WSUP-LICENSE"))
			if err != nil || !bytes.Equal(body, license) {
				t.Fatal("starter license changed")
			}
			if _, err := os.Stat(filepath.Join(root, ".git")); !os.IsNotExist(err) {
				t.Fatal("starter silently initialized Git")
			}
			fixtureGit(t, root, "init", "--quiet", ".")
			for _, name := range append(slices.Clone(ignored), ".env.example") {
				filename := filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filename, []byte("Synthetic local content"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.Command("git", "-c", "core.excludesFile=/dev/null", "check-ignore", "--no-index", "--stdin", "-z")
			command.Dir = root
			command.Stdin = strings.NewReader(strings.Join(append(slices.Clone(ignored), trackable...), "\x00") + "\x00")
			output, err := command.CombinedOutput()
			if err != nil || !reflect.DeepEqual(strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00"), ignored) {
				t.Fatalf("Git ignore contract: %q %v", output, err)
			}
		})
	}
}

func TestNativeGeneratedWorkspaceMixedGitOnboarding(t *testing.T) {
	t.Setenv("WSUP_ONBOARDING_HELPER", "1")
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "mixed workspace")
	run := func(args ...string) []byte {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := Run(append([]string{"--root", root}, args...), &stdout, &stderr); code != 0 || stderr.Len() != 0 {
			t.Fatalf("native %v: %d %s %s", args, code, &stdout, &stderr)
		}
		return stdout.Bytes()
	}
	run("init", root)
	fixtureGit(t, root, "init", "--quiet", ".")
	projects := []map[string]any{}
	for _, ownership := range []string{"workspace", "independent"} {
		name := "owned"
		if ownership == "independent" {
			name = ownership
		}
		project := filepath.Join(root, "apps", name)
		run("init", project, "--template", "project")
		projects = append(projects, map[string]any{
			"id": name, "path": "apps/" + name, "kind": "app", "repository": ownership,
			"purpose": "Onboarding fixture", "owner": "Test",
			"checks": map[string][]string{"test": {binary, "-test.run=^TestOnboardingTool$", "--", project}},
		})
	}
	ignore, err := os.OpenFile(filepath.Join(root, ".gitignore"), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := ignore.WriteString("\n/apps/independent/\n")
	closeErr := ignore.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf("ignore append: %v %v", writeErr, closeErr)
	}
	independent := filepath.Join(root, "apps/independent")
	fixtureGit(t, root, "init", "--quiet", independent)
	top := strings.TrimSpace(fixtureGit(t, independent, "rev-parse", "--show-toplevel"))
	want, err := filepath.EvalSymlinks(independent)
	if err != nil || top != want {
		t.Fatalf("independent repository ownership: %s %s %v", top, want, err)
	}
	if !strings.Contains(fixtureGit(t, root, "check-ignore", "-v", "apps/independent/"), "/apps/independent/") {
		t.Fatal("independent exact ownership ignore missing")
	}
	body, err := json.Marshal(map[string]any{"version": 1, "name": "Mixed fixture", "projects": projects})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "workspace.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	run("validate")
	var listed []map[string]any
	if err := json.Unmarshal(run("projects"), &listed); err != nil {
		t.Fatal(err)
	}
	actual, err := json.Marshal(listed)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := json.Marshal(projects)
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatalf("project listing changed: %s %s %v", actual, expected, err)
	}
	run("config")
	run("guidance")
	run("guidance", "--project", "independent")
	run("check", "--project", "owned", "--project", "independent")
	run("editor")
	body, err = os.ReadFile(filepath.Join(root, "workspace.code-workspace"))
	if err != nil {
		t.Fatal(err)
	}
	var editor struct {
		Folders []struct{ Path string }
	}
	if err := json.Unmarshal(body, &editor); err != nil {
		t.Fatal(err)
	}
	paths := []string{}
	for _, folder := range editor.Folders {
		paths = append(paths, folder.Path)
	}
	if !reflect.DeepEqual(paths, []string{".", "apps/owned", "apps/independent"}) {
		t.Fatalf("editor folders: %v", paths)
	}
	for _, name := range []string{".agents/skills", ".wsup/templates", "workspace.json"} {
		if _, err := os.Lstat(filepath.Join(independent, filepath.FromSlash(name))); !os.IsNotExist(err) {
			t.Fatalf("project inherited workspace runtime/guidance: %s", name)
		}
	}
	fixtureGit(t, root, "add", "--all")
	tracked := strings.Split(strings.TrimSpace(fixtureGit(t, root, "ls-files")), "\n")
	for _, name := range []string{"workspace.json", "apps/owned/README.md", ".wsup/template-state.json", ".agents/skills/project-onboarding/SKILL.md"} {
		if !slices.Contains(tracked, name) {
			t.Fatalf("workspace-owned file not staged: %s", name)
		}
	}
	for _, name := range tracked {
		if strings.HasPrefix(name, "apps/independent") || strings.HasPrefix(name, ".wsup/reports/") {
			t.Fatalf("nonowned or ignored report staged: %s", name)
		}
	}
	if strings.Contains(fixtureGit(t, root, "ls-files", "--stage"), "160000 ") {
		t.Fatal("independent repository staged as a submodule")
	}
}

func TestOnboardingTool(t *testing.T) {
	if os.Getenv("WSUP_ONBOARDING_HELPER") != "1" {
		return
	}
	expected, err := filepath.EvalSymlinks(os.Args[len(os.Args)-1])
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil || cwd != expected {
		t.Fatalf("project cwd: %q, want %q: %v", cwd, expected, err)
	}
	info, err := os.Stat("README.md")
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("project README missing or not regular: %v", err)
	}
}
