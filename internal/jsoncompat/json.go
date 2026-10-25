// Package jsoncompat preserves Python's ASCII JSON encoding where byte identity matters.
package jsoncompat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode/utf16"
)

// Marshal uses Python-compatible ASCII string escapes and optional indentation.
// Compact output uses Go's compact separators; callers needing Python's default
// comma/colon spaces must supply those separators explicitly.
func Marshal(value any, indent string) ([]byte, error) {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetIndent("", indent)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	var ascii bytes.Buffer
	for _, char := range string(bytes.TrimSuffix(encoded.Bytes(), []byte("\n"))) {
		if char < 127 {
			ascii.WriteRune(char)
		} else if char <= 0xffff {
			fmt.Fprintf(&ascii, `\u%04x`, char)
		} else {
			high, low := utf16.EncodeRune(char)
			fmt.Fprintf(&ascii, `\u%04x\u%04x`, high, low)
		}
	}
	return ascii.Bytes(), nil
}
