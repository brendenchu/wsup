package scaffolding

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestMigrationReportsPreviewRetainsExactBytesAndNestedDestinations(t *testing.T) {
	root := pythonFixture(t)
	history := map[string][]byte{
		".work/quality/old.json":           []byte("{\"schema\":1,\"runtime\":\"python\",\"timestamp\":\"2020-01-01\"}\x00\n"),
		".work/quality/nested/evidence.md": []byte("Historical evidence\r\n\xff"),
		".wsup/reports/existing.json":      []byte("Existing native-location report"),
		"wsup.json":                        []byte(`{"version":1,"settings":{"timeout_seconds":42,"report_dir":".wsup/reports/custom"}}`),
		"wsup.local.json":                  []byte("{\"version\":1,\"settings\":{\"output_limit_bytes\":128}}\n"),
	}
	for name, body := range history {
		writeFixture(t, root, name, body)
	}
	before := snapshot(t, root)
	result, err := PlanMigration(root, "0.2.0", nil)
	if err != nil || len(result.Conflicts) != 0 {
		t.Fatalf("report preview: %+v %v", result, err)
	}
	var moves []MigrationAction
	for _, action := range result.Actions {
		if action.Action == "move-report" {
			moves = append(moves, action)
		}
	}
	want := []MigrationAction{
		{Path: ".wsup/reports/nested/evidence.md", Action: "move-report", From: ".work/quality/nested/evidence.md"},
		{Path: ".wsup/reports/old.json", Action: "move-report", From: ".work/quality/old.json"},
	}
	if !reflect.DeepEqual(moves, want) {
		t.Fatalf("report moves: %+v", moves)
	}
	again, err := PlanMigration(root, "0.2.0", nil)
	if err != nil || !reflect.DeepEqual(result, again) || !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("report preview changed bytes, provenance or output order")
	}
}

func TestMigrationReportsCollisionsNeverMergeEvenIdenticalBytes(t *testing.T) {
	for _, directory := range []bool{false, true} {
		root := pythonFixture(t)
		writeFixture(t, root, ".work/quality/history.json", []byte("identical"))
		if directory {
			if err := os.MkdirAll(filepath.Join(root, ".wsup/reports/history.json"), 0755); err != nil {
				t.Fatal(err)
			}
		} else {
			writeFixture(t, root, ".wsup/reports/history.json", []byte("identical"))
		}
		before := snapshot(t, root)
		result, err := PlanMigration(root, "0.2.0", nil)
		if err != nil || !reflect.DeepEqual(result.Conflicts, []string{".wsup/reports/history.json"}) ||
			slices.ContainsFunc(result.Actions, func(a MigrationAction) bool { return a.Action == "move-report" }) {
			t.Fatalf("report collision not blocked: %+v %v", result, err)
		}
		if !reflect.DeepEqual(before, snapshot(t, root)) {
			t.Fatal("report collision changed bytes")
		}
	}
}

func TestMigrationConfigLegacyPoliciesCannotBeHiddenByOverridesOrKeep(t *testing.T) {
	root := pythonFixture(t)
	writeFixture(t, root, "wsup.json", []byte(`{"version":1,"settings":{"report_dir":".work/quality/retained"}}`))
	writeFixture(t, root, "wsup.local.json", []byte(`{"version":1,"settings":{"report_dir":".wsup/reports"}}`))
	t.Setenv("WSUP_REPORT_DIR", ".wsup/reports")
	before := snapshot(t, root)
	result, err := PlanMigration(root, "0.2.0", nil)
	if err != nil || !reflect.DeepEqual(result.Conflicts, []string{"wsup.json"}) {
		t.Fatalf("legacy shared policy hidden: %+v %v", result, err)
	}
	if _, err := PlanMigration(root, "0.2.0", []string{"wsup.json"}); err == nil {
		t.Fatal("keep accepted unmanaged config")
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("legacy policy rewritten")
	}
}

func TestMigrationConfigValidationAndEnvironmentIndependence(t *testing.T) {
	for _, body := range []string{
		"{", "null", `{"version":2}`, `{"version":1,"typo":true}`,
		`{"version":1,"settings":null}`, `{"version":1,"settings":{"unknown":1}}`,
		`{"version":1,"settings":{"timeout_seconds":false}}`,
		`{"version":1,"settings":{"output_limit_bytes":0}}`,
		`{"version":1,"settings":{"report_dir":42}}`,
		`{"version":1,"settings":{"report_dir":"../outside"}}`,
		`{"version":1,"settings":{"report_dir":".wsup/templates"}}`,
	} {
		root := pythonFixture(t)
		writeFixture(t, root, "wsup.json", []byte(body))
		before := snapshot(t, root)
		if _, err := PlanMigration(root, "0.2.0", nil); err == nil {
			t.Fatalf("invalid config accepted: %s", body)
		}
		if !reflect.DeepEqual(before, snapshot(t, root)) {
			t.Fatal("invalid config preview wrote files")
		}
	}
	root := pythonFixture(t)
	t.Setenv("WSUP_TIMEOUT_SECONDS", "invalid")
	t.Setenv("WSUP_REPORT_DIR", "../outside")
	if result, err := PlanMigration(root, "0.2.0", nil); err != nil || len(result.Conflicts) != 0 {
		t.Fatalf("ambient configuration affected persistent preview: %+v %v", result, err)
	}
}

func TestMigrationReportSymlinksAndNonDirectoryParentsRefused(t *testing.T) {
	for _, link := range []string{".work", ".work/quality", ".work/quality/history.json", ".wsup/reports", ".wsup/reports/nested", "wsup.local.json"} {
		root := pythonFixture(t)
		outside := t.TempDir()
		writeFixture(t, outside, "history.json", []byte("external evidence"))
		writeFixture(t, root, ".work/quality/nested/history.json", []byte("legacy evidence"))
		// Use a clean root for source ancestry links to avoid deleting fixtures.
		if link == ".work" || link == ".work/quality" {
			root = pythonFixture(t)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(link))), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, filepath.FromSlash(link))); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		before, external := snapshot(t, root), snapshot(t, outside)
		if _, err := PlanMigration(root, "0.2.0", nil); err == nil {
			t.Fatalf("symlink accepted: %s", link)
		}
		if !reflect.DeepEqual(before, snapshot(t, root)) || !reflect.DeepEqual(external, snapshot(t, outside)) {
			t.Fatal("symlink preview modified owner bytes")
		}
	}
	for _, name := range []string{".work", ".work/quality", ".wsup/reports/nested"} {
		root := pythonFixture(t)
		if strings.HasPrefix(name, ".wsup") {
			writeFixture(t, root, ".work/quality/nested/history.json", []byte("source"))
		}
		writeFixture(t, root, name, []byte("not a directory"))
		before := snapshot(t, root)
		if _, err := PlanMigration(root, "0.2.0", nil); err == nil {
			t.Fatalf("non-directory accepted: %s", name)
		}
		if !reflect.DeepEqual(before, snapshot(t, root)) {
			t.Fatal("invalid parent changed bytes")
		}
	}
}
