package scaffolding

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
)

func collectionWorkspace(t *testing.T, selected ...string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workspace")
	if _, err := InitialiseSelected(root, "0.1.0", "workspace", selected); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCollectionsInitSelectionAndAllOptionalHomes(t *testing.T) {
	for _, selected := range [][]string{{"research", "archive", "archive"}, catalogue.CollectionNames()} {
		root := collectionWorkspace(t, selected...)
		state, err := LoadLock(root)
		want, _ := catalogue.SelectCollections(selected)
		if err != nil || !reflect.DeepEqual(mustSelection(t, state.Lock).Collections, want) {
			t.Fatalf("init selections: %+v %v", state.Lock, err)
		}
		files, err := catalogue.BundleSelected("workspace", selected)
		if err != nil || len(files) != len(state.Lock.Files) {
			t.Fatal("selected file inventory mismatch")
		}
		for name, body := range files {
			current, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
			if err != nil || !bytes.Equal(current, body) || state.Lock.Files[name] != lockDigest(body) {
				t.Fatalf("selected asset/hash mismatch: %s %v", name, err)
			}
		}
		for _, name := range catalogue.CollectionNames() {
			exists := false
			for file := range state.Lock.Files {
				exists = exists || strings.HasPrefix(file, name+"/")
			}
			found := false
			for _, enabled := range want {
				if name == enabled {
					found = true
				}
			}
			if exists != found {
				t.Fatalf("unexpected selected home: %s", name)
			}
		}
		plan, err := Update(root, "0.1.0", nil, false)
		if err != nil || len(plan.Actions) != 0 || len(plan.Conflicts) != 0 {
			t.Fatalf("selected init not settled: %+v %v", plan, err)
		}
	}
	parent := t.TempDir()
	for _, selection := range [][]string{{"unknown"}, {"../escape"}, {""}} {
		if _, err := InitialiseSelected(filepath.Join(parent, "bad"), "0.1.0", "workspace", selection); err == nil {
			t.Fatal("invalid init selection accepted")
		}
	}
	if _, err := InitialiseSelected(filepath.Join(parent, "bad"), "0.1.0", "project", []string{"archive"}); err == nil {
		t.Fatal("project collection accepted")
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid selection wrote output")
	}
}

func TestCollectionListingPreviewApplyAndIdempotence(t *testing.T) {
	if _, err := AddModules(collectionWorkspace(t), nil, false); err == nil {
		t.Fatal("empty addition accepted")
	}
	root := collectionWorkspace(t)
	before := snapshot(t, root)
	preview, err := AddModules(root, []string{"journal", "archive", "journal"}, false)
	want := []Action{{".agents/skills/review-week/SKILL.md", "write"}, {"AGENTS.md", "write"}, {"archive/AGENTS.md", "write"}, {"archive/README.md", "write"},
		{"journal/AGENTS.md", "write"}, {"journal/README.md", "write"}, {"journal/TEMPLATE.md", "write"}, {"wsup.json", "write"}}
	if err != nil || !reflect.DeepEqual(preview.Actions, want) || !reflect.DeepEqual(preview.Selected, []string{"archive", "journal"}) || preview.Applied {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("preview wrote files")
	}
	applied, err := AddModules(root, []string{"journal", "archive"}, true)
	if err != nil || !applied.Applied || !reflect.DeepEqual(applied.Actions, want) {
		t.Fatalf("apply: %+v %v", applied, err)
	}
	before = snapshot(t, root)
	again, err := AddModules(root, []string{"journal", "archive"}, true)
	if err != nil || !again.Applied || len(again.Actions) != 0 || !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatalf("not idempotent: %+v %v", again, err)
	}
	writeFixture(t, root, "archive/README.md", []byte("Owner customization"))
	stateBefore, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	before = snapshot(t, root)
	again, err = AddModules(root, []string{"archive"}, true)
	if err != nil || len(again.Actions) != 0 || !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("selected customized scaffold rewritten")
	}
	stateAfter, err := LoadLock(root)
	if err != nil || !bytes.Equal(stateBefore.Original, stateAfter.Original) {
		t.Fatal("selected customization adopted")
	}
}

func TestCollectionConflictsBlockAllAdditionsEvenIdenticalUnmanagedBytes(t *testing.T) {
	for _, managed := range []bool{false, true} {
		root := collectionWorkspace(t)
		bundle, err := catalogue.BundleSelected("workspace", []string{"archive"})
		if err != nil {
			t.Fatal(err)
		}
		if managed {
			if _, err := AddModules(root, []string{"archive"}, true); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(root, "archive/README.md")); err != nil {
				t.Fatal(err)
			}
		} else {
			writeFixture(t, root, "archive/README.md", bundle["archive/README.md"])
		}
		before := snapshot(t, root)
		preview, err := AddModules(root, []string{"archive", "releases"}, false)
		if err != nil || !reflect.DeepEqual(preview.Conflicts, []string{"archive/README.md"}) {
			t.Fatalf("conflicts: %+v %v", preview, err)
		}
		result, err := AddModules(root, []string{"archive", "releases"}, true)
		if err == nil || result.Applied || !reflect.DeepEqual(before, snapshot(t, root)) {
			t.Fatal("conflict did not block all additions")
		}
	}
}

