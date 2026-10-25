package scaffolding

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMigrationPreviewsRealPinnedHistoricalStarters(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "wsup")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/wsup")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build historical migration binary: %s %v", output, err)
	}
	for _, historical := range []struct{ name, commit, entry string }{
		{"v0.1.0", "b30b8675901b9146afe82165dfb612483334fb04", "tools/wsup.py"},
		{"python-baseline", "3b709d522ddd1aab38c4442b20cfb22eb1787915", "src/wsup.py"},
	} {
		t.Run(historical.name, func(t *testing.T) {
			archive := exec.Command("git", "archive", historical.commit)
			archive.Dir = "../.."
			body, err := archive.Output()
			if err != nil {
				t.Fatalf("required historical commit unavailable: %v", err)
			}
			source := t.TempDir()
			reader := tar.NewReader(bytes.NewReader(body))
			for {
				header, err := reader.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if header.Typeflag == tar.TypeDir || header.Typeflag == tar.TypeXGlobalHeader {
					continue
				}
				if header.Typeflag != tar.TypeReg || !portableFile(header.Name) {
					t.Fatalf("unexpected archive entry: %s", header.Name)
				}
				content, err := io.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
				writeFixture(t, source, header.Name, content)
			}
			for _, kind := range []string{"workspace", "project"} {
				destination := filepath.Join(t.TempDir(), kind)
				command := exec.Command("python3", "-B", filepath.Join(source, historical.entry), "init", destination, "--template", kind)
				command.Dir = source
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("historical generation requires Python: %s %v", output, err)
				}
				if kind == "workspace" {
					// Verify immutable generated inventory hashes, sizes and directory
					// membership against the independently frozen capture.
					inventoryBody, err := os.ReadFile("../../testdata/python-baseline/generated-workspaces.json")
					if err != nil {
						t.Fatal(err)
					}
					type recorded struct {
						Directory bool   `json:"directory"`
						SHA256    string `json:"sha256"`
						Bytes     int    `json:"bytes"`
					}
					var inventories map[string]struct {
						Commit string              `json:"commit"`
						Files  map[string]recorded `json:"files"`
					}
					if err := json.Unmarshal(inventoryBody, &inventories); err != nil {
						t.Fatal(err)
					}
					expected := inventories[historical.name]
					if expected.Commit != historical.commit {
						t.Fatal("historical commit mismatch")
					}
					actual := snapshot(t, destination)
					delete(actual, ".")
					if len(actual) != len(expected.Files) {
						t.Fatalf("historical inventory count: %d want %d", len(actual), len(expected.Files))
					}
					for name, record := range expected.Files {
						key := filepath.FromSlash(strings.TrimSuffix(name, "/"))
						value, exists := actual[key]
						if !exists {
							t.Fatalf("missing historical asset: %s", name)
						}
						if record.Directory {
							if value != "directory" {
								t.Fatalf("historical directory differs: %s", name)
							}
						} else {
							content := []byte(strings.TrimPrefix(value, "file:"))
							if len(content) != record.Bytes || contentDigest(content) != record.SHA256 {
								t.Fatalf("historical bytes differ: %s", name)
							}
						}
					}
				}
				writeFixture(t, destination, "README.md", []byte("Owner customized map"))
				writeFixture(t, destination, "tools/independent/README.md", []byte("Owner project"))
				writeFixture(t, destination, ".wsup/reports/history.json", []byte("Historical evidence"))
				writeFixture(t, destination, ".work/quality/legacy.json", []byte("Historical legacy-location evidence\x00\r\n"))
				writeFixture(t, destination, "wsup.json", []byte(`{"version":1,"settings":{"timeout_seconds":30}}`))
				writeFixture(t, destination, "wsup.local.json", []byte(`{"version":1,"settings":{"report_dir":".wsup/reports/custom"}}`))
				before := snapshot(t, destination)
				result, err := PlanMigration(destination, "0.1.0", []string{"README.md"})
				if err != nil || len(result.Conflicts) != 0 {
					t.Fatalf("real %s/%s preview blocked: %+v %v", historical.name, kind, result, err)
				}
				if !reflect.DeepEqual(before, snapshot(t, destination)) {
					t.Fatal("historical preview wrote files")
				}
				moved := false
				for _, action := range result.Actions {
					if action.Action == "move-report" && action.Path == ".wsup/reports/legacy.json" && action.From == ".work/quality/legacy.json" {
						moved = true
					}
				}
				if !moved {
					t.Fatal("historical report relocation missing")
				}
				apply := exec.Command(binary, "--root", destination, "update", "--migrate-native", "--keep", "README.md", "--apply")
				apply.Dir, apply.Env = destination, []string{"PATH="}
				output, err := apply.CombinedOutput()
				var applied MigrationPlan
				if err != nil {
					t.Fatalf("historical CLI apply %s/%s: %s %v", historical.name, kind, output, err)
				}
				if err := json.Unmarshal(output, &applied); err != nil || !applied.Applied {
					t.Fatalf("historical apply %s/%s: %+v %v", historical.name, kind, applied, err)
				}
				for _, name := range []string{
					"README.md", "tools/independent/README.md", ".wsup/reports/history.json",
					"wsup.json", "wsup.local.json", "workspace.json",
				} {
					old, existed := before[filepath.FromSlash(name)]
					if !existed {
						continue
					}
					current, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(name)))
					if err != nil || "file:"+string(current) != old {
						t.Fatalf("historical owner bytes changed: %s %v", name, err)
					}
				}
				report, err := os.ReadFile(filepath.Join(destination, ".wsup/reports/legacy.json"))
				if err != nil || !bytes.Equal(report, []byte("Historical legacy-location evidence\x00\r\n")) {
					t.Fatal("historical report provenance changed")
				}
				for _, name := range []string{".wsup/wsup.py", ".wsup/wsup_core", ".wsup/templates", "tools/wsup.py", "tools/wsup_core", "tools/wsup_templates", "templates/project", ".work/quality"} {
					if _, err := os.Lstat(filepath.Join(destination, filepath.FromSlash(name))); !os.IsNotExist(err) {
						t.Fatalf("historical runtime/report source remains: %s", name)
					}
				}
				loaded, err := LoadState(destination)
				if err != nil || loaded.State.Version != StateVersion {
					t.Fatalf("historical state not native: %+v %v", loaded.State, err)
				}
				settled, err := Update(destination, "0.1.0", []string{"README.md"}, false)
				if err != nil || len(settled.Conflicts) != 0 || !reflect.DeepEqual(settled.Actions, []Action{{"README.md", "keep-customization"}}) {
					t.Fatalf("historical migration not settled: %+v %v", settled, err)
				}
				relocated := destination + "-relocated"
				if err := os.Rename(destination, relocated); err != nil {
					t.Fatal(err)
				}
				run := func(root string, args ...string) {
					t.Helper()
					command := exec.Command(binary, append([]string{"--root", root}, args...)...)
					command.Dir, command.Env = relocated, []string{"PATH="}
					if output, err := command.CombinedOutput(); err != nil {
						t.Fatalf("migrated standalone %v: %s %v", args, output, err)
					}
				}
				run(relocated, "guidance")
				if kind == "workspace" {
					run(relocated, "validate")
				}
				run(relocated, "update", "--keep", "README.md")
				child := relocated + "-child"
				run(relocated, "init", child, "--template", kind)
				run(child, "guidance")
				if kind == "workspace" {
					run(child, "validate")
				}
				childState, err := LoadState(child)
				if err != nil || childState.State.Version != StateVersion || len(childState.State.Collections) != 0 {
					t.Fatalf("migrated child state: %+v %v", childState.State, err)
				}
			}
		})
	}
}
