package scaffolding

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/brendenchu/wsup/internal/guidance"
)

func legacyGuidanceFixture(t *testing.T, kind string) string {
	t.Helper()
	root := collectionWorkspace(t)
	if kind == "project" {
		root = filepath.Join(t.TempDir(), "project")
		if _, err := InitialiseProject(root, "0.1.0"); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	loaded.State.Version = 1
	for name, digest := range loaded.State.Files {
		previous := name
		if strings.HasPrefix(name, "guidance/rules/") {
			previous = strings.Replace(name, "guidance/rules/", ".ai/rules/", 1)
		} else if strings.HasPrefix(name, ".agents/skills/") {
			previous = strings.Replace(name, ".agents/skills/", ".ai/skills/", 1)
		}
		if previous == name {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		writeFixture(t, root, previous, body)
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Fatal(err)
		}
		delete(loaded.State.Files, name)
		loaded.State.Files[previous] = digest
	}
	if kind == "workspace" {
		for name, body := range map[string]string{
			"tools/wsup.py": "Old runtime",
			"tools/wsup_templates/workspace/.ai/skills/workspace-check/SKILL.md": "Old bundled skill",
			"tools/wsup_templates/workspace/apps/README.md":                      "Obsolete bundled home",
			"templates/project/.ai/rules/index.md":                               "Obsolete project catalogue",
		} {
			writeFixture(t, root, name, []byte(body))
			loaded.State.Files[name] = contentDigest([]byte(body))
		}
	}
	writeFixture(t, root, StatePath, encoded(t, map[string]any{
		"version": loaded.State.Version, "template_version": loaded.State.TemplateVersion,
		"template": loaded.State.Template, "files": loaded.State.Files,
	}))
	return root
}

func TestLegacyGuidanceAndSkillsMigrateForWorkspaceAndProject(t *testing.T) {
	for _, kind := range []string{"workspace", "project"} {
		t.Run(kind, func(t *testing.T) {
			root := legacyGuidanceFixture(t, kind)
			writeFixture(t, root, "apps/README.md", []byte("Owner home"))
			writeFixture(t, root, ".work/investigation/raw.txt", []byte("Owner evidence"))
			writeFixture(t, root, ".work/quality/old.json", []byte("Report provenance"))
			before := snapshot(t, root)
			preview, err := PlanMigration(root, "0.2.0", nil)
			if err != nil || len(preview.Conflicts) != 0 || !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatalf("legacy preview: %+v %v", preview, err)
			}
			skills := 0
			for _, action := range preview.Actions {
				if action.Action == "relocate" && strings.HasPrefix(action.From, ".ai/skills/") {
					skills++
				}
			}
			if kind == "workspace" && skills != 12 {
				t.Fatalf("missing skill relocations: %d", skills)
			}
			result, err := ApplyMigration(root, "0.2.0", nil)
			if err != nil || !result.Applied {
				t.Fatalf("legacy apply: %+v %v", result, err)
			}
			for _, name := range []string{".ai", "tools/wsup.py", "tools/wsup_templates", "templates", ".work/quality", ".wsup/wsup.py"} {
				if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(name))); !os.IsNotExist(err) {
					t.Fatalf("legacy/runtime path remains: %s", name)
				}
			}
			for name, want := range map[string]string{
				"apps/README.md": "Owner home", ".work/investigation/raw.txt": "Owner evidence",
				".wsup/reports/old.json": "Report provenance",
			} {
				body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
				if err != nil || string(body) != want {
					t.Fatalf("owner/provenance changed: %s", name)
				}
			}
			inspection, err := guidance.Inspect(root)
			if err != nil || len(inspection.Errors) != 0 || (kind == "workspace" && len(inspection.Skills) != 12) {
				t.Fatalf("migrated guidance: %+v %v", inspection, err)
			}
			settled, err := Update(root, "0.2.0", nil, true)
			if err != nil || len(settled.Actions) != 0 || len(settled.Conflicts) != 0 {
				t.Fatalf("repeat native apply: %+v %v", settled, err)
			}
		})
	}
}