func TestCollectionStaleStateDestinationAndPartialIOFailure(t *testing.T) {
	for _, change := range []string{LockPath, "archive/README.md"} {
		root := collectionWorkspace(t)
		_, prepared, err := planAdd(root, []string{"archive"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		writeFixture(t, root, change, []byte("owner concurrent bytes"))
		before := snapshot(t, root)
		if _, err := prepared.apply(); err == nil || !reflect.DeepEqual(before, snapshot(t, root)) {
			t.Fatal("stale collection plan wrote content")
		}
	}
	root := collectionWorkspace(t)
	_, prepared, err := planAdd(root, []string{"archive", "journal"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	injected := errors.New("synthetic state write failure")
	_, err = prepared.applyWith(func(handle *os.Root, name string, body []byte, exclusive bool) error {
		if name == LockPath {
			return injected
		}
		return writeManaged(handle, name, body, exclusive)
	})
	if !errors.Is(err, injected) || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("partial failure not surfaced: %v", err)
	}
	current, err := LoadLock(root)
	if err != nil || !bytes.Equal(current.Original, prepared.lockBefore) {
		t.Fatal("partial collection update committed state")
	}
	if _, err := os.Stat(filepath.Join(root, "journal/TEMPLATE.md")); err != nil {
		t.Fatal("partial files not observable")
	}
}

func TestCollectionsInvalidInputsAndSymlinkRefusal(t *testing.T) {
	root := collectionWorkspace(t)
	before := snapshot(t, root)
	for _, names := range [][]string{nil, {"unknown"}, {"../escape"}} {
		if _, err := AddModules(root, names, true); err == nil {
			t.Fatalf("invalid addition accepted: %v", names)
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("invalid addition changed bytes")
	}
	project := filepath.Join(t.TempDir(), "project")
	if _, err := InitialiseProject(project, "0.1.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := AddModules(project, []string{"archive"}, true); err == nil {
		t.Fatal("project addition accepted")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "archive")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	before, external := snapshot(t, root), snapshot(t, outside)
	if _, err := AddModules(root, []string{"archive"}, true); err == nil {
		t.Fatal("symlink addition accepted")
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) || !reflect.DeepEqual(external, snapshot(t, outside)) {
		t.Fatal("symlink target or owner modified")
	}
}

func TestCollectionAdditionInstallsOwnedSkillsOnce(t *testing.T) {
	root := collectionWorkspace(t)
	skills := []string{"diagnose-issue", "infer-conventions", "make-change", "review-change", "write-commit"}
	want := []Action{}
	for _, skill := range skills {
		want = append(want, Action{".agents/skills/" + skill + "/SKILL.md", "write"})
	}
	want = append(want, Action{"AGENTS.md", "write"}, Action{"apps/AGENTS.md", "write"}, Action{"apps/README.md", "write"}, Action{"wsup.json", "write"})
	added, err := AddModules(root, []string{"apps"}, true)
	if err != nil || !added.Applied || !reflect.DeepEqual(added.Actions, want) {
		t.Fatalf("first project collection: %+v %v", added, err)
	}
	added, err = AddModules(root, []string{"tools"}, true)
	if err != nil || !added.Applied || !reflect.DeepEqual(added.Actions, []Action{{"AGENTS.md", "write"}, {"tools/AGENTS.md", "write"}, {"tools/README.md", "write"}, {"wsup.json", "write"}}) {
		t.Fatalf("second project collection: %+v %v", added, err)
	}
	plan, err := Update(root, "0.1.0", nil, false)
	if err != nil || len(plan.Actions) != 0 || len(plan.Conflicts) != 0 {
		t.Fatalf("additions did not settle: %+v %v", plan, err)
	}
}

func TestDataIgnoreEntryRendersIntoTheGitignoreBlock(t *testing.T) {
	block := "# wsup:begin ignore\ndata/**/*.local.*\n# wsup:end ignore\n"
	root := collectionWorkspace(t, "data")
	body, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil || !strings.HasSuffix(string(body), block) {
		t.Fatalf("init ignore block: %q %v", body, err)
	}
	git := exec.Command("git", "init", "--quiet", ".")
	git.Dir = root
	if output, err := git.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s %v", output, err)
	}
	git = exec.Command("git", "-c", "core.excludesFile=/dev/null", "check-ignore", "--no-index", "-q", "data/samples/survey.local.csv")
	git.Dir = root
	if err := git.Run(); err != nil {
		t.Fatalf("data local copy not ignored: %v", err)
	}

	// Adding data later rewrites an unedited .gitignore with the entry.
	root = collectionWorkspace(t)
	added, err := AddModules(root, []string{"data"}, true)
	want := []Action{{".gitignore", "write"}, {"AGENTS.md", "write"}, {"data/AGENTS.md", "write"}, {"data/README.md", "write"}, {"wsup.json", "write"}}
	if err != nil || !added.Applied || !reflect.DeepEqual(added.Actions, want) {
		t.Fatalf("data addition: %+v %v", added, err)
	}
	body, err = os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil || !strings.HasSuffix(string(body), block) {
		t.Fatalf("added ignore block: %q %v", body, err)
	}
	if plan, err := Update(root, "0.1.0", nil, false); err != nil || len(plan.Actions) != 0 || len(plan.Conflicts) != 0 {
		t.Fatalf("data addition did not settle: %+v %v", plan, err)
	}

	// Owner edits outside the block are kept; only the block is replaced.
	root = collectionWorkspace(t)
	owner := "/apps/independent/\n"
	writeFixture(t, root, ".gitignore", append([]byte(owner), must(os.ReadFile(filepath.Join(root, ".gitignore")))...))
	if added, err := AddModules(root, []string{"data"}, true); err != nil || !added.Applied {
		t.Fatalf("edited ignore addition: %+v %v", added, err)
	}
	body = must(os.ReadFile(filepath.Join(root, ".gitignore")))
	if !strings.HasPrefix(string(body), owner) || !strings.HasSuffix(string(body), block) {
		t.Fatalf("edited ignore splice: %q", body)
	}

	// An edit inside the generated block is a conflict, never overwritten.
	root = collectionWorkspace(t)
	inside := strings.Replace(string(must(os.ReadFile(filepath.Join(root, ".gitignore")))), "# wsup:end ignore", "owner-entry\n# wsup:end ignore", 1)
	writeFixture(t, root, ".gitignore", []byte(inside))
	before := snapshot(t, root)
	if added, err := AddModules(root, []string{"data"}, true); err == nil || added.Applied ||
		!reflect.DeepEqual(added.Conflicts, []string{".gitignore"}) || !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatalf("block-edited ignore addition: %+v %v", added, err)
	}
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

func TestClaudeAdapterIgnoresLocalSettingsInWorkspacesAndProjects(t *testing.T) {
	entry := "# wsup:begin ignore\n.claude/settings.local.json\n# wsup:end ignore\n"
	for _, kind := range []string{"workspace", "project"} {
		root := filepath.Join(t.TempDir(), kind)
		if _, err := InitialiseWith(root, "0.1.0", kind, catalogue.Selection{Agents: []string{"claude"}}); err != nil {
			t.Fatal(err)
		}
		if body := must(os.ReadFile(filepath.Join(root, ".gitignore"))); !strings.HasSuffix(string(body), entry) {
			t.Fatalf("%s .gitignore: %q", kind, body)
		}
		// Adding the adapter later merges the entry into an edited .gitignore.
		bare := filepath.Join(t.TempDir(), kind+"-bare")
		if _, err := InitialiseWith(bare, "0.1.0", kind, catalogue.Selection{Agents: []string{}}); err != nil {
			t.Fatal(err)
		}
		writeFixture(t, bare, ".gitignore", append([]byte("/owner/\n"), must(os.ReadFile(filepath.Join(bare, ".gitignore")))...))
		plan, err := AddModules(bare, []string{"claude"}, true)
		if err != nil || !plan.Applied || !slices.Contains(plan.Actions, Action{".gitignore", "merge"}) {
			t.Fatalf("%s add claude: %+v %v", kind, plan, err)
		}
		body := string(must(os.ReadFile(filepath.Join(bare, ".gitignore"))))
		if !strings.HasPrefix(body, "/owner/\n") || !strings.HasSuffix(body, entry) {
			t.Fatalf("%s merged .gitignore: %q", kind, body)
		}
	}
}

func TestAddKeepsOwnerFilesAtInstalledPathsWhenAsked(t *testing.T) {
	root := collectionWorkspace(t)
	writeFixture(t, root, "journal/README.md", []byte("Owner journal\n"))
	if plan, err := AddModules(root, []string{"journal"}, false); err != nil || !reflect.DeepEqual(plan.Conflicts, []string{"journal/README.md"}) {
		t.Fatalf("without keep: %+v %v", plan, err)
	}
	if _, err := AddModulesKeeping(root, []string{"journal"}, []string{"journal/TEMPLATE.md"}, true); err == nil {
		t.Fatal("keep of a file that does not exist accepted")
	}
	plan, err := AddModulesKeeping(root, []string{"journal"}, []string{"journal/README.md"}, true)
	if err != nil || !plan.Applied || !slices.Contains(plan.Actions, Action{"journal/README.md", "keep"}) ||
		!slices.Contains(plan.Actions, Action{"journal/TEMPLATE.md", "write"}) {
		t.Fatalf("keep: %+v %v", plan, err)
	}
	if body := string(must(os.ReadFile(filepath.Join(root, "journal/README.md")))); body != "Owner journal\n" {
		t.Fatalf("kept file changed: %q", body)
	}
	if update, err := Update(root, "0.1.0", nil, false); err != nil || !reflect.DeepEqual(update.Conflicts, []string{"journal/README.md"}) {
		t.Fatalf("update: %+v %v", update, err)
	}
}
