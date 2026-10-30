package scaffolding

import (
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
)

// olderWorkspace initializes a workspace with the given collections, then
// rewinds it to how a wsup without skill would have left it: no skill file
// and an AGENTS.md skills table without its row, with matching baselines.
// ownerPreface, when set, is owner text added above the generated sections.
func olderWorkspace(t *testing.T, skill, ownerPreface string, collections ...string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workspace")
	selection := catalogue.Selection{Collections: collections, Agents: []string{}, Editors: []string{}}
	if _, err := InitialiseWith(root, "0.1.0", "workspace", selection); err != nil {
		t.Fatal(err)
	}
	loaded := must(LoadLock(root))
	name := ".agents/skills/" + skill + "/SKILL.md"
	if err := os.RemoveAll(filepath.Join(root, filepath.Dir(name))); err != nil {
		t.Fatal(err)
	}
	delete(loaded.Lock.Files, name)
	current, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	lines := []string{}
	for _, line := range strings.SplitAfter(string(current), "\n") {
		if !strings.Contains(line, "](.agents/skills/"+skill+"/SKILL.md)") {
			lines = append(lines, line)
		}
	}
	old := []byte(strings.Join(lines, ""))
	loaded.Lock.Files["AGENTS.md"] = lockDigest(old)
	maps.Copy(loaded.Lock.Blocks, blockDigests("AGENTS.md", old))
	saveLock(t, root, loaded.Lock)
	writeFixture(t, root, "AGENTS.md", append([]byte(ownerPreface), old...))
	return root
}

// TestUpdateDeliversNewCollectionSkills covers a skill added to a collection
// after a workspace was created: update installs it only where an owning
// collection is selected, and merges the skills table around owner text.
func TestUpdateDeliversNewCollectionSkills(t *testing.T) {
	for _, tc := range []struct {
		name, preface string
		agents        string
	}{
		{"unedited AGENTS.md", "", "write"},
		{"owner-edited AGENTS.md", "Owner preface.\n\n", "merge"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := olderWorkspace(t, "review-week", tc.preface, "journal")
			skill := ".agents/skills/review-week/SKILL.md"
			want := []Action{{skill, "write"}, {"AGENTS.md", tc.agents}}
			before := snapshot(t, root)
			plan, err := Update(root, "0.1.0", nil, false)
			if err != nil || !reflect.DeepEqual(plan.Actions, want) || len(plan.Conflicts) != 0 ||
				!reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatalf("preview: %+v %v", plan, err)
			}
			if result, err := Update(root, "0.1.0", nil, true); err != nil || !result.Applied {
				t.Fatalf("apply: %+v %v", result, err)
			}
			agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
			if err != nil || !strings.HasPrefix(string(agents), tc.preface) ||
				!strings.Contains(string(agents), "](.agents/skills/review-week/SKILL.md)") {
				t.Fatalf("skills table not updated: %s", agents)
			}
			if _, err := os.Stat(filepath.Join(root, skill)); err != nil {
				t.Fatal(err)
			}
			if settled, err := Update(root, "0.1.0", nil, false); err != nil || len(settled.Actions) != 0 || len(settled.Conflicts) != 0 {
				t.Fatalf("did not settle: %+v %v", settled, err)
			}
		})
	}
	// Without an owning collection, the skill never appears.
	root := collectionWorkspace(t, "archive")
	if plan, err := Update(root, "0.1.0", nil, false); err != nil || len(plan.Actions) != 0 {
		t.Fatalf("unowned workspace: %+v %v", plan, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".agents/skills/review-week")); !os.IsNotExist(err) {
		t.Fatal("skill installed without an owning collection")
	}
}
