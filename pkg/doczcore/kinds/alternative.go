package kinds

import (
	"regexp"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

// Alternative is one option an RFC or ADR considered and did not take.
type Alternative struct {
	// Label is the alternative's letter or number when it has one,
	// lower-cased, else "". The corpus writes both an unlabelled list of
	// alternatives and a lettered one, and neither is wrong.
	Label string

	// Title is the alternative's name: the bold lead-in of a bullet, or
	// the heading text of a level-3 section, with the label removed.
	Title string

	// Text is the rest of the alternative — why it was rejected. Empty
	// when the document gives only a name.
	Text string

	// Line is the 1-based line of the bullet or heading, counted from the
	// start of the region.
	Line int
}

var (
	// boldLeadIn matches a bullet that opens with a bold title:
	// "- **A. Read the template.** Zero config, …" or "- **Do nothing.**".
	boldLeadIn = regexp.MustCompile(`^\*\*(.+?)\*\*[.:]?\s*(.*)$`)

	// labelPrefix matches a leading label on a title or heading: "A.",
	// "a)", "3.".
	labelPrefix = regexp.MustCompile(`^([a-zA-Z0-9]{1,2})[.)]\s+(.*)$`)
)

// Alternatives returns the region's alternatives, read from whichever of
// the two shapes the document uses: top-level bullets, or level-3 headings
// with their bodies.
//
// Bullets win when the region has both. A region that leads with bullets
// and then explains one of them under a heading is describing one set of
// alternatives, and counting the heading again would double it.
func Alternatives(region []byte) []Alternative {
	if out := bulletAlternatives(region); len(out) > 0 {
		return out
	}

	return headingAlternatives(region)
}

func bulletAlternatives(region []byte) []Alternative {
	folded := topLevel(foldItems(region))

	out := make([]Alternative, 0, len(folded))

	for _, it := range folded {
		if it.Text == "" {
			continue
		}

		label, rest := splitLabel(it.Text)
		title, text := splitBoldLeadIn(rest)
		out = append(out, Alternative{Label: label, Title: title, Text: text, Line: it.Line})
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

func headingAlternatives(region []byte) []Alternative {
	sections := Sections(region)

	out := make([]Alternative, 0, len(sections))

	for _, s := range sections {
		// A heading's text is the alternative's name, and the section under
		// it is why the alternative was or was not taken.
		label, title := splitLabel(s.Title)
		out = append(out, Alternative{Label: label, Title: title, Text: s.Body, Line: s.Line})
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

// splitLabel pulls a leading letter or number label off an alternative:
// "a. Read the template" yields "a" and the rest. A label is read before a
// bold lead-in, because the corpus writes the letter outside the bold.
func splitLabel(s string) (label, rest string) {
	m := labelPrefix.FindStringSubmatch(s)
	if m == nil {
		return "", s
	}

	return strings.ToLower(m[1]), strings.TrimSpace(m[2])
}

// splitBoldLeadIn separates a bold title from the text after it.
//
// A bullet with no bold lead-in has no title of its own, so the whole
// bullet is its text: a one-line alternative is still an alternative, and
// inventing a title by cutting at the first period would mangle a bullet
// like "Use goldmark v1.7, since it is already a dependency".
func splitBoldLeadIn(s string) (title, text string) {
	m := boldLeadIn.FindStringSubmatch(s)
	if m == nil {
		return "", s
	}

	return strings.TrimSpace(m[1]), strings.TrimSpace(m[2])
}

// headingText is the comparable form of a heading: inline markdown already
// stripped by docparse, then HTML comments removed, folded, and trimmed.
// Used by the inference spec so a template's placeholder heading and a
// document's real one compare equal on the part they share.
func headingText(h docparse.Heading) string {
	return strings.ToLower(strings.TrimSpace(stripComments(h.Text)))
}
