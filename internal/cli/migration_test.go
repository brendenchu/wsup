package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/scaffolding"
)

func TestMigrationCLIIsExplicitReadOnlyAndConflictingApplyBlocked(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace")
	if _, err := scaffolding.Initialise(root, Version, "workspace"); err != nil {
		t.Fatal(err)
	}
	state, err := scaffolding.LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	state.State.Version = 1
	body, err := json.Marshal(state.State)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, scaffolding.StatePath), body, 0600); err != nil {
		t.Fatal(err)
	}
	before := workspaceBytes(t, root)
	for _, tc := range []struct {
		args    []string
		code    int
		message string
	}{
		{[]string{"--migrate-native"}, 0, ""},
		{nil, 2, "explicit native migration"},
	} {
		var stdout, stderr bytes.Buffer
		code := Run(append([]string{"--root", root, "update"}, tc.args...), &stdout, &stderr)
		if code != tc.code {
			t.Fatalf("migration exit %d: %s %s", code, &stdout, &stderr)
		}
		if code == 0 {
			var result scaffolding.MigrationPlan
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || result.FromSchema != 1 || result.ToSchema != 2 || result.Applied {
				t.Fatalf("migration preview: %+v %v", result, err)
			}
		} else if stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.message) {
			t.Fatalf("migration failure: %s %s", &stdout, &stderr)
		}
		if !reflect.DeepEqual(before, workspaceBytes(t, root)) {
			t.Fatal("migration CLI changed bytes")
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("Owner customization"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--root", root, "update", "--migrate-native"}, &stdout, &stderr); code != 1 || !strings.Contains(stdout.String(), `"README.md"`) || stderr.Len() != 0 {
		t.Fatalf("conflict preview: %d %s %s", code, &stdout, &stderr)
	}
	before = workspaceBytes(t, root)
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"--root", root, "update", "--migrate-native", "--apply"}, &stdout, &stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "conflicts") {
		t.Fatalf("conflicting apply: %d %s %s", code, &stdout, &stderr)
	}
	if !reflect.DeepEqual(before, workspaceBytes(t, root)) {
		t.Fatal("conflicting migration wrote files")
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"--root", root, "update", "--migrate-native", "--keep", "README.md", "--apply"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), `"applied": true`) || stderr.Len() != 0 {
		t.Fatalf("explicit apply: %d %s %s", code, &stdout, &stderr)
	}
	migrated, err := scaffolding.LoadState(root)
	if err != nil || migrated.State.Version != 2 {
		t.Fatal("CLI did not commit native state")
	}
}
