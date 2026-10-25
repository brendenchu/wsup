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
			}
		})
	}
}
