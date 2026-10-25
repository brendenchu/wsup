package catalogue

import (
	"bytes"
	"io/fs"
	"slices"
	"strings"
	"testing"
)

func TestNativeBundleOverridesAreExplicitAndSharedBytesUnchanged(t *testing.T) {
	source, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	overrides := map[string][]string{
		"project": {"README.md"},
		"workspace": {
			"README.md", "docs/README.md", "docs/workspace-model.md",
			".agents/skills/project-onboarding/SKILL.md",
			".agents/skills/registry-checks/SKILL.md",
			".agents/skills/ai-guidance-maintenance/SKILL.md",
			".agents/skills/template-lifecycle/SKILL.md",
			".agents/skills/workspace-check/SKILL.md",
		},
	}
	for _, kind := range []string{"workspace", "project"} {
		t.Run(kind, func(t *testing.T) {
			files, err := Bundle(kind)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			err = fs.WalkDir(source, kind, func(name string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				count++
				relative := strings.TrimPrefix(name, kind+"/")
				original, err := fs.ReadFile(source, name)
				if err != nil {
					return err
				}
				body, exists := files[relative]
				if !exists {
					t.Fatalf("missing native asset: %s", relative)
				}
				if slices.Contains(overrides[kind], relative) {
					override, err := fs.ReadFile(native, "native/"+name)
					if err != nil || !bytes.Equal(body, override) {
						t.Fatalf("override not embedded: %s %v", name, err)
					}
					if strings.HasSuffix(relative, "/SKILL.md") {
						before := strings.SplitN(string(original), "---", 3)
						after := strings.SplitN(string(body), "---", 3)
						if len(before) != 3 || len(after) != 3 || before[1] != after[1] {
							t.Fatalf("native skill changed canonical discovery metadata: %s", name)
						}
					}
				} else if !bytes.Equal(body, original) {
					t.Fatalf("shared asset changed: %s", name)
				}
				for _, stale := range []string{"python3 -B .wsup/wsup.py", "src/wsup.py", ".wsup/templates/", "interpreter running the CLI", "CLI's interpreter."} {
					if bytes.Contains(body, []byte(stale)) {
						t.Errorf("stale installation guidance in %s: %s", name, stale)
					}
				}
				return nil
			})
			if err != nil || count != len(files) {
				t.Fatalf("native inventory: source %d native %d %v", count, len(files), err)
			}
			// Each override must correspond to an existing source surface. A new
			// native-only asset requires an intentional inventory/test change.
			overrideCount := 0
			err = fs.WalkDir(native, "native/"+kind, func(name string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				overrideCount++
				if !slices.Contains(overrides[kind], strings.TrimPrefix(name, "native/"+kind+"/")) {
					t.Errorf("unreviewed native override: %s", name)
				}
				return nil
			})
			if err != nil || overrideCount != len(overrides[kind]) {
				t.Fatalf("override inventory: %d %v", overrideCount, err)
			}
		})
	}
	if _, err := Bundle("unknown"); err == nil {
		t.Fatal("invalid bundle kind accepted")
	}
}
