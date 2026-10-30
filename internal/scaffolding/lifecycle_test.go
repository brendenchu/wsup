package scaffolding

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
)

// TestAddMatchesInitWithTheSameSelection adds each module kind to a bare
// starter and compares the result with a starter initialized with that
// selection from the start.
func TestAddMatchesInitWithTheSameSelection(t *testing.T) {
	cases := []struct {
		name, starter string
		added         []string
	}{
		{"development collections", "workspace", contractPreset},
		{"collections from every group", "workspace", []string{"data", "journal", "sandbox"}},
		{"every agent", "workspace", []string{"claude", "gemini"}},
		{"editor", "workspace", []string{"vscode"}},
		{"one of each kind", "workspace", []string{"claude", "research", "vscode"}},
		{"project agents", "project", []string{"claude", "gemini"}},
	}
	for _, id := range catalogue.CollectionNames() {
		cases = append(cases, struct {
			name, starter string
			added         []string
		}{"collection " + id, "workspace", []string{id}})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bare := catalogue.Selection{Agents: []string{}, Editors: []string{}}
			added := filepath.Join(t.TempDir(), tc.starter)
			if _, err := InitialiseWith(added, "0.1.0", tc.starter, bare); err != nil {
				t.Fatal(err)
			}
			if result, err := AddModules(added, tc.added, true); err != nil || !result.Applied {
				t.Fatalf("add: %+v %v", result, err)
			}
			want := bare
			for _, id := range tc.added {
				switch kind, _ := catalogue.KindOf(id); kind {
				case catalogue.KindCollection:
					want.Collections = append(want.Collections, id)
				case catalogue.KindAgent:
					want.Agents = append(want.Agents, id)
				case catalogue.KindEditor:
					want.Editors = append(want.Editors, id)
				}
			}
			initialized := filepath.Join(t.TempDir(), tc.starter)
			if _, err := InitialiseWith(initialized, "0.1.0", tc.starter, want); err != nil {
				t.Fatal(err)
			}
			got, expected := snapshot(t, added), snapshot(t, initialized)
			for name := range mergedNames(got, expected) {
				if got[name] != expected[name] {
					t.Errorf("%s differs after add", name)
				}
			}
		})
	}
}

func mergedNames(a, b map[string]string) map[string]bool {
	names := map[string]bool{}
	for name := range a {
		names[name] = true
	}
	for name := range b {
		names[name] = true
	}
	return names
}

// TestEditedSkillsSurviveUpdatesAndAdditions covers a locally edited skill:
// update reports it and writes nothing until it is kept, and adding another
// collection that owns the same skill leaves it alone.
func TestEditedSkillsSurviveUpdatesAndAdditions(t *testing.T) {
	root := collectionWorkspace(t, "tools")
	skill := ".agents/skills/make-change/SKILL.md"
	edited := []byte("---\nname: make-change\ndescription: Owner's version.\n---\n\nOwner steps.\n")
	writeFixture(t, root, skill, edited)
	baseline := must(LoadLock(root)).Lock.Files[skill]

	before := snapshot(t, root)
	plan, err := Update(root, "0.1.0", nil, false)
	if err != nil || !reflect.DeepEqual(plan.Conflicts, []string{skill}) {
		t.Fatalf("edited skill not reported: %+v %v", plan, err)
	}
	if _, err := Update(root, "0.1.0", nil, true); err == nil || !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("update overwrote an edited skill")
	}
	if result, err := Update(root, "0.1.0", []string{skill}, true); err != nil || !result.Applied {
		t.Fatalf("keep: %+v %v", result, err)
	}

	// apps owns make-change too; adding it must not reinstall the skill.
	if result, err := AddModules(root, []string{"apps"}, true); err != nil || !result.Applied {
		t.Fatalf("add apps: %+v %v", result, err)
	}
	body, err := os.ReadFile(filepath.Join(root, skill))
	if err != nil || !bytes.Equal(body, edited) {
		t.Fatal("adding a co-owning collection replaced the edited skill")
	}
	if got := must(LoadLock(root)).Lock.Files[skill]; got != baseline {
		t.Fatalf("baseline changed: %s, want %s", got, baseline)
	}
}

// TestMigratedStartersMatchTheContract migrates the v0.2.0 fixtures and runs
// the generated-output matrix checks on the result: a migrated starter is
// indistinguishable from a fresh v0.3.0 one with the same selection.
func TestMigratedStartersMatchTheContract(t *testing.T) {
	allAgents := []string{"claude", "gemini"}
	for _, entry := range []matrixEntry{
		{"workspace", "workspace", contractPreset, allAgents, []string{"vscode"}},
		{"project", "project", nil, allAgents, nil},
	} {
		t.Run(entry.name, func(t *testing.T) {
			root := legacyFixture(t, entry.starter)
			if result, err := Migrate(root, "0.3.0", MigrationOptions{}, true); err != nil || !result.Applied {
				t.Fatalf("migrate: %+v %v", result, err)
			}
			// The backup is ignored local content, not part of the starter.
			if err := os.RemoveAll(filepath.Join(root, "workbench", ".local")); err != nil {
				t.Fatal(err)
			}
			files, links, skills, ignore := entry.expected()
			checkInventory(t, root, files, links)
			checkSkills(t, root, skills)
			checkSelectionRecorded(t, root, entry)
			checkMarkdownLinks(t, root)
			checkIgnoreBlock(t, root, ignore)
			if entry.starter == "workspace" {
				checkLayoutBlock(t, root, entry.collections)
			}
			fresh := filepath.Join(t.TempDir(), entry.starter)
			selection := catalogue.Selection{Collections: entry.collections, Agents: entry.agents, Editors: entry.editors}
			if _, err := InitialiseWith(fresh, "0.3.0", entry.starter, selection); err != nil {
				t.Fatal(err)
			}
			migrated, want := snapshot(t, root), snapshot(t, fresh)
			for name := range mergedNames(migrated, want) {
				if migrated[name] != want[name] {
					t.Errorf("%s differs from a fresh starter", name)
				}
			}
		})
	}
}
