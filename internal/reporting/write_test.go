package reporting

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/brendenchu/wsup/internal/configuration"
	"github.com/brendenchu/wsup/internal/registry"
)

var fixedTime = time.Date(2026, 10, 25, 17, 0, 0, 0, time.UTC)

func reportFixture(t *testing.T) (string, configuration.Result, []registry.Selection, []Result) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "wsup.json"), []byte(`{"synthetic":"input"}`), 0600); err != nil {
		t.Fatal(err)
	}
	config := configuration.Result{
		Settings: configuration.Config{ReportDir: "workbench/.local/wsup/reports"},
		Sources:  map[string]string{"timeout_seconds": "default", "output_limit_bytes": "default", "report_dir": "default"},
	}
	code := 0
	selected := []registry.Selection{{
		Project: registry.Project{ID: "app", Path: "app"}, Name: "test",
		Check: registry.Check{Command: []string{"tool", "SYNTHETIC-SECRET-CANARY"}, Mode: "required"},
	}}
	results := []Result{{Status: "passed", Evidence: &Evidence{
		ExitCode: &code, DurationSeconds: 0.25, OutputBytes: 400000, OutputTruncated: true,
	}}}
	return root, config, selected, results
}

func deterministicWrite(root string, config configuration.Result, selected []registry.Selection, results []Result) (string, error) {
	return write(root, "0.1.0", config, selected, results,
		func() time.Time { return fixedTime }, bytes.NewReader(make([]byte, 6)),
		func(string) gitState { return gitState{Available: false} })
}

func TestPrivateSanitizedReportsAndExactEvidence(t *testing.T) {
	root, config, selected, results := reportFixture(t)
	t.Setenv("SYNTHETIC_PRIVATE_TOKEN", "SYNTHETIC-SECRET-CANARY")
	selected = append(selected, registry.Selection{
		Project: registry.Project{ID: "app", Path: "app"}, Name: "skip", Check: registry.Check{Mode: "report", Command: []string{"unused"}},
	})
	results = append(results, Result{Status: "skipped", Reason: "Unavailable fixture"})
	filename, err := deterministicWrite(root, config, selected, results)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var data report
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatal(err)
	}
	input := sha256.Sum256([]byte(`{"synthetic":"input"}`))
	command, err := CommandDigest(selected[0].Check.Command)
	if err != nil {
		t.Fatal(err)
	}
	if data.Version != 2 || data.WsupVersion != "0.1.0" || data.Runtime != CurrentRuntime() ||
		data.RecordedAt != "2026-10-25T17:00:00Z" || data.Git.Available ||
		data.Inputs["wsup.json"] != hex.EncodeToString(input[:]) ||
		len(data.Inputs) != 1 || len(data.Selection) != 2 || data.Selection[0].CommandSHA256 != command ||
		len(data.Results) != 2 || data.Results[0].Evidence == nil ||
		!reflect.DeepEqual(*data.Results[0].Evidence, *results[0].Evidence) ||
		data.Results[1].Evidence != nil || data.Results[1].Reason != "Unavailable fixture" ||
		!reflect.DeepEqual(data.Summary, map[string]int{"passed": 1, "skipped": 1}) ||
		!reflect.DeepEqual(data.ConfigSources, config.Sources) {
		t.Fatalf("incorrect report evidence: %s", body)
	}
	for _, extension := range []string{".json", ".md"} {
		path := strings.TrimSuffix(filename, ".json") + extension
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"SYNTHETIC-SECRET-CANARY", "SYNTHETIC_PRIVATE_TOKEN", `"stdout"`, `"stderr"`, `"command":`} {
			if bytes.Contains(content, []byte(forbidden)) {
				t.Fatalf("%s persisted %q", extension, forbidden)
			}
		}
		if runtime.GOOS != "windows" {
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("private report mode: %v %v", info, err)
			}
		}
	}
	markdown, err := os.ReadFile(strings.TrimSuffix(filename, ".json") + ".md")
	want := "# Local verification\n\nRecorded: 2026-10-25T17:00:00Z\n\n- app:test: passed\n- app:skip: skipped\n"
	if err != nil || string(markdown) != want {
		t.Fatalf("markdown %q: %v", markdown, err)
	}
}

