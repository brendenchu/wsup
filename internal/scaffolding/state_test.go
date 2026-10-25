package scaffolding

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func stateDocument(version int) map[string]any {
	return map[string]any{
		"version": version, "template_version": "0.1.0", "template": "workspace",
		"files":              map[string]string{"README.md": strings.Repeat("a", 64)},
		"collections":        []string{"research", "archive", "research"},
		"omitted_home_files": []string{"apps/.gitkeep"},
	}
}

func encoded(t *testing.T, document map[string]any) []byte {
	t.Helper()
	body, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestNativeStatePreservesBaselineAndOriginalBytes(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".wsup"), 0755); err != nil {
		t.Fatal(err)
	}
	document := stateDocument(StateVersion)
	body := append(encoded(t, document), '\n')
	filename := filepath.Join(root, filepath.FromSlash(StatePath))
	if err := os.WriteFile(filename, body, 0600); err != nil {
		t.Fatal(err)
	}
	custom := []byte("Owner's customized bytes\x00\n")
	if err := os.WriteFile(filepath.Join(root, "README.md"), custom, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.State.Version != StateVersion || !bytes.Equal(loaded.Original, body) ||
		loaded.State.Files["README.md"] != strings.Repeat("a", 64) ||
		!reflect.DeepEqual(loaded.State.Collections, []string{"archive", "research"}) {
		t.Fatalf("state changed ownership: %+v", loaded)
	}
	current, err := os.ReadFile(filename)
	if err != nil || !bytes.Equal(current, body) {
		t.Fatal("state bytes changed")
	}
	current, err = os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil || !bytes.Equal(current, custom) {
		t.Fatal("custom content changed")
	}
}

func TestInvalidStateContracts(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(map[string]any)
	}{
		{"unknown schema", func(d map[string]any) { d["version"] = 3 }},
		{"unsupported schema", func(d map[string]any) { d["version"] = 1 }},
		{"boolean schema", func(d map[string]any) { d["version"] = true }},
		{"float schema", func(d map[string]any) { d["version"] = 1.5 }},
		{"unknown field", func(d map[string]any) { d["typo"] = true }},
		{"empty version", func(d map[string]any) { d["template_version"] = " " }},
		{"unknown template", func(d map[string]any) { d["template"] = "unknown" }},
		{"null files", func(d map[string]any) { d["files"] = nil }},
		{"null collections", func(d map[string]any) { d["collections"] = nil }},
		{"unknown collection", func(d map[string]any) { d["collections"] = []string{"unknown"} }},
		{"null omissions", func(d map[string]any) { d["omitted_home_files"] = nil }},
		{"bad omission", func(d map[string]any) { d["omitted_home_files"] = []string{"README.md"} }},
		{"project collections", func(d map[string]any) { d["template"] = "project" }},
		{"bad digest", func(d map[string]any) { d["files"] = map[string]string{"README.md": "not-a-hash"} }},
		{"parent collision", func(d map[string]any) {
			d["files"] = map[string]string{"docs": strings.Repeat("a", 64), "docs/readme": strings.Repeat("b", 64)}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			document := stateDocument(StateVersion)
			tc.edit(document)
			if _, err := ParseState(encoded(t, document)); err == nil {
				t.Fatal("invalid state accepted")
			}
		})
	}
	for _, name := range []string{"", ".", "../outside", "/absolute", "a/../b", "a//b", "a\\b", "a:b", "a\x00b", StatePath, ".wsup/reports/report.json", ".wsup/wsup.py"} {
		document := stateDocument(StateVersion)
		document["files"] = map[string]string{name: strings.Repeat("a", 64)}
		if _, err := ParseState(encoded(t, document)); err == nil {
			t.Fatalf("invalid owned path accepted: %q", name)
		}
	}
	for _, body := range []string{"null", "[]", "{} {}", "{"} {
		if _, err := ParseState([]byte(body)); err == nil {
			t.Fatalf("invalid JSON accepted: %s", body)
		}
	}
}

func TestStateSymlinksRefusedWithoutTouchingTargets(t *testing.T) {
	for _, link := range []string{".wsup", StatePath} {
		t.Run(link, func(t *testing.T) {
			root, outside := t.TempDir(), t.TempDir()
			body := encoded(t, stateDocument(StateVersion))
			target := filepath.Join(outside, "template-state.json")
			if err := os.WriteFile(target, body, 0600); err != nil {
				t.Fatal(err)
			}
			if link == ".wsup" {
				target = outside
			} else if err := os.Mkdir(filepath.Join(root, ".wsup"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(root, filepath.FromSlash(link))); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			if _, err := LoadState(root); err == nil {
				t.Fatal("symlinked state accepted")
			}
			current, err := os.ReadFile(filepath.Join(outside, "template-state.json"))
			if err != nil || !bytes.Equal(current, body) {
				t.Fatal("external state changed")
			}
		})
	}
}
