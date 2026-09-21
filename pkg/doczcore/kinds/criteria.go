package kinds

import "strings"

// Criterion is one success criterion: a statement of how to tell the work
// is done.
type Criterion struct {
	// Text is the criterion with wrapped lines folded and a leading
	// checkbox removed.
	Text string

	// Executable is true when the criterion opens with a backtick span,
	// which the corpus uses for a criterion that is a command to run —
	// "`make ci` passes with zero errors".
	Executable bool

	// Command is the contents of the opening backtick span, and is set only
	// when Executable is. A span in the middle of a criterion names
	// something else — the corpus writes filenames and identifiers there,
	// "every `.orig.md` fixture parses" — and a consumer reading Command
	// without checking Executable would offer to run one.
	Command string

	// Line is the 1-based line of the bullet, counted from the start of
	// the region.
	Line int
}

// Criteria returns the region's top-level dash bullets as criteria.
//
// One definition serves both positions the kind appears in: an RFC's
// success criteria at the top level and an IMPL phase's inside its phase
// (DESIGN-0013's rule, moved here unchanged). A leading checkbox is
// tolerated, because half the corpus writes criteria as a checklist and
// the other half as bullets, and the distinction carries no meaning here.
//
// Numbered items are not criteria. The kind is documented as dash bullets,
// and an ordered list in this position is a procedure, not a checklist.
func Criteria(region []byte) []Criterion {
	folded := topLevel(foldItems(region))

	out := make([]Criterion, 0, len(folded))

	for _, it := range folded {
		if it.Ordered {
			continue
		}

		text := trimCheckbox(it.Text)
		if text == "" {
			continue
		}

		command, leading := backtickSpan(text)
		if !leading {
			command = ""
		}

		out = append(out, Criterion{
			Text:       text,
			Executable: leading,
			Command:    command,
			Line:       it.Line + bodyOffset,
		})
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

// trimCheckbox removes a leading GFM checkbox marker. docparse.ListItem
// keeps it, because to a list walker it is simply the first thing the item
// says; a criterion's checkbox is state, not text.
func trimCheckbox(text string) string {
	for _, marker := range []string{"[ ] ", "[x] ", "[X] ", "[ ]", "[x]", "[X]"} {
		if rest, ok := strings.CutPrefix(text, marker); ok {
			return strings.TrimSpace(rest)
		}
	}

	return text
}
