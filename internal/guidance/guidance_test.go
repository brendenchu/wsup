package guidance

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
)

func put(t *testing.T, root, name, body string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestSkillMetadataAndLinks(t *testing.T) {
	root := t.TempDir()
	put(t, root, "AI.md", "[rules](guidance/rules/index.md)\n")
	put(t, root, "guidance/rules/index.md", "Local rules\n")
	body := "---\nname: example\ndescription: Synthetic\n---\nBody\n"
	name := ".agents/skills/example/SKILL.md"
	put(t, root, name, body)
	result, err := Inspect(root)
	if err != nil || len(result.Errors) != 0 || len(result.Skills) != 1 {
		t.Fatalf("valid guidance: %+v %v", result, err)
	}
	for _, malformed := range []string{
		"Body", strings.Replace(body, "name: example", "name: wrong", 1),
		strings.Replace(body, "description: Synthetic", "description: ", 1),
		strings.Replace(body, "Body", "", 1),
	} {
		put(t, root, name, malformed)
		result, err := Inspect(root)
		if err != nil || len(result.Errors) != 1 || len(result.Skills) != 0 {
			t.Fatalf("malformed metadata: %+v %v", result, err)
		}
	}
	put(t, root, name, body+"\n[missing](missing.md)\n")
	result, err = Inspect(root)
	if err != nil || len(result.Errors) != 1 || !strings.Contains(result.Errors[0], "unresolved/nonlocal") {
		t.Fatalf("broken link: %+v %v", result, err)
	}
}

func TestLinksAndSymlinksCannotEscapeOwner(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	put(t, root, "AI.md", "[outside](../outside.md)\n")
	put(t, root, "guidance/rules/index.md", "Rules\n")
	result, err := Inspect(root)
	if err != nil || len(result.Errors) != 1 {
		t.Fatalf("escaping reference: %+v %v", result, err)
	}

	put(t, outside, "external.md", "External\n")
	if err := os.Symlink(filepath.Join(outside, "external.md"), filepath.Join(root, "guidance/external.md")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	result, err = Inspect(root)
	if err != nil || len(result.Errors) != 2 || result.Errors[1] != "Guidance symlink escapes owner" {
		t.Fatalf("escaping symlink: %+v %v", result, err)
	}
}

func TestFindingsUseDeterministicLexicalOrder(t *testing.T) {
	root := t.TempDir()
	put(t, root, "AI.md", "[missing](missing.md)\n")
	put(t, root, "guidance/rules/index.md", "Rules\n")
	put(t, root, "guidance/z.md", "[missing](missing.md)\n[again](missing.md)\n")
	put(t, root, "guidance/a/nested.md", "[missing](missing.md)\n")
	result, err := Inspect(root)
	expected := strings.Join([]string{
		"AI.md: unresolved/nonlocal reference",
		"guidance/a/nested.md: unresolved/nonlocal reference",
		"guidance/z.md: unresolved/nonlocal reference",
		"guidance/z.md: unresolved/nonlocal reference",
	}, "\n")
	if err != nil || strings.Join(result.Errors, "\n") != expected {
		t.Fatalf("findings are not lexically ordered with duplicates preserved: %+v %v", result, err)
	}
}

func TestMetadataColonsAndUniversalNewlines(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n", "\r"} {
		root := t.TempDir()
		put(t, root, "AI.md", "Local\n")
		put(t, root, "guidance/rules/index.md", "Rules\n")
		body := strings.Join([]string{
			"---", "name: example", "description: Example: with colon", "---", "Body", "",
		}, newline)
		put(t, root, ".agents/skills/example/SKILL.md", body)
		result, err := Inspect(root)
		if err != nil || len(result.Errors) != 0 || len(result.Skills) != 1 {
			t.Fatalf("newline %q: %+v %v", newline, result, err)
		}
	}
}

func TestSkillsInGuidanceAreInspected(t *testing.T) {
	root := t.TempDir()
	put(t, root, "AI.md", "Local\n")
	put(t, root, "guidance/rules/index.md", "Rules\n")
	body := "---\nname: example\ndescription: Local\n---\nBody\n"
	put(t, root, "guidance/example/SKILL.md", body)
	result, err := Inspect(root)
	if err != nil || len(result.Errors) != 0 || len(result.Skills) != 1 {
		t.Fatalf("guidance skill not inspected: %+v %v", result, err)
	}
}

func TestSymlinkScanRootButNotDescendant(t *testing.T) {
	root := t.TempDir()
	put(t, root, "AI.md", "Local\n")
	put(t, root, "local-guidance/rules/index.md", "Rules\n")
	put(t, root, "extra/hidden.md", "[missing](missing.md)\n")
	if err := os.Symlink("local-guidance", filepath.Join(root, "guidance")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if err := os.Symlink("../extra", filepath.Join(root, "local-guidance/descendant")); err != nil {
		t.Fatal(err)
	}
	result, err := Inspect(root)
	if err != nil || len(result.Errors) != 0 ||
		strings.Join(result.Files, ",") != "AI.md,guidance/rules/index.md" {
		t.Fatalf("unexpected symlink traversal: %+v %v", result, err)
	}
}

func TestBundledSharedSkillsAndInvalidEdits(t *testing.T) {
	root := t.TempDir()
	assets := catalogue.Open()
	starter, err := fs.Sub(assets, "workspace")
	if err != nil {
		t.Fatal(err)
	}
	err = fs.WalkDir(starter, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := fs.ReadFile(starter, name)
		if err != nil {
			return err
		}
		put(t, root, name, string(body))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Inspect(root)
	if err != nil || len(result.Errors) != 0 || len(result.Skills) != 12 {
		t.Fatalf("bundled starter: %+v %v", result, err)
	}
	name := ".agents/skills/workspace-check/SKILL.md"
	body, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{
		string(body) + "\n[missing](missing.md)\n",
		strings.Replace(string(body), "name: workspace-check", "name: wrong-name", 1),
		string(body) + "\n[outside](../../../../workspace.json)\n",
	} {
		put(t, root, name, text)
		result, err = Inspect(root)
		if err != nil || len(result.Errors) == 0 {
			t.Fatalf("invalid shared skill accepted: %+v %v", result, err)
		}
	}
}
