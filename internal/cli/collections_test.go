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

func TestCollectionCLIListPreviewApplyAndErrors(t *testing.T) {
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
	list := invoke(0, "collections")
	if _, selected := list["selected"]; selected || len(list) != 4 {
		t.Fatalf("listing shape: %v", list)
	}
	var available []string
	if err := json.Unmarshal(list["available"], &available); err != nil || !reflect.DeepEqual(available, catalogue.CollectionNames()) {
		t.Fatal("wrong available names")
	}
	invoke(2, "collections", "--apply")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"init", root, "--collection", "research", "--collection", "archive", "--collection", "research"}, &stdout, &stderr); code != 0 {
		t.Fatalf("selected init: %d %s", code, &stderr)
	}
	loaded, err := scaffolding.LoadState(root)
	// The default preset combines with explicit collections.
	if err != nil || !reflect.DeepEqual(loaded.State.Collections, []string{"apps", "archive", "infra", "packages", "research", "tools"}) {
		t.Fatalf("selected init state: %+v %v", loaded, err)
	}
	before := workspaceBytes(t, root)
	preview := invoke(0, "collections", "--add", "journal", "--add", "journal")
	if _, available := preview["available"]; available || len(preview) != 4 || string(preview["applied"]) != "false" {
		t.Fatalf("addition shape: %v", preview)
	}
	for _, args := range [][]string{
		{"collections", "--add", "unknown", "--apply"}, {"collections", "--add", "archive,journal"},
		{"collections", "--invalid"}, {"collections", "extra"},
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
	applied := invoke(0, "collections", "--add", "journal", "--apply")
	if string(applied["applied"]) != "true" {
		t.Fatal("apply not reported")
	}
	loaded, err = scaffolding.LoadState(root)
	if err != nil || !reflect.DeepEqual(loaded.State.Collections, []string{"apps", "archive", "infra", "journal", "packages", "research", "tools"}) {
		t.Fatalf("addition lost selections: %+v %v", loaded, err)
	}
	if err := os.MkdirAll(filepath.Join(root, "releases"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "releases/README.md"), []byte("owner"), 0600); err != nil {
		t.Fatal(err)
	}
	before = workspaceBytes(t, root)
	invoke(1, "collections", "--add", "releases", "--add", "research")
	invoke(2, "collections", "--add", "releases", "--add", "research", "--apply")
	if !reflect.DeepEqual(before, workspaceBytes(t, root)) {
		t.Fatal("conflicting addition wrote bytes")
	}
}