func TestReportCollisionsNeverOverwriteAndPartialPairsAreExplicit(t *testing.T) {
	for _, extension := range []string{".json", ".md"} {
		t.Run(extension, func(t *testing.T) {
			root, config, selected, results := reportFixture(t)
			directory := filepath.Join(root, "workbench/.local/wsup/reports")
			if err := os.MkdirAll(directory, 0755); err != nil {
				t.Fatal(err)
			}
			existing := filepath.Join(directory, "20261025T170000-000000000000"+extension)
			if err := os.WriteFile(existing, []byte("Keep existing\n"), 0600); err != nil {
				t.Fatal(err)
			}
			path, err := deterministicWrite(root, config, selected, results)
			if path != "" || !errors.Is(err, os.ErrExist) || !strings.Contains(err.Error(), "partial report pair") {
				t.Fatalf("collision was not explicit: %q %v", path, err)
			}
			body, err := os.ReadFile(existing)
			if err != nil || string(body) != "Keep existing\n" {
				t.Fatalf("collision changed existing bytes: %q %v", body, err)
			}
			files, err := os.ReadDir(directory)
			want := 1
			if extension == ".md" {
				want = 2
			}
			if err != nil || len(files) != want {
				t.Fatalf("unexpected partial pair: %v %v", files, err)
			}
		})
	}
}

func TestInvalidEvidenceAndInputFailuresDoNotCreateReports(t *testing.T) {
	for _, result := range []Result{
		{Status: "unknown"}, {Status: "passed"}, {Status: "skipped"},
		{Status: "skipped", Reason: "skip", Evidence: &Evidence{}},
		{Status: "passed", Reason: "skip", Evidence: &Evidence{}},
		{Status: "error", Evidence: &Evidence{Error: "private tool diagnostic"}},
		{Status: "passed", Evidence: &Evidence{DurationSeconds: math.NaN()}},
		{Status: "passed", Evidence: &Evidence{OutputBytes: -1}},
	} {
		root, config, selected, _ := reportFixture(t)
		if path, err := deterministicWrite(root, config, selected, []Result{result}); path != "" || err == nil {
			t.Fatalf("invalid evidence accepted: %+v %q %v", result, path, err)
		}
		if _, err := os.Stat(filepath.Join(root, "workbench")); !os.IsNotExist(err) {
			t.Fatalf("invalid evidence created output: %v", err)
		}
	}
	root, config, selected, results := reportFixture(t)
	if _, err := deterministicWrite(root, config, selected, nil); err == nil {
		t.Fatal("partial result selection accepted")
	}
	if err := os.Mkdir(filepath.Join(root, "wsup.local.json"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := deterministicWrite(root, config, selected, results); err == nil {
		t.Fatal("non-file input accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "workbench")); !os.IsNotExist(err) {
		t.Fatalf("bad input created output: %v", err)
	}
}

func TestReportDirectoryRecheckedAtWriteAndEntropyFailure(t *testing.T) {
	root, config, selected, results := reportFixture(t)
	_, err := write(root, "0.1.0", config, selected, results,
		func() time.Time { return fixedTime }, bytes.NewReader(nil), readGit)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("entropy failure hidden: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "workbench")); !os.IsNotExist(err) {
		t.Fatalf("entropy failure created output: %v", err)
	}
	for _, link := range []string{"workbench", "workbench/.local", "workbench/.local/wsup", "workbench/.local/wsup/reports"} {
		t.Run(link, func(t *testing.T) {
			root, config, selected, results := reportFixture(t)
			outside := t.TempDir()
			// Simulate a change after a successful configuration check.
			if _, err := configuration.ReportDirectory(root, config.Settings.ReportDir); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, link)), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(root, link)); err != nil {
				t.Skipf("symlink creation unavailable: %v", err)
			}
			if _, err := deterministicWrite(root, config, selected, results); err == nil {
				t.Fatal("writer accepted redirected report root")
			}
			files, err := os.ReadDir(outside)
			if err != nil || len(files) != 0 {
				t.Fatalf("external report write: %v %v", files, err)
			}
		})
	}
}

func TestExclusiveReportRefusesSymlinkAndNonDirectoryParent(t *testing.T) {
	root, config, selected, results := reportFixture(t)
	outside := filepath.Join(t.TempDir(), "keep")
	if err := os.WriteFile(outside, []byte("Keep\n"), 0600); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "workbench/.local/wsup/reports")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(directory, "20261025T170000-000000000000.json")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if _, err := deterministicWrite(root, config, selected, results); !errors.Is(err, os.ErrExist) {
		t.Fatalf("symlink collision accepted: %v", err)
	}
	body, err := os.ReadFile(outside)
	if err != nil || string(body) != "Keep\n" {
		t.Fatalf("symlink destination changed: %q %v", body, err)
	}
	other, otherConfig, otherSelected, otherResults := reportFixture(t)
	if err := os.WriteFile(filepath.Join(other, "workbench"), []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := deterministicWrite(other, otherConfig, otherSelected, otherResults); err == nil {
		t.Fatal("non-directory report parent accepted")
	}
}

