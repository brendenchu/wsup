package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/brendenchu/wsup/internal/scaffolding"
)

func workspaceBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			files[name] = "directory"
			return nil
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		files[name] = string(body)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestNativeUpdateCLIReadsEmbeddedCatalogueAndPreservesOwnerFiles(t *testing.T) {
	for _, kind := range []string{"workspace", "project"} {
		t.Run(kind, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), kind)
			if _, err := scaffolding.Initialise(root, Version, kind); err != nil {
				t.Fatal(err)
			}
			loaded, err := scaffolding.LoadState(root)
			if err != nil {
				t.Fatal(err)
			}
			old := []byte("Synthetic old managed README")
			if err := os.WriteFile(filepath.Join(root, "README.md"), old, 0600); err != nil {
				t.Fatal(err)
			}
			loaded.State.Files["README.md"] = fmt.Sprintf("%x", sha256.Sum256(old))
			loaded.State.TemplateVersion = "0.0.0"
			state, err := scaffolding.EncodeNativeState(loaded.State)
			if err != nil {
				t.Fatal(err)
			}
			statePath := filepath.Join(root, filepath.FromSlash(scaffolding.StatePath))
			if err := os.WriteFile(statePath, state, 0600); err != nil {
				t.Fatal(err)
			}
			for name, body := range map[string]string{
				"AI.md": "Owner guidance customization", "wsup.local.json": "Owner local bytes",
				"workspace.json":         "Owner registry bytes, deliberately not parsed by update",
				".wsup/reports/old.json": "Old evidence", "tools/independent/source": "Independent project",
			} {
				filename := filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filename, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before := workspaceBytes(t, root)
			invoke := func(code int, args ...string) scaffolding.UpdatePlan {
				t.Helper()
				var stdout, stderr bytes.Buffer
				actual := Run(append([]string{"--root", root, "update"}, args...), &stdout, &stderr)
				if actual != code {
					t.Fatalf("update %v: exit %d stdout=%s stderr=%s", args, actual, &stdout, &stderr)
				}
				if code == 2 {
					if stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "Error: ") {
						t.Fatalf("invalid update error shape: %q %q", &stdout, &stderr)
					}
					return scaffolding.UpdatePlan{}
				}
				var result scaffolding.UpdatePlan
				if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || stderr.Len() != 0 {
					t.Fatalf("invalid update output: %s %s %v", &stdout, &stderr, err)
				}
				return result
			}
			conflict := invoke(1)
			if !reflect.DeepEqual(conflict.Conflicts, []string{"AI.md"}) || conflict.Applied {
				t.Fatalf("conflict preview: %+v", conflict)
			}
			invoke(2, "--apply")
			invoke(2, "--keep", "unknown")
			invoke(2, "--keep", "../escape")
			invoke(2, "--invalid")
			if !reflect.DeepEqual(before, workspaceBytes(t, root)) {
				t.Fatal("previews/errors wrote workspace")
			}
			preview := invoke(0, "--keep", "AI.md")
			if preview.Applied || len(preview.Actions) != 2 || preview.FromVersion != "0.0.0" || preview.ToVersion != Version {
				t.Fatalf("keep preview: %+v", preview)
			}
			applied := invoke(0, "--keep", "AI.md", "--apply")
			if !applied.Applied {
				t.Fatal("successful apply not reported")
			}
			after := workspaceBytes(t, root)
			for name, body := range before {
				if name != statePath && name != filepath.Join(root, "README.md") && after[name] != body {
					t.Fatalf("owner content changed: %s", name)
				}
			}
			bundle, err := catalogue.Bundle(kind)
			if err != nil || after[filepath.Join(root, "README.md")] != string(bundle["README.md"]) {
				t.Fatal("update did not use embedded native content")
			}
			settled := invoke(0, "--keep", "AI.md")
			if !reflect.DeepEqual(settled.Actions, []scaffolding.Action{{Path: "AI.md", Action: "keep-customization"}}) {
				t.Fatalf("update did not settle: %+v", settled)
			}
			if !reflect.DeepEqual(invoke(1).Conflicts, []string{"AI.md"}) {
				t.Fatal("keep silently adopted customized baseline")
			}
		})
	}
}

func TestNativeUpdateCLIRejectsPythonStateWithoutMigration(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".wsup"), 0755); err != nil {
		t.Fatal(err)
	}
	body := `{"version":1,"template_version":"0.1.0","template":"workspace","files":{}}`
	if err := os.WriteFile(filepath.Join(root, ".wsup/template-state.json"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	before := workspaceBytes(t, root)
	for _, args := range [][]string{{}, {"--apply"}} {
		var stdout, stderr bytes.Buffer
		code := Run(append([]string{"--root", root, "update"}, args...), &stdout, &stderr)
		if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "explicit native migration") {
			t.Fatalf("legacy state accepted: %d %s %s", code, &stdout, &stderr)
		}
	}
	if !reflect.DeepEqual(before, workspaceBytes(t, root)) {
		t.Fatal("legacy update changed files")
	}
}
