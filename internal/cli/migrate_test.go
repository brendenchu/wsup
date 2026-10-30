package cli

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateCLIPreviewsThenConvertsAWorkspace(t *testing.T) {
	source := filepath.Join("..", "scaffolding", "testdata", "v0.2.0", "workspace")
	root := filepath.Join(t.TempDir(), "workspace")
	err := filepath.WalkDir(source, func(file string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, _ := filepath.Rel(source, file)
		body, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, relative)), 0755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, relative), body, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	invoke := func(args ...string) (int, string, string) {
		var stdout, stderr bytes.Buffer
		code := Run(append([]string{"--root", root}, args...), &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	if code, _, stderr := invoke("update"); code != 2 || !strings.Contains(stderr, "wsup migrate converts it") {
		t.Fatalf("update before migration: %d %s", code, stderr)
	}
	if code, stdout, _ := invoke("migrate"); code != 0 || !strings.Contains(stdout, `"applied": false`) {
		t.Fatalf("preview: %d %s", code, stdout)
	}
	if _, err := os.Stat(filepath.Join(root, "wsup.lock")); !os.IsNotExist(err) {
		t.Fatal("preview wrote wsup.lock")
	}
	if err := os.WriteFile(filepath.Join(root, "apps/README.md"), []byte("Owner\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if code, stdout, _ := invoke("migrate"); code != 1 || !strings.Contains(stdout, `"apps/README.md"`) {
		t.Fatalf("owner file conflict: %d %s", code, stdout)
	}
	if code, _, stderr := invoke("migrate", "--keep", "apps/README.mdx"); code != 2 || !strings.Contains(stderr, "--keep") {
		t.Fatalf("mistyped keep: %d %s", code, stderr)
	}
	if code, _, _ := invoke("migrate", "--agent", "unknown"); code != 2 {
		t.Fatal("unknown agent accepted")
	}
	if code, stdout, stderr := invoke("migrate", "--apply", "--agent", "claude", "--keep", "apps/README.md"); code != 0 || !strings.Contains(stdout, `"backup": "workbench/.local/wsup/migrate/`) {
		t.Fatalf("apply: %d %s %s", code, stdout, stderr)
	}
	for _, args := range [][]string{{"validate"}, {"update", "--keep", "apps/README.md"}} {
		if code, stdout, stderr := invoke(args...); code != 0 {
			t.Fatalf("%v after migration: %d %s %s", args, code, stdout, stderr)
		}
	}
	if _, err := os.Lstat(filepath.Join(root, "GEMINI.md")); !os.IsNotExist(err) {
		t.Fatal("unselected agent installed")
	}
}
