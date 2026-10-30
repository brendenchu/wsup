package scaffolding

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, root, name string, body []byte) {
	t.Helper()
	filename := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, body, 0600); err != nil {
		t.Fatal(err)
	}
}

func nativeFixture(t *testing.T, files map[string][]byte) (string, Lock) {
	t.Helper()
	root := t.TempDir()
	lock := Lock{Version: LockVersion, WsupVersion: "0.1.0", Starter: "workspace",
		Modules: map[string]string{"workspace-base": "1.0.0"}, Files: map[string]string{}}
	for name, body := range files {
		lock.Files[name] = lockDigest(body)
		writeFixture(t, root, name, body)
	}
	saveLock(t, root, lock)
	return root, lock
}

func saveLock(t *testing.T, root string, lock Lock) {
	t.Helper()
	body, err := EncodeLock(lock)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, LockPath, body)
}

// Snapshot includes directories and symlink destinations as well as file bytes.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(filename string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name, err := filepath.Rel(root, filename)
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			result[name] = "directory"
		case entry.Type()&os.ModeSymlink != 0:
			target, err := os.Readlink(filename)
			if err != nil {
				return err
			}
			result[name] = "link:" + target
		default:
			body, err := os.ReadFile(filename)
			if err != nil {
				return err
			}
			result[name] = "file:" + string(body)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestPlanNativeUpdateClassificationsAndReadOnlyRepeatability(t *testing.T) {
	root, _ := nativeFixture(t, map[string][]byte{
		"README.md":  []byte("old"),
		"same.md":    []byte("same"),
		"custom.md":  []byte("old custom"),
		"missing.md": []byte("missing"),
		"orphan.md":  []byte("retired starter"),
	})
	writeFixture(t, root, "custom.md", []byte("owner changes"))
	writeFixture(t, root, "wsup.json", []byte("owner registry"))
	writeFixture(t, root, "collision.md", []byte("owner collision"))
	writeFixture(t, root, "already.md", []byte("already new"))
	writeFixture(t, root, "workbench/.local/wsup/reports/evidence.json", []byte("historical evidence"))
	writeFixture(t, root, "tools/project/source.go", []byte("independent project"))
	writeFixture(t, root, "wsup.local.json", []byte("owner settings"))
	if err := os.Remove(filepath.Join(root, "missing.md")); err != nil {
		t.Fatal(err)
	}
	bundle := map[string][]byte{
		"README.md": []byte("new"), "same.md": []byte("same"),
		"custom.md": []byte("new custom"), "missing.md": []byte("missing"),
		"new/nested.md": []byte("new file"),
		"collision.md":  []byte("starter collision"), "already.md": []byte("already new"),
	}
	before := snapshot(t, root)
	plan, err := PlanUpdate(root, "0.2.0", bundle, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := UpdatePlan{
		FromVersion: "0.1.0", ToVersion: "0.2.0",
		Actions:   []Action{{"README.md", "write"}, {"new/nested.md", "write"}, {"orphan.md", "retain-orphan"}},
		Conflicts: []string{"collision.md", "custom.md", "missing.md"},
	}
	if !reflect.DeepEqual(plan, want) {
		t.Fatalf("plan = %+v; want %+v", plan, want)
	}
	again, err := PlanUpdate(root, "0.2.0", bundle, nil)
	if err != nil || !reflect.DeepEqual(plan, again) {
		t.Fatalf("unstable repeat preview: %+v %v", again, err)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("preview changed workspace bytes or directory inventory")
	}
}

func TestPlanKeepPreservesOriginalBaselines(t *testing.T) {
	root, state := nativeFixture(t, map[string][]byte{"README.md": []byte("starter")})
	writeFixture(t, root, "README.md", []byte("owner readme"))
	bundle := map[string][]byte{"README.md": []byte("new starter")}
	before := snapshot(t, root)
	plan, err := PlanUpdate(root, "0.2.0", bundle, []string{"README.md", "README.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan.Actions, []Action{{"README.md", "keep-customization"}}) || len(plan.Conflicts) != 0 {
		t.Fatalf("unexpected keep plan: %+v", plan)
	}
	unkept, err := PlanUpdate(root, "0.2.0", bundle, nil)
	if err != nil || !reflect.DeepEqual(unkept.Conflicts, []string{"README.md"}) {
		t.Fatalf("keep permanently accepted owner hashes: %+v %v", unkept, err)
	}
	loaded, err := LoadLock(root)
	if err != nil || !reflect.DeepEqual(loaded.Lock, state) || !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("planning changed baselines, modules, or files")
	}
}

func TestPlanInvalidKeepBundleAndUnsupportedStateRefuseWithoutWrites(t *testing.T) {
	root, state := nativeFixture(t, map[string][]byte{"README.md": []byte("old")})
	valid := map[string][]byte{"README.md": []byte("new")}
	before := snapshot(t, root)
	if _, err := PlanUpdate(root, "0.2.0", nil, nil); err == nil {
		t.Fatal("unavailable catalogue treated as an empty template")
	}
	for _, name := range []string{"unknown.md", "../outside", LockPath} {
		if _, err := PlanUpdate(root, "0.2.0", valid, []string{name}); err == nil {
			t.Fatalf("invalid keep accepted: %s", name)
		}
	}
	for _, name := range []string{"../outside", "wsup.json", LockPath, "bad\xffname"} {
		if _, err := PlanUpdate(root, "0.2.0", map[string][]byte{name: []byte("bad")}, nil); err == nil {
			t.Fatalf("unsafe bundle accepted: %q", name)
		}
	}
	if _, err := PlanUpdate(root, "", valid, nil); err == nil {
		t.Fatal("empty target version accepted")
	}
	if _, err := PlanUpdate(root, "0.2.0", map[string][]byte{"dir": {}, "dir/file": {}}, nil); err == nil {
		t.Fatal("file/parent collision in bundle accepted")
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("invalid planning changed files")
	}
	state.Version = 2
	body, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, LockPath, body)
	before = snapshot(t, root)
	if _, err := PlanUpdate(root, "0.2.0", valid, nil); err == nil || !strings.Contains(err.Error(), "unsupported wsup.lock version 2") {
		t.Fatalf("unsupported state not explicitly gated: %v", err)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("unsupported planning changed files")
	}
}

