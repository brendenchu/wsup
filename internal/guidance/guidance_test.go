package guidance

import (
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
	put(t, root, "AGENTS.md", "[docs](docs/README.md)\n")
	put(t, root, "docs/README.md", "Docs\n")
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
	put(t, root, "AGENTS.md", "[outside](../outside.md)\n")
	result, err := Inspect(root)
	if err != nil || len(result.Errors) != 1 {
		t.Fatalf("escaping reference: %+v %v", result, err)
	}

	put(t, outside, "external.md", "External\n")
	put(t, root, "docs/README.md", "Docs\n")
	if err := os.Symlink(filepath.Join(outside, "external.md"), filepath.Join(root, "docs/AGENTS.md")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	result, err = Inspect(root)
	if err != nil || len(result.Errors) != 2 || result.Errors[1] != "Guidance symlink escapes owner" {
		t.Fatalf("escaping symlink: %+v %v", result, err)
	}
}

func TestFindingsUseDeterministicLexicalOrder(t *testing.T) {
	root := t.TempDir()
	put(t, root, "AGENTS.md", "[missing](missing.md)\n")
	put(t, root, "research/AGENTS.md", "[missing](missing.md)\n[again](missing.md)\n")
	put(t, root, "journal/AGENTS.md", "[missing](missing.md)\n")
	result, err := Inspect(root)
	expected := strings.Join([]string{
		"AGENTS.md: unresolved/nonlocal reference",
		"journal/AGENTS.md: unresolved/nonlocal reference",
		"research/AGENTS.md: unresolved/nonlocal reference",
		"research/AGENTS.md: unresolved/nonlocal reference",
	}, "\n")
	if err != nil || strings.Join(result.Errors, "\n") != expected {
		t.Fatalf("findings are not lexically ordered with duplicates preserved: %+v %v", result, err)
	}
}

func TestMetadataColonsAndUniversalNewlines(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n", "\r"} {
		root := t.TempDir()
		put(t, root, "AGENTS.md", "Local\n")
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

func TestFolderRulesAreInspectedButProjectRulesAreNot(t *testing.T) {
	root := t.TempDir()
	put(t, root, "AGENTS.md", "Local\n")
	put(t, root, "research/AGENTS.md", "Folder rules\n")
	put(t, root, "apps/app/AGENTS.md", "[missing](missing.md)\n")
	put(t, root, ".hidden/AGENTS.md", "[missing](missing.md)\n")
	put(t, root, "nested/AGENTS.md", "[missing](missing.md)\n")
	put(t, root, "nested/wsup.lock", "{}\n")
	result, err := Inspect(root)
	if err != nil || len(result.Errors) != 0 || strings.Join(result.Files, ",") != "AGENTS.md,research/AGENTS.md" {
		t.Fatalf("folder rule scope: %+v %v", result, err)
	}
	if missing, err := Inspect(t.TempDir()); err != nil || strings.Join(missing.Errors, ",") != "Missing AGENTS.md" {
		t.Fatalf("missing entry point: %+v %v", missing, err)
	}
}

func TestGeneratedSectionMarkersMustPair(t *testing.T) {
	section := func(names ...string) string {
		text := ""
		for _, name := range names {
			text += "<!-- wsup:" + name + " -->\n"
		}
		return text
	}
	for body, want := range map[string]string{
		section("begin layout", "end layout", "begin skills", "end skills"): "",
		section("begin layout"):                                             "wsup section layout is not closed",
		section("end layout"):                                               "wsup section end layout has no matching begin",
		section("begin layout", "begin skills", "end skills"):               "wsup section skills starts inside layout",
		section("begin layout", "end layout", "begin layout", "end layout"): "wsup section layout appears twice",
		section("begin layout", "end skills"):                               "wsup section end skills has no matching begin",
	} {
		root := t.TempDir()
		put(t, root, "AGENTS.md", body)
		result, err := Inspect(root)
		got := strings.Join(result.Errors, ",")
		if err != nil || (want == "" && got != "") || (want != "" && got != "AGENTS.md: "+want) {
			t.Fatalf("%q: %q %v", body, got, err)
		}
	}
}

func TestSymlinkScanRootButNotDescendant(t *testing.T) {
	root := t.TempDir()
	put(t, root, "AGENTS.md", "Local\n")
	put(t, root, "local-skills/example/SKILL.md", "---\nname: example\ndescription: Local\n---\nBody\n")
	put(t, root, "extra/hidden.md", "[missing](missing.md)\n")
	if err := os.MkdirAll(filepath.Join(root, ".agents"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../local-skills", filepath.Join(root, ".agents/skills")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if err := os.Symlink("../extra", filepath.Join(root, "local-skills/descendant")); err != nil {
		t.Fatal(err)
	}
	result, err := Inspect(root)
	if err != nil || len(result.Errors) != 0 ||
		strings.Join(result.Files, ",") != ".agents/skills/example/SKILL.md,AGENTS.md" {
		t.Fatalf("unexpected symlink traversal: %+v %v", result, err)
	}
}

func TestBundledSharedSkillsAndInvalidEdits(t *testing.T) {
	root := t.TempDir()
	starter, err := catalogue.Bundle("workspace")
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range starter {
		put(t, root, name, string(body))
	}
	result, err := Inspect(root)
	if err != nil || len(result.Errors) != 0 || len(result.Skills) != 7 {
		t.Fatalf("bundled starter: %+v %v", result, err)
	}
	name := ".agents/skills/check-workspace/SKILL.md"
	body, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{
		string(body) + "\n[missing](missing.md)\n",
		strings.Replace(string(body), "name: check-workspace", "name: wrong-name", 1),
		string(body) + "\n[outside](../../../../workspace.json)\n",
	} {
		put(t, root, name, text)
		result, err = Inspect(root)
		if err != nil || len(result.Errors) == 0 {
			t.Fatalf("invalid shared skill accepted: %+v %v", result, err)
		}
	}
}

func TestAdapterImportsMustResolve(t *testing.T) {
	root := t.TempDir()
	put(t, root, "AGENTS.md", "Local\n")
	put(t, root, "CLAUDE.md", "@AGENTS.md\n")
	put(t, root, "GEMINI.md", "@missing.md\n")
	result, err := Inspect(root)
	if err != nil || strings.Join(result.Errors, ",") != "GEMINI.md: unresolved @ import" ||
		strings.Join(result.Files, ",") != "AGENTS.md,CLAUDE.md,GEMINI.md" {
		t.Fatalf("adapter imports: %+v %v", result, err)
	}
}
