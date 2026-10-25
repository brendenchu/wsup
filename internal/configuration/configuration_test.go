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

func TestPrecedenceAndSources(t *testing.T) {
	root := t.TempDir()
	write(t, root, "wsup.json", `{"version":1,"settings":{"timeout_seconds":20}}`)
	write(t, root, "wsup.local.json", `{"version":1,"settings":{"timeout_seconds":30}}`)
	environment := func(name string) (string, bool) {
		if name == "WSUP_TIMEOUT_SECONDS" {
			return " 40 ", true
		}
		return "", false
	}
	timeout := 50
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
		`{"version":1,"settings":{"report_dir":".wsup/templates"}}`,
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
	write(t, root, "wsup.json", `{"version":1,"settings":{"timeout_seconds":86400,"output_limit_bytes":10485760,"report_dir":".wsup/reports/custom"}}`)
	if _, err := Load(root, nil, noEnv); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".wsup"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, ".wsup/reports")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if _, err := Load(root, nil, noEnv); err == nil {
		t.Fatalf("symlinked reports: %v", err)
	}
}
