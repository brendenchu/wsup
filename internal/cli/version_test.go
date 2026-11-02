package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wavykit/wsup/internal/scaffolding"
)

func TestProductVersionPropagatesToCLIStateAndUpdatePlans(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--version"}, &stdout, &stderr); code != 0 ||
		stdout.String() != Version+"\n" || stderr.Len() != 0 {
		t.Fatalf("product version output: %d %q %q", code, &stdout, &stderr)
	}
	changelog, err := os.ReadFile("../../CHANGELOG.md")
	if err != nil || !strings.Contains(string(changelog), "## "+Version+" (") {
		t.Fatalf("product version missing from changelog: %v", err)
	}
	for _, kind := range []string{"workspace", "project"} {
		t.Run(kind, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), kind)
			stdout.Reset()
			stderr.Reset()
			if code := Run([]string{"init", root, "--template", kind}, &stdout, &stderr); code != 0 {
				t.Fatalf("init: %d %s", code, &stderr)
			}
			loaded, err := scaffolding.LoadLock(root)
			if err != nil || loaded.Lock.WsupVersion != Version || loaded.Lock.Version != scaffolding.LockVersion {
				t.Fatalf("generated product/schema version: %+v %v", loaded.Lock, err)
			}
			loaded.Lock.WsupVersion = "0.1.0"
			body, err := scaffolding.EncodeLock(loaded.Lock)
			if err != nil {
				t.Fatal(err)
			}
			filename := filepath.Join(root, filepath.FromSlash(scaffolding.LockPath))
			if err := os.WriteFile(filename, body, 0600); err != nil {
				t.Fatal(err)
			}
			for _, apply := range []bool{false, true} {
				stdout.Reset()
				stderr.Reset()
				args := []string{"--root", root, "update"}
				if apply {
					args = append(args, "--apply")
				}
				if code := Run(args, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
					t.Fatalf("update: %d %s", code, &stderr)
				}
				var plan scaffolding.UpdatePlan
				if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil ||
					plan.FromVersion != "0.1.0" || plan.ToVersion != Version ||
					plan.Applied != apply || len(plan.Actions) != 0 || len(plan.Conflicts) != 0 {
					t.Fatalf("product-version plan: %+v %v", plan, err)
				}
				current, err := scaffolding.LoadLock(root)
				if err != nil || current.Lock.Version != scaffolding.LockVersion {
					t.Fatalf("update changed schema: %+v %v", current.Lock, err)
				}
				want := "0.1.0"
				if apply {
					want = Version
				}
				if current.Lock.WsupVersion != want {
					t.Fatalf("state product version: %q, want %q", current.Lock.WsupVersion, want)
				}
			}
		})
	}
}
