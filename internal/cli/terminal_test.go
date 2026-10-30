//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDevNullIsNotATerminal(t *testing.T) {
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	if isTerminal(devNull) {
		t.Fatal("/dev/null treated as a terminal")
	}
	// The executable with /dev/null for stdin and stdout uses the defaults.
	binary := filepath.Join(t.TempDir(), "wsup")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/wsup")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %s %v", output, err)
	}
	root := filepath.Join(t.TempDir(), "workspace")
	var stderr strings.Builder
	init := exec.Command(binary, "init", root)
	init.Stdin, init.Stdout, init.Stderr = devNull, devNull, &stderr
	if err := init.Run(); err != nil {
		t.Fatalf("init with /dev/null streams: %v %s", err, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "wsup.lock")); err != nil {
		t.Fatal("init wrote nothing")
	}
}
