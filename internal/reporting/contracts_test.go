package reporting

import (
	"encoding/json"
	"reflect"
	"runtime"
	"testing"
)

func TestCommandDigestMatchesPython(t *testing.T) {
	for _, tc := range []struct {
		command []string
		digest  string
	}{
		{nil, "4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945"},
		{[]string{"tool"}, "f3a60c587a13313f1669991d6dbf560c8deb18ac2b5602f05c121d3ed630c964"},
		{[]string{"{python}", "-c", "print(\"caf\u00e9 \U0001f680\")", "", "$(touch unsafe)", "space value", "\x7f", "<>&", "\u2028", "\\", "\b\f\n\r\t"},
			"6370d1b1e45f10c199b8319a9bced3b20e71027e0ae5672c9a8391b4e1579a3a"},
	} {
		actual, err := CommandDigest(tc.command)
		if err != nil || actual != tc.digest {
			t.Fatalf("digest %q, want %q: %v", actual, tc.digest, err)
		}
	}
	if _, err := CommandDigest([]string{"tool", "\xff"}); err == nil {
		t.Fatal("invalid UTF-8 silently changed command identity")
	}
}

func TestRuntimeMetadataIsNativeAndVersioned(t *testing.T) {
	if SchemaVersion != 2 {
		t.Fatal("native runtime reports require schema 2")
	}
	body, err := json.Marshal(CurrentRuntime())
	if err != nil {
		t.Fatal(err)
	}
	var metadata map[string]string
	if err := json.Unmarshal(body, &metadata); err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{"go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH}
	if !reflect.DeepEqual(metadata, expected) {
		t.Fatalf("runtime metadata: %v, want %v", metadata, expected)
	}
}
