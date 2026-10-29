package scaffolding

import "testing"

func TestSpliceBlocksReplacesOnlyUnchangedGeneratedSections(t *testing.T) {
	prior := "# Title\n\n<!-- wsup:begin layout -->\n- a\n<!-- wsup:end layout -->\n\n# wsup:begin ignore\nx\n# wsup:end ignore\n"
	next := "# Title\n\n<!-- wsup:begin layout -->\n- a\n- b\n<!-- wsup:end layout -->\n\n# wsup:begin ignore\nx\ny\n# wsup:end ignore\n"
	current := "# Owner title\n\nOwner purpose.\n\n<!-- wsup:begin layout -->\n- a\n<!-- wsup:end layout -->\n\nOwner notes.\n# wsup:begin ignore\nx\n# wsup:end ignore\nowner-entry\n"
	want := "# Owner title\n\nOwner purpose.\n\n<!-- wsup:begin layout -->\n- a\n- b\n<!-- wsup:end layout -->\n\nOwner notes.\n# wsup:begin ignore\nx\ny\n# wsup:end ignore\nowner-entry\n"
	got, ok := spliceBlocks([]byte(current), []byte(prior), []byte(next))
	if !ok || string(got) != want {
		t.Fatalf("splice: %v %q", ok, got)
	}
	for name, edited := range map[string]string{
		"edited inside a section": "<!-- wsup:begin layout -->\n- a\n- owner\n<!-- wsup:end layout -->\n# wsup:begin ignore\nx\n# wsup:end ignore\n",
		"section removed":         "<!-- wsup:begin layout -->\n- a\n<!-- wsup:end layout -->\n",
		"unclosed section":        "<!-- wsup:begin layout -->\n- a\n# wsup:begin ignore\nx\n# wsup:end ignore\n",
		"no sections":             "Owner only\n",
	} {
		if _, ok := spliceBlocks([]byte(edited), []byte(prior), []byte(next)); ok {
			t.Fatalf("%s spliced", name)
		}
	}
}
