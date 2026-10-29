package scaffolding

import (
	"bytes"
	"regexp"
	"slices"
)

// blockMarker matches one whole begin or end line of a wsup-generated section,
// in Markdown (<!-- wsup:begin name -->) or line-comment (# wsup:begin name) form.
var blockMarker = regexp.MustCompile(`(?m)^(?:<!-- wsup:(begin|end) ([a-z][a-z0-9-]*) -->|# wsup:(begin|end) ([a-z][a-z0-9-]*))[ \t]*\r?$`)

// block is a generated section's byte span, from its begin line through the end
// of its end line.
type block struct {
	name       string
	start, end int
}

// findBlocks returns the generated sections in order, or false when markers do
// not pair up (an unclosed, nested, or unmatched marker).
func findBlocks(body []byte) ([]block, bool) {
	blocks := []block{}
	open := -1
	for _, match := range blockMarker.FindAllSubmatchIndex(body, -1) {
		// Groups 1–2 are the Markdown form, 3–4 the line-comment form.
		groups := match[2:6]
		if groups[0] < 0 {
			groups = match[6:10]
		}
		kind, name := body[groups[0]:groups[1]], body[groups[2]:groups[3]]
		end := match[1]
		if end < len(body) && body[end] == '\n' {
			end++
		}
		switch {
		case string(kind) == "begin" && open < 0:
			blocks = append(blocks, block{name: string(name), start: match[0]})
			open = len(blocks) - 1
		case string(kind) == "end" && open >= 0 && blocks[open].name == string(name):
			blocks[open].end = end
			open = -1
		default:
			return nil, false
		}
	}
	return blocks, open < 0
}

// spliceBlocks replaces the generated sections of an owner-edited file with
// those of next, keeping every byte outside them. It succeeds only when current
// and prior (the content wsup generated for the previous selection) have the
// same sections and the owner left every section's text unchanged; otherwise
// the edit belongs to the owner and must be resolved by hand.
func spliceBlocks(current, prior, next []byte) ([]byte, bool) {
	have, okHave := findBlocks(current)
	was, okWas := findBlocks(prior)
	want, okWant := findBlocks(next)
	names := func(blocks []block) []string {
		result := []string{}
		for _, b := range blocks {
			result = append(result, b.name)
		}
		return result
	}
	if !okHave || !okWas || !okWant || len(have) == 0 ||
		!slices.Equal(names(have), names(was)) || !slices.Equal(names(have), names(want)) {
		return nil, false
	}
	var spliced bytes.Buffer
	last := 0
	for i, b := range have {
		if !bytes.Equal(current[b.start:b.end], prior[was[i].start:was[i].end]) {
			return nil, false
		}
		spliced.Write(current[last:b.start])
		spliced.Write(next[want[i].start:want[i].end])
		last = b.end
	}
	spliced.Write(current[last:])
	return spliced.Bytes(), true
}
