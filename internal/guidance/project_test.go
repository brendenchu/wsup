package guidance

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func projectFile(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestProjectDiscoveryRejectsMalformedAndEscapingCandidates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		path   string
		action string
		want   string
	}{
		{"instruction directory", "apps/api/AGENTS.md", "directory", "regular file"},
		{"root instruction escape", "AGENTS.md", "outside", "escapes owner"},
		{"instruction broken link", "apps/AGENTS.md", "broken", "no such file"},
		{"project instruction sibling", "apps/api/AGENTS.md", "sibling", "escapes owner"},
		{"skill root escape", "apps/api/.agents", "outside", "escapes owner"},
		{"skill root file", ".agents", "file", "directory"},
		{"skills root file", ".agents/skills", "file", "directory"},
		{"skill directory escape", ".agents/skills/test", "outside", "escapes owner"},
		{"skill directory broken", ".agents/skills/test", "broken", "no such file"},
		{"skill file escape", ".agents/skills/test/SKILL.md", "outside", "escapes owner"},
		{"skill file directory", ".agents/skills/test/SKILL.md", "directory", "regular file"},
		{"missing metadata", ".agents/skills/test", "directory", "missing SKILL.md"},
		{"malformed metadata", ".agents/skills/test/SKILL.md", "file", "frontmatter"},
		{"nested workspace", "apps/wsup.json", "file", "select that workspace explicitly"},
		{"nested project workspace", "apps/api/wsup.json", "file", "nested workspace"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			projectFile(t, root, "apps/api/README.md", "Project")
			projectFile(t, root, "apps/web/AGENTS.md", "Sibling")
			path := filepath.Join(root, tc.path)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			switch tc.action {
			case "directory":
				if err := os.Mkdir(path, 0755); err != nil {
					t.Fatal(err)
				}
			case "file":
				projectFile(t, root, tc.path, "malformed")
			default:
				target := filepath.Join(t.TempDir(), "missing")
				if tc.action == "broken" {
					target = filepath.Join(root, "missing")
				}
				if tc.action == "outside" {
					projectFile(t, filepath.Dir(target), "missing", "Outside")
				}
				if tc.action == "sibling" {
					target = filepath.Join(root, "apps/web/AGENTS.md")
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := InspectProject(root, "apps/api"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

func TestProjectDiscoveryHandlesInRootAliasesAndNestedPhysicalAncestors(t *testing.T) {
	root := t.TempDir()
	projectFile(t, root, "real/api/README.md", "Project")
	projectFile(t, root, "real/api/AGENTS.md", "Instructions")
	projectFile(t, root, "real/api/wsup.lock", "{}")
	projectFile(t, root, "AGENTS.md", "Root")
	if err := os.Symlink("real/api", filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	result, err := InspectProject(root, "alias")
	if err != nil || !reflect.DeepEqual(result.Instructions, []string{"AGENTS.md", "alias/AGENTS.md"}) {
		t.Fatalf("in-root alias/project starter: %+v %v", result, err)
	}
	projectFile(t, root, "real/wsup.json", "{}")
	if _, err := InspectProject(root, "alias"); err == nil || !strings.Contains(err.Error(), "nested workspace") {
		t.Fatalf("physical boundary bypass: %v", err)
	}
}

func TestProjectDiscoveryAllowsContainedSkillLinksAndCRLF(t *testing.T) {
	root := t.TempDir()
	projectFile(t, root, "app/README.md", "Project")
	projectFile(t, root, "app/metadata.md", "---\r\nname: test\r\ndescription: Test\r\n---\r\nBody\r\n")
	if err := os.MkdirAll(filepath.Join(root, "app/.agents/skills/test"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink("../../../metadata.md", filepath.Join(root, "app/.agents/skills/test/SKILL.md")); err != nil {
		t.Fatal(err)
	}
	result, err := InspectProject(root, "app")
	if err != nil || len(result.Skills) != 1 || result.Skills[0].Name != "test" || result.Skills[0].Owner != "app" {
		t.Fatalf("contained metadata link: %+v %v", result, err)
	}
	projectFile(t, root, "app/metadata.md", "---\nname: wrong\ndescription: Test\n---\nBody\n")
	if _, err := InspectProject(root, "app"); err == nil {
		t.Fatal("mismatched skill directory accepted")
	}
}

func TestProjectDiscoveryRejectsOutsideAncestorEvenWhenProjectReturnsInside(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	projectFile(t, root, "real/README.md", "Project")
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(outside, "api")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectProject(root, "alias/api"); err == nil || !strings.Contains(err.Error(), "ancestor symlink escapes") {
		t.Fatalf("outside ancestor accepted: %v", err)
	}
}
