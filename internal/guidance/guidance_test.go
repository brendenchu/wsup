package guidance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestSkillMetadataLinksAndCoexistence(t *testing.T) {
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
	put(t, root, name, body)
	put(t, root, ".ai/skills/example/SKILL.md", body)
	result, err = Inspect(root)
	if err != nil || len(result.Errors) != 1 || !strings.Contains(result.Errors[0], "coexist") {
		t.Fatalf("legacy coexistence: %+v %v", result, err)
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
