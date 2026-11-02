package projectview

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/wavykit/wsup/internal/guidance"
)

func write(t *testing.T, root, name, body string) {
	t.Helper()
	target := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workspace")
	write(t, root, "wsup.json", `{"version":3,"name":"Fixture","projects":[
{"id":"api","path":"apps/api","kind":"app","repository":"independent","owner":"API team","purpose":"Example API",
"git":"https://example.invalid/private","dependencies":["shared"],"tasks":{
"serve":["missing-tool","$TOKEN","*.go"],
"test":{"command":["sh","-c","touch EXECUTED"],"check":true,"cwd":"src","mode":"report","enabled":false,"reason":"Manual","finding_exit_codes":[]}}},
{"id":"web","path":"apps/web","kind":"app","repository":"workspace","owner":"Web team","purpose":"Web","dependencies":["api","shared"]},
{"id":"shared","path":"packages/shared","kind":"package","repository":"independent","owner":"Libraries","purpose":"Shared"},
{"id":"infra","path":"infra/local","kind":"infra","repository":"workspace","owner":"Ops","purpose":"Local infrastructure"},
{"id":"study","path":"research/study","kind":"tool","repository":"workspace","owner":"Research","purpose":"Study"}
]}`)
	for _, path := range []string{"apps/api", "apps/web", "packages/shared", "infra/local", "research/study"} {
		write(t, root, path+"/README.md", "Project\n")
	}
	if err := os.MkdirAll(filepath.Join(root, "apps/api/src"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"AGENTS.md", "apps/AGENTS.md", "apps/api/AGENTS.md"} {
		// Metadata presence must not require reading content or following links.
		write(t, root, path, "\xff\n[do not read](https://example.invalid)\n")
	}
	for _, owner := range []string{".", "apps/api"} {
		write(t, root, filepath.Join(owner, ".agents/skills/build/SKILL.md"), "---\nname: build\ndescription: Build\n---\nDo not execute.\n")
	}
	write(t, root, "apps/web/.agents/skills/bad/SKILL.md", "\xff")
	write(t, root, "apps/api/wsup.lock", "{}")
	write(t, root, ".agents/skills/unrelated.md", "\xff")
	write(t, filepath.Dir(root), "AGENTS.md", "Parent must not apply")
	return root
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(name)
			if err != nil {
				return err
			}
			result[relative] = "link:" + target
		} else if !entry.IsDir() {
			body, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			result[relative] = string(body)
		} else {
			result[relative] = "directory"
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestInspectAppendixBProjectIsBoundedAndReadOnly(t *testing.T) {
	root := fixture(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	write(t, root, ".agents/skills/build/SKILL.md", "---\nname: build\ndescription: Build\n---\n[Remote]("+server.URL+")\n")
	t.Setenv("PATH", "")
	before := snapshot(t, filepath.Dir(root))
	result, err := Inspect(root, "api")
	if err != nil {
		t.Fatal(err)
	}
	if result.Project != (Project{ID: "api", Path: "apps/api", Kind: "app", Repository: "independent", Owner: "API team", Purpose: "Example API"}) ||
		result.Workspace.Name != "Fixture" {
		t.Fatalf("project: %+v", result)
	}
	if !reflect.DeepEqual(result.Dependencies, []string{"shared"}) || !reflect.DeepEqual(result.Dependents, []string{"web"}) ||
		!reflect.DeepEqual(result.Instructions, []string{"AGENTS.md", "apps/AGENTS.md", "apps/api/AGENTS.md"}) ||
		!reflect.DeepEqual(result.Skills, []guidance.OwnedSkill{
			{Name: "build", Path: ".agents/skills/build/SKILL.md", Owner: "."},
			{Name: "build", Path: "apps/api/.agents/skills/build/SKILL.md", Owner: "apps/api"},
		}) {
		t.Fatalf("relationships/guidance: %+v", result)
	}
	if len(result.Tasks) != 2 || result.Tasks[0].Name != "serve" || result.Tasks[0].CheckPolicy != nil ||
		!reflect.DeepEqual(result.Tasks[0].Command, []string{"missing-tool", "$TOKEN", "*.go"}) {
		t.Fatalf("literal ordered tasks: %+v", result.Tasks)
	}
	policy := result.Tasks[1].CheckPolicy
	if result.Tasks[1].Name != "test" || result.Tasks[1].Cwd != "src" || !result.Tasks[1].Check ||
		policy == nil || policy.Mode != "report" || policy.Enabled || policy.Reason != "Manual" ||
		policy.TimeoutSeconds != nil || policy.FindingExitCodes == nil || len(policy.FindingExitCodes) != 0 {
		t.Fatalf("check policy: %+v", result.Tasks[1])
	}
	body, err := json.Marshal(result)
	if err != nil || strings.Contains(string(body), "example.invalid") || strings.Contains(string(body), root) {
		t.Fatalf("unexpected disclosure: %s %v", body, err)
	}
	if !reflect.DeepEqual(before, snapshot(t, filepath.Dir(root))) {
		t.Fatal("inspection wrote files, executed marker task or changed sibling/parent data")
	}
	if requests.Load() != 0 {
		t.Fatal("inspection fetched a skill link")
	}
	shared, err := Inspect(root, "shared")
	if err != nil || !reflect.DeepEqual(shared.Dependents, []string{"api", "web"}) {
		t.Fatalf("incoming registry order: %+v %v", shared, err)
	}
}

func TestInspectEmptyCollectionsAndDefaultCheckPolicy(t *testing.T) {
	root := t.TempDir()
	write(t, root, "README.md", "Root project")
	write(t, root, "wsup.json", `{"version":3,"name":"Root","projects":[{"id":"root","path":".","kind":"tool","repository":"workspace","owner":"O","purpose":"P"}]}`)
	result, err := Inspect(root, "root")
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(result)
	if err != nil || strings.Contains(string(body), "null") {
		t.Fatalf("empty arrays: %s %v", body, err)
	}
	write(t, root, "wsup.json", `{"version":3,"name":"Root","projects":[{"id":"root","path":".","kind":"tool","repository":"workspace","owner":"O","purpose":"P","tasks":{"test":{"command":["test"],"check":true,"timeout_seconds":10}}}]}`)
	result, err = Inspect(root, "root")
	if err != nil {
		t.Fatal(err)
	}
	policy := result.Tasks[0].CheckPolicy
	if policy.Mode != "required" || !policy.Enabled || policy.TimeoutSeconds == nil || *policy.TimeoutSeconds != 10 ||
		!reflect.DeepEqual(policy.FindingExitCodes, []int{1}) {
		t.Fatalf("default policy: %+v", policy)
	}
}

func TestInspectRejectsUnknownAndInvalidRegistryPaths(t *testing.T) {
	root := fixture(t)
	if _, err := Inspect(root, "missing"); err == nil || err.Error() != "Unknown project: missing" {
		t.Fatalf("unknown ID: %v", err)
	}
	write(t, root, "wsup.json", "{")
	if _, err := Inspect(root, "api"); err == nil {
		t.Fatal("malformed registry accepted")
	}
	root = fixture(t)
	if err := os.RemoveAll(filepath.Join(root, "apps/web")); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(root, "api"); err == nil {
		t.Fatal("unrelated invalid registered project must still block inspection")
	}
	root = fixture(t)
	if err := os.RemoveAll(filepath.Join(root, "apps/api")); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	write(t, outside, "README.md", "Outside")
	if err := os.Symlink(outside, filepath.Join(root, "apps/api")); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(root, "api"); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("outside project accepted: %v", err)
	}
}