func TestLegacySkillConflictsAndSymlinksPreserveEveryFile(t *testing.T) {
	source := ".ai/skills/workspace-check/SKILL.md"
	destination := ".agents/skills/workspace-check/SKILL.md"
	for _, change := range []string{"edited", "missing", "collision", "unknown", "unmanaged", "embedded-edit", "obsolete-edit", "source-symlink", "destination-symlink"} {
		t.Run(change, func(t *testing.T) {
			root := legacyGuidanceFixture(t, "workspace")
			external := t.TempDir()
			writeFixture(t, external, "owner.md", []byte("External owner"))
			conflict := source
			switch change {
			case "edited":
				writeFixture(t, root, source, []byte("Owner skill customization"))
			case "missing":
				if err := os.Remove(filepath.Join(root, filepath.FromSlash(source))); err != nil {
					t.Fatal(err)
				}
			case "collision":
				body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(source)))
				if err != nil {
					t.Fatal(err)
				}
				writeFixture(t, root, destination, body)
				conflict = destination
			case "unknown":
				conflict = ".ai/skills/workspace-check/notes.txt"
				writeFixture(t, root, conflict, []byte("Owner notes"))
			case "unmanaged":
				loaded, err := LoadState(root)
				if err != nil {
					t.Fatal(err)
				}
				delete(loaded.State.Files, source)
				writeFixture(t, root, StatePath, encoded(t, map[string]any{
					"version": 1, "template_version": "0.1.0", "template": "workspace", "files": loaded.State.Files,
				}))
			case "embedded-edit", "obsolete-edit":
				conflict = "tools/wsup_templates/workspace/.ai/skills/workspace-check/SKILL.md"
				if change == "obsolete-edit" {
					conflict = "tools/wsup_templates/workspace/apps/README.md"
				}
				writeFixture(t, root, conflict, []byte("Owner catalogue customization"))
			case "source-symlink", "destination-symlink":
				name := source
				if change == "destination-symlink" {
					name = destination
				} else if err := os.Remove(filepath.Join(root, filepath.FromSlash(name))); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(external, "owner.md"), filepath.Join(root, filepath.FromSlash(name))); err != nil {
					t.Fatal(err)
				}
			}
			before, outside := snapshot(t, root), snapshot(t, external)
			preview, err := PlanMigration(root, "0.2.0", nil)
			if strings.HasSuffix(change, "symlink") {
				if err == nil {
					t.Fatal("symlink accepted")
				}
			} else if err != nil || !slices.Contains(preview.Conflicts, conflict) {
				t.Fatalf("missing conflict %s: %+v %v", conflict, preview, err)
			}
			if _, err := ApplyMigration(root, "0.2.0", nil); err == nil {
				t.Fatal("unsafe legacy apply accepted")
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) || !reflect.DeepEqual(outside, snapshot(t, external)) {
				t.Fatal("blocked legacy migration changed owner bytes")
			}
		})
	}
}

func TestLegacySkillWriteFailuresKeepOriginalSourcesAndState(t *testing.T) {
	for _, stage := range []string{"first", "later", "state"} {
		t.Run(stage, func(t *testing.T) {
			root := legacyGuidanceFixture(t, "workspace")
			var prepared preparedMigration
			if _, err := planMigration(root, "0.2.0", nil, &prepared); err != nil {
				t.Fatal(err)
			}
			before := snapshot(t, root)
			injected := errors.New("synthetic skill write failure")
			writes := 0
			result, err := prepared.applyWith(func(handle *os.Root, name string, body []byte, exclusive bool) error {
				writes++
				if (stage == "first" && writes == 1) || (stage == "later" && writes == 3) || (stage == "state" && name == StatePath) {
					return injected
				}
				return writeManaged(handle, name, body, exclusive)
			}, func(*os.Root, string) error {
				t.Fatal("sources retired before failed state commit")
				return nil
			})
			if !errors.Is(err, injected) || result.Applied {
				t.Fatalf("failure not surfaced: %+v %v", result, err)
			}
			after := snapshot(t, root)
			for name, body := range before {
				if after[name] != body {
					t.Fatalf("source/state changed on failure: %s", name)
				}
			}

		})
	}
}

func TestUnselectedFormerDefaultsRetainOwnerBytesAndRetireCatalogueCopies(t *testing.T) {
	root := pythonFixture(t)
	loaded, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := catalogue.BundleSelected("workspace", []string{"journal", "learning", "reference", "research"})
	if err != nil {
		t.Fatal(err)
	}
	homes := []string{}
	for name, body := range files {
		if _, managed := loaded.State.Files[name]; managed {
			continue
		}
		homes = append(homes, name)
		for _, filename := range []string{name, ".wsup/templates/workspace/" + name} {
			writeFixture(t, root, filename, body)
			loaded.State.Files[filename] = contentDigest(body)
		}
	}
	writeFixture(t, root, StatePath, encoded(t, map[string]any{
		"version": 1, "template_version": "0.1.0", "template": "workspace", "files": loaded.State.Files,
	}))
	for _, name := range homes {
		writeFixture(t, root, name, []byte("Owner policy for "+name))
	}
	if _, err := ApplyMigration(root, "0.2.0", nil); err != nil {
		t.Fatal(err)
	}
	next, err := LoadState(root)
	if err != nil || len(next.State.Collections) != 0 || len(homes) != 5 {
		t.Fatalf("former homes: %+v %v (%d)", next.State, err, len(homes))
	}
	for _, name := range homes {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || string(body) != "Owner policy for "+name {
			t.Fatalf("orphan changed: %s", name)
		}
		if _, managed := next.State.Files[name]; managed {
			t.Fatalf("unselected former default still managed: %s", name)
		}
		if _, err := os.Lstat(filepath.Join(root, ".wsup/templates/workspace", filepath.FromSlash(name))); !os.IsNotExist(err) {
			t.Fatalf("obsolete catalogue copy remains: %s", name)
		}
	}
}
