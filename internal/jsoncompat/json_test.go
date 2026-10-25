package jsoncompat

import (
	"testing"
)

func TestASCIIEscapesAndIndentation(t *testing.T) {
	body, err := Marshal(map[string]string{"path": "caf\u00e9/\U0001f680/\x7f/<>&/\u2028"}, "  ")
	want := "{\n  \"path\": \"caf\\u00e9/\\ud83d\\ude80/\\u007f/<>&/\\u2028\"\n}"
	if err != nil || string(body) != want {
		t.Fatalf("got %q, want %q: %v", body, want, err)
	}
}

func TestMarshalErrorsAreNotHidden(t *testing.T) {
	if _, err := Marshal(make(chan int), ""); err == nil {
		t.Fatal("unsupported JSON value accepted")
	}
}
