package catalogue

import (
	"bytes"
	"io/fs"
	"reflect"
	"testing"
)

func TestSelectedCollectionBundleInventoryAndExactBytes(t *testing.T) {
	all := []string{"archive", "artifacts", "journal", "learning", "private", "reference", "research"}
	files, err := BundleSelected("workspace", all)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := Bundle("workspace")
	if err != nil {
		t.Fatal(err)
	}
	source := Open()
	count := len(plain)
	for _, collection := range all {
		for _, filename := range []string{"README.md", "TEMPLATE.md"} {
			body, err := fs.ReadFile(source, "modules/"+collection+"/files/"+collection+"/"+filename)
			if filename == "TEMPLATE.md" && collection != "journal" {
				if err == nil {
					t.Fatalf("unexpected collection source: %s", collection)
				}
				continue
			}
			count++
			if err != nil || !bytes.Equal(body, files[collection+"/"+filename]) {
				t.Fatalf("collection bytes differ: %s/%s %v", collection, filename, err)
			}
		}
	}
	if count != len(files) {
		t.Fatalf("bundle inventory %d, want %d", len(files), count)
	}
	again, err := BundleSelected("workspace", append(all, "archive", "journal"))
	if err != nil || !reflect.DeepEqual(files, again) {
		t.Fatal("collection deduplication changed output")
	}
	for _, names := range [][]string{{"unknown"}, {"../workspace"}, {""}} {
		if _, err := BundleSelected("workspace", names); err == nil {
			t.Fatalf("invalid collection accepted: %v", names)
		}
	}
	if _, err := BundleSelected("project", []string{"archive"}); err == nil {
		t.Fatal("project collection selection accepted")
	}
}

func TestComposeCollectionsExpandsPresetsAndAddsSelections(t *testing.T) {
	development := []string{"apps", "infrastructure", "packages", "tools"}
	for _, tc := range []struct {
		preset      string
		collections []string
		want        []string
	}{
		{DefaultPreset, nil, development},
		{DefaultPreset, []string{"journal", "apps"}, []string{"apps", "infrastructure", "journal", "packages", "tools"}},
		{NoPreset, nil, []string{}},
		{NoPreset, []string{"research", "apps", "research"}, []string{"apps", "research"}},
	} {
		got, err := ComposeCollections(tc.preset, tc.collections)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s %v: %v %v", tc.preset, tc.collections, got, err)
		}
	}
	for _, preset := range []string{"", "unknown", "apps", "workspace-base"} {
		if _, err := ComposeCollections(preset, nil); err == nil {
			t.Fatalf("invalid preset accepted: %q", preset)
		}
	}
	if _, err := ComposeCollections(DefaultPreset, []string{"unknown"}); err == nil {
		t.Fatal("unknown collection accepted with a preset")
	}
	if !reflect.DeepEqual(PresetNames(), []string{DefaultPreset}) {
		t.Fatalf("presets: %v", PresetNames())
	}
}
