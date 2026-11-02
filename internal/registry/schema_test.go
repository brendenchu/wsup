package registry

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"github.com/wavykit/wsup/schema"
)

// schemaObject is the part of a JSON Schema object definition the test reads.
type schemaObject struct {
	Properties           map[string]json.RawMessage `json:"properties"`
	AdditionalProperties *bool                      `json:"additionalProperties"`
	Required             []string                   `json:"required"`
}

func TestSchemaDeclaresExactlyWhatDecodeAccepts(t *testing.T) {
	var document struct {
		schemaObject
		Defs map[string]json.RawMessage `json:"$defs"`
		ID   string                     `json:"$id"`
	}
	if err := json.Unmarshal(schema.Workspace, &document); err != nil {
		t.Fatal(err)
	}
	if document.ID != SchemaURL {
		t.Fatalf("$id %q is not the URL generated files name", document.ID)
	}
	object := func(raw json.RawMessage) schemaObject {
		t.Helper()
		var parsed schemaObject
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	var task struct {
		OneOf []json.RawMessage `json:"oneOf"`
	}
	if err := json.Unmarshal(document.Defs["task"], &task); err != nil || len(task.OneOf) != 2 {
		t.Fatalf("task definition: %v", err)
	}
	for name, tc := range map[string]struct {
		object schemaObject
		fields []string
	}{
		"document": {document.schemaObject, documentFields},
		"settings": {object(document.Properties["settings"]), settingsFields},
		"vscode":   {object(document.Properties["vscode"]), vscodeFields},
		"project":  {object(document.Defs["project"]), projectFields},
		"task":     {object(task.OneOf[1]), slices.Concat(checkFields, taskFields)},
	} {
		declared := slices.Sorted(maps.Keys(tc.object.Properties))
		if !slices.Equal(declared, slices.Sorted(slices.Values(tc.fields))) {
			t.Errorf("%s: schema declares %v, Decode accepts %v", name, declared, tc.fields)
		}
		if tc.object.AdditionalProperties == nil || *tc.object.AdditionalProperties {
			t.Errorf("%s: schema must reject unknown fields as Decode does", name)
		}
	}
	var kinds struct {
		Enum []string `json:"enum"`
	}
	if err := json.Unmarshal(object(document.Defs["project"]).Properties["kind"], &kinds); err != nil || !slices.Equal(kinds.Enum, projectKinds) {
		t.Fatalf("schema kinds %v, Decode kinds %v", kinds.Enum, projectKinds)
	}
	var version struct {
		Const int `json:"const"`
	}
	if err := json.Unmarshal(document.Properties["version"], &version); err != nil || version.Const != DocumentVersion {
		t.Fatalf("schema version %d", version.Const)
	}
}
