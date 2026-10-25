package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/brendenchu/wsup/internal/scaffolding"
)

func TestMigrationCLIReportsAndPersistentConfigurationAreReadOnly(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace")
	if _, err := scaffolding.Initialise(root, Version, "workspace"); err != nil {
		t.Fatal(err)
	}
	loaded, err := scaffolding.LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	loaded.State.Version = 1
	body, err := json.Marshal(loaded.State)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, scaffolding.StatePath), body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".work/quality"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".work/quality/old.json"), []byte("Historical provenance\x00\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "wsup.json"), []byte(`{"version":1,"settings":{"timeout_seconds":30}}`), 0600); err != nil {
		t.Fatal(err)
	}
	before := workspaceBytes(t, root)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--root", root, "update", "--migrate-native"}, &stdout, &stderr)
	var plan scaffolding.MigrationPlan
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil || code != 0 || stderr.Len() != 0 || plan.Applied {
		t.Fatalf("preview: %d %s %s %v", code, &stdout, &stderr, err)
	}
	found := false
	for _, action := range plan.Actions {
		if action.Action == "move-report" && action.Path == ".wsup/reports/old.json" && action.From == ".work/quality/old.json" {
			found = true
		}
	}
	if !found || !reflect.DeepEqual(before, workspaceBytes(t, root)) {
		t.Fatal("report plan missing or preview wrote bytes")
	}
}
