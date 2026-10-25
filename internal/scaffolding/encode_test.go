package scaffolding

import (
	"bytes"
	"reflect"
	"testing"
)

func TestNativeEncodingRoundTripDoesNotMigrateLegacy(t *testing.T) {
	native, err := ParseState(encoded(t, stateDocument(StateVersion)))
	if err != nil {
		t.Fatal(err)
	}
	body, err := EncodeNativeState(native)
	if err != nil || !bytes.HasSuffix(body, []byte("\n")) {
		t.Fatalf("native encoding: %q %v", body, err)
	}
	roundtrip, err := ParseState(body)
	if err != nil || !reflect.DeepEqual(native, roundtrip) {
		t.Fatalf("roundtrip changed baselines/selections: %+v %v", roundtrip, err)
	}
	again, err := EncodeNativeState(roundtrip)
	if err != nil || !bytes.Equal(body, again) {
		t.Fatal("native state serialization is not deterministic")
	}
	legacy, err := ParseState(encoded(t, stateDocument(1)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EncodeNativeState(legacy); err == nil || legacy.Version != 1 {
		t.Fatal("serializer silently migrated legacy ownership")
	}
}

func TestProjectStateWithoutSelections(t *testing.T) {
	document := stateDocument(StateVersion)
	document["template"] = "project"
	delete(document, "collections")
	delete(document, "omitted_home_files")
	state, err := ParseState(encoded(t, document))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EncodeNativeState(state); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidUTF8StateIsNotReplacedSilently(t *testing.T) {
	if _, err := ParseState([]byte("{\"template_version\":\"\xff\"}")); err == nil {
		t.Fatal("invalid UTF-8 state accepted")
	}
}
