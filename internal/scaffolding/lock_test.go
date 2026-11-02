package scaffolding

import (
	"reflect"
	"strings"
	"testing"

	"github.com/wavykit/wsup/internal/catalogue"
)

// mustSelection returns a lock's installed modules as a selection.
func mustSelection(t *testing.T, lock Lock) catalogue.Selection {
	t.Helper()
	selection, err := lock.Selection()
	if err != nil {
		t.Fatal(err)
	}
	return selection
}

// contractLock is the wsup.lock example from the v0.3.0 contract with its
// elided digests filled in.
var contractLock = strings.NewReplacer("…", strings.Repeat("a", 64)).Replace(`{
  "version": 1,
  "wsup_version": "0.3.0",
  "starter": "workspace",
  "modules": {
    "workspace-base": "1.0.0",
    "apps": "1.0.0", "infra": "1.0.0", "issues": "1.0.0",
    "packages": "1.0.0", "tools": "1.0.0",
    "claude": "1.0.0", "gemini": "1.0.0", "vscode": "1.0.0"
  },
  "files": {
    "CLAUDE.md": "sha256:…",
    ".agents/skills/make-change/SKILL.md": "sha256:…",
    "issues/TEMPLATE.md": "sha256:…"
  },
  "blocks": {
    "AGENTS.md#layout": "sha256:…",
    "AGENTS.md#skills": "sha256:…"
  },
  "links": { ".claude/skills": "../.agents/skills" }
}`)

func TestLockRoundTripsTheContractExample(t *testing.T) {
	lock, err := ParseLock([]byte(contractLock))
	if err != nil {
		t.Fatal(err)
	}
	if lock.Modules["vscode"] != "1.0.0" || len(lock.Blocks) != 2 || lock.Links[".claude/skills"] != "../.agents/skills" {
		t.Fatalf("lock: %+v", lock)
	}
	encoded, err := EncodeLock(lock)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ParseLock(encoded)
	if err != nil || !reflect.DeepEqual(decoded, lock) {
		t.Fatalf("round trip: %+v %v", decoded, err)
	}
	if again, err := EncodeLock(decoded); err != nil || string(again) != string(encoded) || !strings.HasSuffix(string(encoded), "}\n") {
		t.Fatalf("encoding is not stable: %v\n%s", err, encoded)
	}
}

func TestEncodeLockOmitsEmptyBlocksAndLinks(t *testing.T) {
	encoded, err := EncodeLock(Lock{Version: LockVersion, WsupVersion: "0.3.0", Starter: "project",
		Modules: map[string]string{"project-base": "1.0.0"}, Files: map[string]string{"AGENTS.md": lockDigest([]byte("x"))}})
	if err != nil {
		t.Fatal(err)
	}
	if text := string(encoded); strings.Contains(text, "blocks") || strings.Contains(text, "links") {
		t.Fatalf("empty maps written:\n%s", text)
	}
}

func TestParseLockRejectsInvalidLocks(t *testing.T) {
	digest := `"sha256:` + strings.Repeat("a", 64) + `"`
	lock := func(fields string) string {
		return `{"version":1,"wsup_version":"0.3.0","starter":"workspace","modules":{"workspace-base":"1.0.0"},"files":{"README.md":` + digest + `}` + fields + `}`
	}
	if _, err := ParseLock([]byte(lock(""))); err != nil {
		t.Fatalf("base lock rejected: %v", err)
	}
	for name, body := range map[string]string{
		"version 2":         strings.Replace(lock(""), `"version":1`, `"version":2`, 1),
		"unknown field":     lock(`,"state":{}`),
		"trailing JSON":     lock("") + "{}",
		"no wsup version":   strings.Replace(lock(""), `"0.3.0"`, `""`, 1),
		"bad starter":       strings.Replace(lock(""), `"starter":"workspace"`, `"starter":"repo"`, 1),
		"missing base":      strings.Replace(lock(""), `"workspace-base"`, `"apps"`, 1),
		"bad module id":     strings.Replace(lock(""), `"workspace-base":"1.0.0"`, `"workspace-base":"1.0.0","Apps":"1.0.0"`, 1),
		"bad version":       strings.Replace(lock(""), `"workspace-base":"1.0.0"`, `"workspace-base":"v1"`, 1),
		"no files":          `{"version":1,"wsup_version":"0.3.0","starter":"workspace","modules":{"workspace-base":"1.0.0"}}`,
		"bare hex digest":   strings.Replace(lock(""), `"sha256:`, `"`, 1),
		"escaping file":     strings.Replace(lock(""), `"README.md"`, `"../README.md"`, 1),
		"lock lists itself": strings.Replace(lock(""), `"README.md"`, `"wsup.lock"`, 1),
		"block without #":   lock(`,"blocks":{"AGENTS.md":` + digest + `}`),
		"block bad name":    lock(`,"blocks":{"AGENTS.md#Layout":` + digest + `}`),
		"absolute link":     lock(`,"links":{".claude/skills":"/tmp/skills"}`),
	} {
		if _, err := ParseLock([]byte(body)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