func TestPlanDirectoriesMissingKeepsAndRegistryOwnership(t *testing.T) {
	root, _ := nativeFixture(t, map[string][]byte{"README.md": []byte("old")})
	writeFixture(t, root, "wsup.json", []byte("registry"))
	if err := os.Remove(filepath.Join(root, "README.md")); err != nil {
		t.Fatal(err)
	}
	bundle := map[string][]byte{"README.md": []byte("new")}
	if _, err := PlanUpdate(root, "0.2.0", bundle, []string{"README.md"}); err == nil {
		t.Fatal("missing managed file accepted by keep")
	}
	if err := os.Mkdir(filepath.Join(root, "README.md"), 0755); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanUpdate(root, "0.2.0", bundle, nil)
	if err != nil || !reflect.DeepEqual(plan.Conflicts, []string{"README.md"}) {
		t.Fatalf("directory collision not reported: %+v %v", plan, err)
	}
	if err := os.Remove(filepath.Join(root, "wsup.json")); err != nil {
		t.Fatal(err)
	}
	plan, err = PlanUpdate(root, "0.2.0", bundle, nil)
	if err != nil || len(plan.Actions) != 0 {
		t.Fatalf("missing user registry recreated: %+v %v", plan, err)
	}
	body, err := json.Marshal(plan)
	if err != nil || !bytes.Contains(body, []byte(`"actions":[]`)) || !bytes.Contains(body, []byte(`"applied":false`)) {
		t.Fatalf("incorrect preview JSON shape: %s %v", body, err)
	}
}

func TestPlanRefusesSymlinkComponentsAndNonDirectoryParents(t *testing.T) {
	for _, link := range []string{"docs", "docs/rule.md"} {
		t.Run(link, func(t *testing.T) {
			root, _ := nativeFixture(t, map[string][]byte{})
			outside := t.TempDir()
			writeFixture(t, outside, "rule.md", []byte("owner bytes"))
			target := outside
			if link != "docs" {
				if err := os.Mkdir(filepath.Join(root, "docs"), 0755); err != nil {
					t.Fatal(err)
				}
				target = filepath.Join(outside, "rule.md")
			}
			if err := os.Symlink(target, filepath.Join(root, filepath.FromSlash(link))); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			before, external := snapshot(t, root), snapshot(t, outside)
			if _, err := PlanUpdate(root, "0.2.0", map[string][]byte{"docs/rule.md": []byte("new")}, nil); err == nil {
				t.Fatal("symlink destination accepted")
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) || !reflect.DeepEqual(external, snapshot(t, outside)) {
				t.Fatal("symlink target or workspace modified")
			}
		})
	}
	root, _ := nativeFixture(t, map[string][]byte{"docs": []byte("not a directory")})
	if _, err := PlanUpdate(root, "0.2.0", map[string][]byte{"docs/rule.md": []byte("new")}, nil); err == nil {
		t.Fatal("non-directory parent accepted")
	}
}

func TestPlanUsesSHA256AndSupportsProjectState(t *testing.T) {
	root, state := nativeFixture(t, map[string][]byte{"README.md": []byte("abc")})
	const abcSHA256 = "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if state.Files["README.md"] != abcSHA256 {
		t.Fatalf("incorrect SHA-256 baseline: %s", state.Files["README.md"])
	}
	state.Starter, state.Modules = "project", map[string]string{"project-base": "1.0.0"}
	saveLock(t, root, state)
	before := snapshot(t, root)
	plan, err := PlanUpdate(root, "0.2.0", map[string][]byte{"README.md": []byte("updated")}, nil)
	if err != nil || !reflect.DeepEqual(plan.Actions, []Action{{"README.md", "write"}}) || len(plan.Conflicts) != 0 {
		t.Fatalf("project preview: %+v %v", plan, err)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("project preview changed files")
	}
}
