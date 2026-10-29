package catalogue

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func manifest(fields string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(`{` + fields + `}`)}
}

func base(starter string) string {
	return `"id":"` + starter + `-base","kind":"base","version":"1.0.0","description":"Base.","applies_to":["` + starter + `"],"requires":[],"skills":[],"ignore":[],"generate":[]`
}

func testTree(extra fstest.MapFS) fstest.MapFS {
	tree := fstest.MapFS{
		"skills/plan/SKILL.md":                   {Data: []byte("plan")},
		"modules/workspace-base/module.json":     manifest(base("workspace")),
		"modules/workspace-base/files/README.md": {Data: []byte("workspace")},
		"modules/project-base/module.json":       manifest(base("project")),
		"modules/notes/module.json":              manifest(`"id":"notes","kind":"collection","version":"1.0.0","description":"Notes.","applies_to":["workspace"],"requires":[],"skills":["plan"],"ignore":[],"generate":[]`),
		"modules/notes/files/notes/README.md":    {Data: []byte("notes")},
		"modules/drafts/module.json":             manifest(`"id":"drafts","kind":"collection","version":"1.0.0","description":"Drafts.","applies_to":["workspace"],"requires":["notes"],"skills":["plan"],"ignore":[],"generate":[]`),
		"modules/drafts/files/drafts/README.md":  {Data: []byte("drafts")},
		"modules/writing/module.json":            manifest(`"id":"writing","kind":"preset","version":"1.0.0","description":"Writing.","applies_to":[],"requires":[],"skills":[],"ignore":[],"generate":[],"modules":["drafts"]`),
	}
	for name, file := range extra {
		if file == nil {
			delete(tree, name)
		} else {
			tree[name] = file
		}
	}
	return tree
}

func TestBuiltInCatalogueLoads(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, starter := range starters {
		if _, ok := c.Module(starter + "-base"); !ok {
			t.Fatalf("missing %s-base", starter)
		}
	}
	if !reflect.DeepEqual(c.IDs(KindCollection), CollectionNames()) || len(CollectionNames()) == 0 {
		t.Fatalf("collection ids: %v", c.IDs(KindCollection))
	}
}

func TestResolveExpandsPresetsAndRequiresOnce(t *testing.T) {
	c, err := load(testTree(nil))
	if err != nil {
		t.Fatal(err)
	}
	ids, err := c.Resolve("workspace", []string{"writing", "notes", "drafts"})
	if err != nil || !reflect.DeepEqual(ids, []string{"workspace-base", "drafts", "notes"}) {
		t.Fatalf("resolve: %v %v", ids, err)
	}
	files, err := c.Files(ids)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".agents/skills/plan/SKILL.md", "README.md", "drafts/README.md", "notes/README.md"}
	got := []string{}
	for name := range files {
		got = append(got, name)
	}
	if len(got) != len(want) {
		t.Fatalf("files: %v", got)
	}
	for _, name := range want {
		if _, ok := files[name]; !ok {
			t.Fatalf("missing %s in %v", name, got)
		}
	}
	for _, selection := range [][]string{{"unknown"}, {"project-base"}} {
		if _, err := c.Resolve("workspace", selection); err == nil {
			t.Fatalf("invalid selection accepted: %v", selection)
		}
	}
	if _, err := c.Resolve("project", []string{"notes"}); err == nil {
		t.Fatal("workspace-only module accepted for a project")
	}
	if _, err := c.Resolve("library", nil); err == nil {
		t.Fatal("unknown starter accepted")
	}
}

func TestFilesRejectsTwoModulesWritingOnePath(t *testing.T) {
	c, err := load(testTree(fstest.MapFS{"modules/drafts/files/notes/README.md": {Data: []byte("clash")}}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Files([]string{"notes", "drafts"}); err == nil || !strings.Contains(err.Error(), "both install") {
		t.Fatalf("path collision: %v", err)
	}
}

func TestLoadRejectsInvalidModules(t *testing.T) {
	collection := func(fields string) *fstest.MapFile {
		return manifest(`"id":"notes","version":"1.0.0","description":"Notes.","applies_to":["workspace"],"ignore":[],"generate":[],` + fields)
	}
	for name, change := range map[string]fstest.MapFS{
		"unknown field":       {"modules/notes/module.json": collection(`"kind":"collection","skills":[],"extra":true,"requires":[]`)},
		"unknown kind":        {"modules/notes/module.json": collection(`"kind":"plugin","skills":[],"requires":[]`)},
		"unknown skill":       {"modules/notes/module.json": collection(`"kind":"collection","skills":["missing"],"requires":[]`)},
		"id mismatch":         {"modules/notes/module.json": manifest(`"id":"other","kind":"collection","version":"1.0.0","description":"x","applies_to":["workspace"],"requires":[],"skills":[],"ignore":[],"generate":[]`)},
		"bad version":         {"modules/notes/module.json": manifest(`"id":"notes","kind":"collection","version":"1","description":"x","applies_to":["workspace"],"requires":[],"skills":[],"ignore":[],"generate":[]`)},
		"unknown generator":   {"modules/notes/module.json": manifest(`"id":"notes","kind":"collection","version":"1.0.0","description":"x","applies_to":["workspace"],"requires":[],"skills":[],"ignore":[],"generate":["shell"]`)},
		"requires preset":     {"modules/notes/module.json": collection(`"kind":"collection","skills":[],"requires":["writing"]`)},
		"preset with files":   {"modules/writing/files/x.md": {Data: []byte("x")}},
		"preset bad member":   {"modules/writing/module.json": manifest(`"id":"writing","kind":"preset","version":"1.0.0","description":"x","applies_to":[],"requires":[],"skills":[],"ignore":[],"generate":[],"modules":["workspace-base"]`)},
		"stray entry":         {"modules/notes/notes.txt": {Data: []byte("x")}},
		"skill path in files": {"modules/notes/files/.agents/skills/plan/SKILL.md": {Data: []byte("x")}},
		"missing base":        {"modules/project-base/module.json": nil},
		"skill without body":  {"skills/empty/notes.md": {Data: []byte("x")}},
		"reserved id":         {"modules/none/module.json": manifest(`"id":"none","kind":"collection","version":"1.0.0","description":"x","applies_to":["workspace"],"requires":[],"skills":[],"ignore":[],"generate":[]`)},
	} {
		if _, err := load(testTree(change)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