func TestGitEvidenceUnavailableCleanAndDirty(t *testing.T) {
	t.Run("missing tool", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if state := readGit(t.TempDir()); state.Available || state.Dirty != nil {
			t.Fatalf("unavailable Git claimed clean: %+v", state)
		}
	})
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git unavailable for optional repository evidence test")
	}
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"-c", "user.name=Synthetic", "-c", "user.email=synthetic@example.invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "--allow-empty", "-m", "Synthetic report fixture"},
	} {
		command := exec.Command("git", args...)
		command.Dir = root
		if body, err := command.CombinedOutput(); err != nil {
			t.Fatalf("synthetic git setup: %s %v", body, err)
		}
	}
	state := readGit(root)
	if !state.Available || state.Head == "" || state.Dirty == nil || *state.Dirty {
		t.Fatalf("clean Git evidence: %+v", state)
	}
	if err := os.WriteFile(filepath.Join(root, "untracked"), []byte("Synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	state = readGit(root)
	if !state.Available || state.Dirty == nil || !*state.Dirty {
		t.Fatalf("dirty Git evidence: %+v", state)
	}
}

func TestAllStatusesAndGitUnknownStateRemainExplicit(t *testing.T) {
	root, config, selected, results := reportFixture(t)
	results = nil
	selected = nil
	zero, nonzero, killed := 0, 7, -9
	for _, result := range []Result{
		{Status: "passed", Evidence: &Evidence{ExitCode: &zero}},
		{Status: "failed", Evidence: &Evidence{ExitCode: &nonzero}},
		{Status: "findings", Evidence: &Evidence{ExitCode: &nonzero}},
		{Status: "error", Evidence: &Evidence{Error: "launch_error:2"}},
		{Status: "timeout", Evidence: &Evidence{ExitCode: &killed}},
		{Status: "cancelled", Evidence: &Evidence{ExitCode: &killed}},
		{Status: "skipped", Reason: "cancelled"},
	} {
		selected = append(selected, registry.Selection{
			Project: registry.Project{ID: "app", Path: "app"}, Name: result.Status,
			Check: registry.Check{Mode: "report", Command: []string{"tool"}},
		})
		results = append(results, result)
	}
	filename, err := write(root, "0.1.0", config, selected, results,
		func() time.Time { return fixedTime }, bytes.NewReader(make([]byte, 6)),
		func(string) gitState { return gitState{Available: true, Head: "synthetic-head"} })
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatal(err)
	}
	var git map[string]any
	if err := json.Unmarshal(data["git"], &git); err != nil {
		t.Fatal(err)
	}
	if len(git) != 3 || git["available"] != true || git["head"] != "synthetic-head" || git["dirty"] != nil {
		t.Fatalf("unknown Git dirty state not explicit: %s", data["git"])
	}
	var summary map[string]int
	if err := json.Unmarshal(data["summary"], &summary); err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		if summary[result.Status] != 1 {
			t.Fatalf("missing status summary %q: %v", result.Status, summary)
		}
	}
}

func TestNestedReportSymlinkWithinReportsRemainsValid(t *testing.T) {
	root, config, selected, results := reportFixture(t)
	target := filepath.Join(root, "workbench/.local/wsup/reports/actual")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "workbench/.local/wsup/reports/link")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	config.Settings.ReportDir = "workbench/.local/wsup/reports/link"
	filename, err := deterministicWrite(root, config, selected, results)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filename, filepath.Join("workbench", ".local", "wsup", "reports", "link")) {
		t.Fatalf("report path did not preserve configured spelling: %s", filename)
	}
	if _, err := os.Stat(filepath.Join(target, filepath.Base(filename))); err != nil {
		t.Fatalf("contained symlink report missing: %v", err)
	}
}

func TestPublicWriterCreatesUniqueReportsAndHashesAllInputBytes(t *testing.T) {
	root, config, selected, results := reportFixture(t)
	inputs := map[string][]byte{
		"wsup.json":       []byte("{\"synthetic\":\"registry\"}\n"),
		"wsup.local.json": []byte("{\"version\":1,\"settings\":{}}\n"),
	}
	for name, body := range inputs {
		if err := os.WriteFile(filepath.Join(root, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	first, err := Write(root, "0.1.0", config, selected, results)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Write(root, "0.1.0", config, selected, results)
	if err != nil || first == second {
		t.Fatalf("public writer did not produce distinct successful pairs: %q %q %v", first, second, err)
	}
	body, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	var data report
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Inputs) != len(inputs) {
		t.Fatalf("incomplete input inventory: %v", data.Inputs)
	}
	for name, original := range inputs {
		hash := sha256.Sum256(original)
		if data.Inputs[name] != hex.EncodeToString(hash[:]) {
			t.Fatalf("incorrect hash for %s", name)
		}
		current, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || !bytes.Equal(current, original) {
			t.Fatalf("reporting changed input %s: %v", name, err)
		}
	}
}
