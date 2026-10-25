package scaffolding

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/guidance"
)

func migrationFixture(t *testing.T) (string, preparedMigration) {
	t.Helper()
	root := pythonFixture(t)
	writeFixture(t, root, ".work/quality/old.json", []byte("Historical report\x00\r\n"))
	writeFixture(t, root, "README.md", []byte("Owner map"))
	writeFixture(t, root, "tools/independent/source", []byte("Owner source"))
	writeFixture(t, root, "workspace.json", []byte("Owner registry"))
	writeFixture(t, root, "wsup.local.json", []byte(`{"version":1,"settings":{"report_dir":".wsup/reports"}}`))
	var prepared preparedMigration
	result, err := planMigration(root, "0.2.0", []string{"README.md"}, &prepared)
	if err != nil || len(result.Conflicts) != 0 {
		t.Fatalf("prepare: %+v %v", result, err)
	}
	return root, prepared
}

func removeMigrationFile(root *os.Root, name string) error { return root.Remove(name) }

func TestMigrationApplyCommitsNativeContentAndStateBeforeRetirement(t *testing.T) {
	root, prepared := migrationFixture(t)
	before := snapshot(t, root)
	removed := 0
	result, err := prepared.applyWith(writeManaged, func(handle *os.Root, name string) error {
		loaded, err := LoadState(root)
		if err != nil || loaded.State.Version != StateVersion {
			t.Fatal("retirement preceded native state")
		}
		for filename, body := range prepared.content.files {
			current, err := handle.ReadFile(filename)
			if err != nil || !bytes.Equal(current, body) {
				t.Fatalf("retirement preceded required output: %s", filename)
			}
		}
		removed++
		return handle.Remove(name)
	})
	if err != nil || !result.Applied || removed != 4 {
		t.Fatalf("apply: %+v removed=%d %v", result, removed, err)
	}
	for _, name := range []string{"README.md", "workspace.json", "tools/independent/source", "wsup.local.json"} {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || "file:"+string(body) != before[filepath.FromSlash(name)] {
			t.Fatalf("owner bytes changed: %s", name)
		}
	}
	for _, name := range []string{".wsup/wsup.py", ".wsup/wsup_core", ".wsup/templates", ".work/quality"} {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(name))); !os.IsNotExist(err) {
			t.Fatalf("legacy source retained: %s", name)
		}
	}
	report, err := os.ReadFile(filepath.Join(root, ".wsup/reports/old.json"))
	if err != nil || !bytes.Equal(report, []byte("Historical report\x00\r\n")) {
		t.Fatal("report provenance changed")
	}
	loaded, err := LoadState(root)
	if err != nil || loaded.State.Files["README.md"] == contentDigest([]byte("Owner map")) {
		t.Fatal("kept hash adopted")
	}
	for name := range loaded.State.Files {
		if strings.HasPrefix(name, ".wsup/") {
			t.Fatalf("native state owns runtime/report: %s", name)
		}
	}
	plan, err := Update(root, "0.2.0", []string{"README.md"}, false)
	if err != nil || len(plan.Conflicts) != 0 || !reflect.DeepEqual(plan.Actions, []Action{{"README.md", "keep-customization"}}) {
		t.Fatalf("migration did not settle: %+v %v", plan, err)
	}
}

func TestMigrationStaleInputsBlockBeforeAnyWrite(t *testing.T) {
	for _, name := range []string{
		StatePath, ".wsup/wsup.py", ".wsup/wsup_core/cli.py",
		".wsup/templates/new-owner.txt", ".work/quality/new-report.json",
		".wsup/reports/old.json", "wsup.local.json", "README.md",
	} {
		t.Run(name, func(t *testing.T) {
			root, prepared := migrationFixture(t)
			writeFixture(t, root, name, []byte("Concurrent owner changes"))
			before := snapshot(t, root)
			result, err := prepared.applyWith(writeManaged, removeMigrationFile)
			if err == nil || result.Applied || !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatalf("stale migration wrote content: %+v %v", result, err)
			}
		})
	}
}

