// Package reporting defines sanitized verification evidence for the native runtime.
package reporting

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"runtime"
	"unicode/utf8"

	"github.com/wavykit/wsup/internal/jsoncompat"
)

// SchemaVersion identifies the runtime metadata and evidence format.
const SchemaVersion = 2

// Runtime identifies the actual CLI runtime, not a tool invoked by a check.
type Runtime struct {
	Go   string `json:"go"`
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

// CurrentRuntime describes the Go binary regardless of installed check tools.
func CurrentRuntime() Runtime {
	return Runtime{Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH}
}

// CommandDigest hashes the literal argument array with comma-space separators
// and ASCII escapes for a stable command identity.
func CommandDigest(command []string) (string, error) {
	var encoded bytes.Buffer
	encoded.WriteByte('[')
	for index, argument := range command {
		if !utf8.ValidString(argument) {
			return "", fmt.Errorf("command arguments must be UTF-8")
		}
		if index != 0 {
			encoded.WriteString(", ")
		}
		value, err := jsoncompat.Marshal(argument, "")
		if err != nil {
			return "", err
		}
		encoded.Write(value)
	}
	encoded.WriteByte(']')
	sum := sha256.Sum256(encoded.Bytes())
	return fmt.Sprintf("%x", sum), nil
}
