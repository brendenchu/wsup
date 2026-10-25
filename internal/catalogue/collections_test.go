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
	source, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	count := len(plain)
	for _, collection := range all {
		for _, filename := range []string{"README.md", "TEMPLATE.md"} {
			body, err := fs.ReadFile(source, "collections/"+collection+"/"+filename)
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