func TestMigrationWriteFailuresNeverRetireSourcesOrCommitState(t *testing.T) {
	for _, failing := range []string{".wsup/reports/old.json", StatePath} {
		root, prepared := migrationFixture(t)
		injected := errors.New("synthetic write denied")
		removed := false
		result, err := prepared.applyWith(func(handle *os.Root, name string, body []byte, exclusive bool) error {
			if name == failing {
				return injected
			}
			return writeManaged(handle, name, body, exclusive)
		}, func(*os.Root, string) error { removed = true; return nil })
		if !errors.Is(err, injected) || result.Applied || removed || !strings.Contains(err.Error(), "state was not committed") {
			t.Fatalf("write failure unsafe: %+v %v", result, err)
		}
		state, err := LoadState(root)
		if err != nil || !bytes.Equal(state.Original, prepared.content.stateBefore) {
			t.Fatal("failed writes committed native state")
		}
		for _, name := range []string{".wsup/wsup.py", ".wsup/wsup_core/cli.py", ".wsup/templates/workspace/README.md", ".work/quality/old.json"} {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
				t.Fatalf("legacy source lost: %s", name)
			}
		}
	}
}

func TestMigrationRetirementFailureDisclosesCommittedStateAndPreservesRemainingSources(t *testing.T) {
	root, prepared := migrationFixture(t)
	injected := errors.New("synthetic retirement denied")
	result, err := prepared.applyWith(writeManaged, func(*os.Root, string) error { return injected })
	if !errors.Is(err, injected) || result.Applied || !strings.Contains(err.Error(), "state committed but legacy retirement incomplete") {
		t.Fatalf("retirement failure hidden: %+v %v", result, err)
	}
	state, err := LoadState(root)
	if err != nil || state.State.Version != StateVersion {
		t.Fatal("native state not committed before retirement attempt")
	}
	for _, name := range []string{".wsup/wsup.py", ".wsup/wsup_core/cli.py", ".wsup/templates/workspace/README.md", ".work/quality/old.json", ".wsup/reports/old.json"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Fatalf("recovery source/output missing: %s", name)
		}
	}
}

func TestMigrationMutationsDuringWritesPreventStateCommitAndRetirement(t *testing.T) {
	for _, changed := range []string{".wsup/wsup.py", ".wsup/templates/new.txt", ".work/quality/new.json", "wsup.local.json"} {
		root, prepared := migrationFixture(t)
		removed := false
		once := false
		result, err := prepared.applyWith(func(handle *os.Root, name string, body []byte, exclusive bool) error {
			if err := writeManaged(handle, name, body, exclusive); err != nil {
				return err
			}
			if !once {
				once = true
				writeFixture(t, root, changed, []byte("Owner changed during writes"))
			}
			return nil
		}, func(*os.Root, string) error { removed = true; return nil })
		if err == nil || result.Applied || removed {
			t.Fatalf("changed inputs retired sources: %s %+v %v", changed, result, err)
		}
		state, err := LoadState(root)
		if err != nil || !bytes.Equal(state.Original, prepared.content.stateBefore) {
			t.Fatal("changed inputs committed state")
		}
	}
}

func TestMigrationLegacyGuidanceRelocationAndEmptyAncestorCleanup(t *testing.T) {
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
	result, err := ApplyMigration(root, "0.2.0", nil)
	if err != nil || !result.Applied {
		t.Fatalf("relocation: %+v %v", result, err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".ai")); !os.IsNotExist(err) {
		t.Fatal("empty legacy ancestors not removed")
	}
	guidanceResult, err := guidance.Inspect(root)
	if err != nil || len(guidanceResult.Errors) != 0 {
		t.Fatalf("relocated guidance broken: %+v %v", guidanceResult, err)
	}
}
