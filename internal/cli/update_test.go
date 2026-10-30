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
		if entry.Type()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(name)
			files[name] = "link:" + target
			return err
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
			loaded, err := scaffolding.LoadLock(root)
			if err != nil {
				t.Fatal(err)
			}
			old := []byte("Synthetic old managed README")
			if err := os.WriteFile(filepath.Join(root, "README.md"), old, 0600); err != nil {
				t.Fatal(err)
			}
			loaded.Lock.Files["README.md"] = fmt.Sprintf("sha256:%x", sha256.Sum256(old))
			loaded.Lock.WsupVersion = "0.0.0"
			state, err := scaffolding.EncodeLock(loaded.Lock)
			if err != nil {
				t.Fatal(err)
			}
			statePath := filepath.Join(root, filepath.FromSlash(scaffolding.LockPath))
			if err := os.WriteFile(statePath, state, 0600); err != nil {
				t.Fatal(err)
			}
			for name, body := range map[string]string{
				"AGENTS.md": "Owner guidance customization", "wsup.local.json": "Owner local bytes",
				"wsup.json":                              "Owner registry bytes, deliberately not parsed by update",
				"workbench/.local/wsup/reports/old.json": "Old evidence", "tools/independent/source": "Independent project",
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
			if !reflect.DeepEqual(conflict.Conflicts, []string{"AGENTS.md"}) || conflict.Applied {
				t.Fatalf("conflict preview: %+v", conflict)
			}
			invoke(2, "--apply")
			invoke(2, "--keep", "unknown")
			invoke(2, "--keep", "../escape")
			invoke(2, "--invalid")
			if !reflect.DeepEqual(before, workspaceBytes(t, root)) {
				t.Fatal("previews/errors wrote workspace")
			}
			preview := invoke(0, "--keep", "AGENTS.md")
			if preview.Applied || len(preview.Actions) != 2 || preview.FromVersion != "0.0.0" || preview.ToVersion != Version {
				t.Fatalf("keep preview: %+v", preview)
			}
			applied := invoke(0, "--keep", "AGENTS.md", "--apply")
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
			settled := invoke(0, "--keep", "AGENTS.md")
			if !reflect.DeepEqual(settled.Actions, []scaffolding.Action{{Path: "AGENTS.md", Action: "keep-customization"}}) {
				t.Fatalf("update did not settle: %+v", settled)
			}
			if !reflect.DeepEqual(invoke(1).Conflicts, []string{"AGENTS.md"}) {
				t.Fatal("keep silently adopted customized baseline")
			}
		})
	}
}

func TestNativeUpdateCLIRejectsUnsupportedStateWithoutWrites(t *testing.T) {
	for file, tc := range map[string]struct{ body, error string }{
		"wsup.lock": {`{"version":2,"wsup_version":"0.1.0","starter":"workspace","modules":{"workspace-base":"1.0.0"},"files":{}}`,
			"unsupported wsup.lock version 2"},
		".wsup/template-state.json": {`{"version":2,"template_version":"0.2.0","template":"workspace","files":{}}`,
			"created by wsup 0.2.0"},
	} {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, file)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, file), []byte(tc.body), 0600); err != nil {
			t.Fatal(err)
		}
		before := workspaceBytes(t, root)
		for _, args := range [][]string{{}, {"--apply"}} {
			var stdout, stderr bytes.Buffer
			code := Run(append([]string{"--root", root, "update"}, args...), &stdout, &stderr)
			if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.error) {
				t.Fatalf("unsupported %s accepted: %d %s %s", file, code, &stdout, &stderr)
			}
		}
		if !reflect.DeepEqual(before, workspaceBytes(t, root)) {
			t.Fatal("unsupported update changed files")
		}
	}
}
