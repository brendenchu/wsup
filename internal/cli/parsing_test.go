package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpAndVersionDoNotBypassInvalidOptions(t *testing.T) {
	for _, args := range [][]string{
		{"--help", "--invalid"},
		{"--version", "--invalid"},
		{"guidance", "--help", "--invalid"},
		{"vscode", "--help", "--invalid"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 2 || stdout.Len() != 0 ||
			!strings.HasPrefix(stderr.String(), "Error: ") {
			t.Fatalf("%v: stdout=%q stderr=%q", args, &stdout, &stderr)
		}
	}
}
