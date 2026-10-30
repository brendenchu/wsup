package configuration

import (
	"os"
	"path/filepath"
	"testing"
)

func noEnv(string) (string, bool) { return "", false }

func write(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

// shared returns a minimal wsup.json with the given settings object.
func shared(settings string) string {
	return `{"version":3,"name":"W","projects":[],"settings":` + settings + `}`
}

func TestPrecedenceAndSources(t *testing.T) {
	root := t.TempDir()
	write(t, root, "wsup.json", shared(`{"timeout_seconds":20}`))
	write(t, root, "wsup.local.json", `{"version":1,"settings":{"timeout_seconds":30}}`)
	environment := func(name string) (string, bool) {
		if name == "WSUP_TIMEOUT_SECONDS" {
			return " 40 ", true
		}
		return "", false
	}
	timeout := 50
	write(t, root, "wsup.local.json", `{"version":1}`)
	if result, err := Load(root, nil, noEnv); err != nil || result.Settings.TimeoutSeconds != 20 || result.Sources["timeout_seconds"] != "wsup.json" {
		t.Fatalf("wsup.json settings: %+v %v", result, err)
	}
	write(t, root, "wsup.local.json", `{"version":1,"settings":{"timeout_seconds":30}}`)
	for _, tc := range []struct {
		override *int
		lookup   func(string) (string, bool)
		timeout  int
		source   string
	}{
		{nil, noEnv, 30, "wsup.local.json"},
		{nil, environment, 40, "WSUP_TIMEOUT_SECONDS"},
		{&timeout, environment, 50, "CLI"},
	} {
		result, err := Load(root, tc.override, tc.lookup)
		if err != nil {
			t.Fatal(err)
		}
		if result.Settings.TimeoutSeconds != tc.timeout || result.Sources["timeout_seconds"] != tc.source ||
			result.Settings.OutputLimitBytes != 65536 || result.Sources["report_dir"] != "default" {
			t.Fatalf("unexpected config: %+v", result)
		}
	}
}

func TestInvalidSharedSettings(t *testing.T) {
	for _, body := range []string{
		`{"version":1,"settings":{"timeout_seconds":20}}`,
		shared(`{"unknown":1}`), shared(`{"timeout_seconds":0}`),
	} {
		root := t.TempDir()
		write(t, root, "wsup.json", body)
		if _, err := Load(root, nil, noEnv); err == nil {
			t.Fatalf("invalid wsup.json accepted: %s", body)
		}
	}
}

func TestInvalidConfiguration(t *testing.T) {
	for _, body := range []string{
		`{"version":true}`, `{"version":1.0}`, `{"version":1,"typo":true}`,
		`{"version":1,"settings":null}`, `{"version":1,"settings":{"unknown":1}}`,
		`{"version":1,"settings":{"timeout_seconds":null}}`,
		`{"version":1,"settings":{"timeout_seconds":false}}`,
		`{"version":1,"settings":{"timeout_seconds":1.0}}`,
		`{"version":1,"settings":{"timeout_seconds":0}}`,
		`{"version":1,"settings":{"timeout_seconds":86401}}`,
		`{"version":1,"settings":{"output_limit_bytes":10485761}}`,
		`{"version":1,"settings":{"report_dir":null}}`,
		`{"version":1,"settings":{"report_dir":".wsup/reports"}}`,
		`{"version":1,"settings":{"report_dir":"../reports"}}`,
	} {
		t.Run(body, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "wsup.local.json", body)
			if _, err := Load(root, nil, noEnv); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}

func TestBoundsAndSymlinkedReports(t *testing.T) {
	root := t.TempDir()
	write(t, root, "wsup.json", shared(`{"timeout_seconds":86400,"output_limit_bytes":10485760,"report_dir":"workbench/.local/wsup/reports/custom"}`))
	if _, err := Load(root, nil, noEnv); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "workbench/.local/wsup"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "workbench/.local/wsup/reports")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if _, err := Load(root, nil, noEnv); err == nil {
		t.Fatalf("symlinked reports: %v", err)
	}
}

func TestReportContainmentAndSymlinkDefenses(t *testing.T) {
	for _, path := range []string{".work/quality", ".wsup/reports", "workbench/.local/notes", "../reports", "workbench/.local/wsup/reports/../../escape"} {
		t.Run(path, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "wsup.json", shared(`{"report_dir":"`+path+`"}`))
			if _, err := Load(root, nil, noEnv); err == nil {
				t.Fatalf("accepted report path %q", path)
			}
		})
	}
	root := t.TempDir()
	write(t, root, "wsup.json", shared(`{"report_dir":"workbench/.local/wsup/reports/custom"}`))
	config, err := Load(root, nil, noEnv)
	if err != nil || config.Settings.ReportDir != "workbench/.local/wsup/reports/custom" {
		t.Fatalf("nested report path: %+v %v", config, err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "workbench")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if _, err := Load(root, nil, noEnv); err == nil {
		t.Fatal("accepted symlinked workbench root")
	}
}
