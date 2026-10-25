package cli

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/brendenchu/wsup/internal/registry"
)

func TestDevelopmentRegistryUsesNativeChecks(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	data, err := registry.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := data.Select([]string{"wsup"}, nil)
	if err != nil || len(selected) != 2 {
		t.Fatalf("development checks: %+v %v", selected, err)
	}
	expected := map[string][]string{
		"test": {"go", "test", "-race", "./..."},
		"vet":  {"go", "vet", "./..."},
	}
	for _, item := range selected {
		if item.Project.Path != "." || !reflect.DeepEqual(item.Check.Command, expected[item.Name]) || !item.Check.Enabled {
			t.Fatalf("incorrect native check: %+v", item)
		}
		delete(expected, item.Name)
	}
	if len(expected) != 0 {
		t.Fatalf("missing development checks: %v", expected)
	}
}
