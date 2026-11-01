package main

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureArchive(t *testing.T, mutate func([]*tar.Header) []*tar.Header) string {
	t.Helper()
	var entries []*tar.Header
	for name := range inventory() {
		mode := int64(0644)
		if name == "wsup" {
			mode = 0755
		}
		entries = append(entries, &tar.Header{Name: "bundle/" + name, Mode: mode, Size: 1, Typeflag: tar.TypeReg})
	}
	if mutate != nil {
		entries = mutate(entries)
	}
	name := filepath.Join(t.TempDir(), "archive.tar.gz")
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, h := range entries {
		if err = tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err = tw.Write([]byte("x")); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, close := range []func() error{tw.Close, gz.Close, f.Close} {
		if err = close(); err != nil {
			t.Fatal(err)
		}
	}
	return name
}
func TestArchiveInventoryAndExtraction(t *testing.T) {
	archive := fixtureArchive(t, nil)
	dest := t.TempDir()
	if err := extractArchive(archive, dest, "bundle"); err != nil {
		t.Fatal(err)
	}
	for name := range inventory() {
		if body, err := os.ReadFile(filepath.Join(dest, name)); err != nil || string(body) != "x" {
			t.Fatalf("%s: %q %v", name, body, err)
		}
	}
}
func TestArchiveRejectsUnexpectedAndUnsafeEntries(t *testing.T) {
	tests := map[string]func([]*tar.Header) []*tar.Header{
		"missing notice": func(h []*tar.Header) []*tar.Header { return h[1:] },
		"AppleDouble": func(h []*tar.Header) []*tar.Header {
			return append(h, &tar.Header{Name: "bundle/._wsup", Mode: 0644, Size: 1, Typeflag: tar.TypeReg})
		},
		"path escape":   func(h []*tar.Header) []*tar.Header { h[0].Name = "bundle/../outside"; return h },
		"absolute path": func(h []*tar.Header) []*tar.Header { h[0].Name = "/outside"; return h },
		"symlink": func(h []*tar.Header) []*tar.Header {
			h[0].Typeflag = tar.TypeSymlink
			h[0].Size = 0
			h[0].Linkname = "/outside"
			return h
		},
		"duplicate": func(h []*tar.Header) []*tar.Header { return append(h, h[0]) },
		"extra directory": func(h []*tar.Header) []*tar.Header {
			return append(h, &tar.Header{Name: "bundle/private/", Typeflag: tar.TypeDir, Mode: 0755})
		},
		"nonexecutable": func(h []*tar.Header) []*tar.Header {
			for _, v := range h {
				if v.Name == "bundle/wsup" {
					v.Mode = 0644
				}
			}
			return h
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			if err := extractArchive(fixtureArchive(t, mutate), t.TempDir(), "bundle"); err == nil {
				t.Fatal("unsafe/incomplete archive accepted")
			}
		})
	}
}
func TestVerifyRejectsTamperedChecksumBeforeExecution(t *testing.T) {
	dist := t.TempDir()
	if err := saveInputs(dist, inputs{Version: "0.3.0", ProductVersion: "0.3.0"}); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"wsup_0.3.0_darwin_arm64.tar.gz": "tampered", "SHA256SUMS": strings.Repeat("0", 64) + "  wsup_0.3.0_darwin_arm64.tar.gz\n"} {
		if err := os.WriteFile(filepath.Join(dist, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := verifyRelease(dist); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("tampered archive: %v", err)
	}
}

func TestFingerprintTracksDeletedSource(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := command(nil, "git", "init"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("tracked", []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := command(nil, "git", "add", "tracked"); err != nil {
		t.Fatal(err)
	}
	before, err := fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove("tracked"); err != nil {
		t.Fatal(err)
	}
	after, err := fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("deleting a tracked file did not change its source fingerprint")
	}
}
