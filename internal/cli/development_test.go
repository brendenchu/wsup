package cli

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/brendenchu/wsup/internal/registry"
)

func TestDevelopmentRegistryUsesNativeChecksAndExplicitPythonReference(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	data, err := registry.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := data.Select([]string{"wsup"}, nil)
	if err != nil || len(selected) != 3 {
		t.Fatalf("development checks: %+v %v", selected, err)
	}
	expected := map[string][]string{
		"test":             {"go", "test", "-race", "./..."},
		"vet":              {"go", "vet", "./..."},
		"python-reference": {"{python}", "-B", "-m", "unittest", "discover", "-s", "tests"},
	}
	for _, item := range selected {
		if item.Project.Path != "." || !reflect.DeepEqual(item.Check.Command, expected[item.Name]) || !item.Check.Enabled {
			t.Fatalf("incorrect native/reference check: %+v", item)
		}
		delete(expected, item.Name)
	}
	if len(expected) != 0 {
		t.Fatalf("missing development checks: %v", expected)
	}
}
