package kinds

import (
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

// Item is a top-level list item with its continuation lines folded in.
type Item struct {
	// Text is the item, wrapped lines joined by single spaces. Inline
	// markdown is kept verbatim so a consumer can match Text back to the
	// document.
	Text string

	// Line is the 1-based line of the bullet, counted from the start of
	// the region.
	Line int
}

// Section is a level-3 heading inside a region and the body under it.
type Section struct {
	// Title is the heading text with inline markdown stripped.
	Title string

	// Body is everything under the heading up to the next level-3
	// heading, HTML comments removed and trimmed.
	Body string

	// Line is the 1-based line of the heading, counted from the start of
	// the region.
	Line int
}

// Body returns the region's text without its heading or HTML comments,
// trimmed.
//
// This is the reader for a kind with no structure of its own — an RFC's
// problem statement, an ADR's decision, an IMPL's dependencies. The
// comments go because the templates put their guidance to the author in
// them, so a document the author has not filled in yields "" rather than
// the instructions it was shipped with.
func Body(region []byte) string {
	return strings.TrimSpace(stripComments(strings.Join(bodyLines(region), "\n")))
}

// Items returns the region's top-level list items, bulleted or numbered,
// with continuation lines folded in.
//
// This is the reader for goals, non-goals, in-scope, out-of-scope, and an
// ADR's positive, negative, and neutral consequences: a kind whose content
// is a flat list of statements.
//
// A nested bullet is not reported. It is neither a top-level item nor a
// continuation line, and Item has nowhere to put it — the kinds this reads
// are flat by contract. A consumer that needs the nesting reads
// docparse.ListItems over the same bytes, where Indent is a fact.
func Items(region []byte) []Item {
	folded := topLevel(foldItems(region))

	out := make([]Item, 0, len(folded))

	for _, it := range folded {
		if it.Text == "" {
			continue
		}

		out = append(out, Item{Text: it.Text, Line: it.Line + bodyOffset})
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

// Sections returns the level-3 headings inside the region with their
// bodies. An investigation's findings are the motivating case: each
// observation is a level-3 heading and the evidence under it.
//
// Only level 3 counts. A deeper heading is part of its section's body,
// which is how a finding that breaks its evidence into sub-points stays
// one observation.
func Sections(region []byte) []Section {
	lines := bodyLines(region)
	body := strings.Join(lines, "\n")

	var starts []docparse.Heading

	for _, h := range docparse.Headings([]byte(body)) {
		if h.Level == 3 {
			starts = append(starts, h)
		}
	}

	if len(starts) == 0 {
		return nil
	}

	out := make([]Section, 0, len(starts))

	for i, h := range starts {
		end := len(lines)
		if i+1 < len(starts) {
			end = starts[i+1].Line - 1
		}

		out = append(out, Section{
			Title: h.Text,
			Body:  strings.TrimSpace(stripComments(strings.Join(lines[h.Line:end], "\n"))),
			Line:  h.Line + bodyOffset,
		})
	}

	return out
}
