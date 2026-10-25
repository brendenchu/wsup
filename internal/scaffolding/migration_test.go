package scaffolding

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
)

func pythonFixture(t *testing.T) string {
	t.Helper()
	files, err := catalogue.Bundle("workspace")
	if err != nil {
		t.Fatal(err)
	}
	root, state := nativeFixture(t, files)
	state.Version = 1
	for name, body := range map[string]string{
		".wsup/wsup.py": "synthetic Python runtime", ".wsup/wsup_core/cli.py": "synthetic module",
		".wsup/templates/workspace/README.md": "synthetic catalogue",
	} {
		writeFixture(t, root, name, []byte(body))
		state.Files[name] = contentDigest([]byte(body))
	}
	body, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, StatePath, body)
	return root
}

func TestMigrationPreviewIsExplicitReadOnlyAndRetirementIsManifestBound(t *testing.T) {
	root := pythonFixture(t)
	writeFixture(t, root, "README.md", []byte("Owner map"))
	writeFixture(t, root, "workspace.json", []byte("Owner registry"))
	writeFixture(t, root, ".wsup/reports/history.json", []byte("Unchanged evidence"))
	writeFixture(t, root, "tools/independent/source", []byte("Owner project"))
	before := snapshot(t, root)
	preview, err := PlanMigration(root, "0.2.0", nil)
	if err != nil || !reflect.DeepEqual(preview.Conflicts, []string{"README.md"}) {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	preview, err = PlanMigration(root, "0.2.0", []string{"README.md"})
	if err != nil || len(preview.Conflicts) != 0 || preview.FromSchema != 1 || preview.ToSchema != 2 || preview.Applied {
		t.Fatalf("keep preview: %+v %v", preview, err)
	}
	retired := []string{}
	for _, action := range preview.Actions {
		if action.Action == "retire-runtime" {
			retired = append(retired, action.Path)
		}
	}
	if !reflect.DeepEqual(retired, []string{".wsup/templates/workspace/README.md", ".wsup/wsup.py", ".wsup/wsup_core/cli.py"}) {
		t.Fatalf("retirement exceeded manifest: %v", retired)
	}
	again, err := PlanMigration(root, "0.2.0", []string{"README.md"})
	if err != nil || !reflect.DeepEqual(preview, again) || !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("migration preview changed bytes or was unstable")
	}
}

func TestMigrationRuntimeEditsMissingFilesUnknownFilesAndKeepCannotBypass(t *testing.T) {
	for _, name := range []string{".wsup/wsup.py", ".wsup/wsup_core/cli.py", ".wsup/templates/workspace/README.md"} {
		for _, missing := range []bool{false, true} {
			root := pythonFixture(t)
			if missing {
				if err := os.Remove(filepath.Join(root, filepath.FromSlash(name))); err != nil {
					t.Fatal(err)
				}
			} else {
				writeFixture(t, root, name, []byte("customized installation"))
			}
			before := snapshot(t, root)
			result, err := PlanMigration(root, "0.2.0", nil)
			if err != nil || !slices.Contains(result.Conflicts, name) {
				t.Fatalf("unsafe retirement: %+v %v", result, err)
			}
			if _, err := PlanMigration(root, "0.2.0", []string{name}); err == nil {
				t.Fatal("keep bypassed retirement")
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("unsafe preview changed bytes")
			}
		}
	}
	root := pythonFixture(t)
	writeFixture(t, root, ".wsup/templates/custom.txt", []byte("unknown owner content"))
	writeFixture(t, root, ".wsup/wsup_core/custom.py", []byte("unknown module"))
	before := snapshot(t, root)
	result, err := PlanMigration(root, "0.2.0", nil)
	if err != nil || !reflect.DeepEqual(result.Conflicts, []string{".wsup/templates/custom.txt", ".wsup/wsup_core/custom.py"}) ||
		!reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatalf("unknown files not protected: %+v %v", result, err)
	}
}

func TestMigrationRelocationChecksAndLegacyReportPolicyBlockers(t *testing.T) {
	root := pythonFixture(t)
	loaded, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "guidance/rules/changes.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "guidance/rules/changes.md")); err != nil {
		t.Fatal(err)
	}
	delete(loaded.State.Files, "guidance/rules/changes.md")
	writeFixture(t, root, ".ai/rules/changes.md", body)
	loaded.State.Files[".ai/rules/changes.md"] = contentDigest(body)
	state, err := json.Marshal(loaded.State)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, StatePath, state)
	preview, err := PlanMigration(root, "0.2.0", nil)
	if err != nil || len(preview.Conflicts) != 0 || !slices.ContainsFunc(preview.Actions, func(a MigrationAction) bool {
		return a.Action == "relocate" && a.Path == "guidance/rules/changes.md" && a.From == ".ai/rules/changes.md"
	}) {
		t.Fatalf("relocation missing: %+v %v", preview, err)
	}
	if _, err := PlanMigration(root, "0.2.0", []string{".ai/rules/changes.md"}); err == nil {
		t.Fatal("keep bypassed relocation")
	}
	writeFixture(t, root, "guidance/rules/changes.md", []byte("collision"))
	writeFixture(t, root, ".work/quality/old.json", []byte("legacy report bytes"))
	writeFixture(t, root, "wsup.local.json", []byte("legacy policy"))
	before := snapshot(t, root)
	preview, err = PlanMigration(root, "0.2.0", nil)
	if err != nil || !reflect.DeepEqual(preview.Conflicts, []string{".work/quality", "guidance/rules/changes.md", "wsup.local.json"}) ||
		!reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatalf("policy/collision not blocked: %+v %v", preview, err)
	}
}

func TestMigrationRefusesNativeStateAndSymlinkedInstallations(t *testing.T) {
	root := collectionWorkspace(t)
	if _, err := PlanMigration(root, "0.2.0", nil); err == nil || !strings.Contains(err.Error(), "schema-1") {
		t.Fatalf("native state migrated: %v", err)
	}
	root = pythonFixture(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".wsup/templates/unknown")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	before, external := snapshot(t, root), snapshot(t, outside)
	if _, err := PlanMigration(root, "0.2.0", nil); err == nil {
		t.Fatal("symlink in retiring tree accepted")
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) || !reflect.DeepEqual(external, snapshot(t, outside)) {
		t.Fatal("symlink preview changed files")
	}
}
