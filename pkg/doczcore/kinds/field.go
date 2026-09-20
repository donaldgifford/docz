package kinds

import "strings"

// Field returns the text after a bold label on a line of the region, and
// whether the label was found.
//
// The corpus uses this shape for the one-line fields the templates ask
// for: an IMPL's "**Implements:** DESIGN-0011", an investigation's
// "**Answer:** Yes", its "**Triggered by:** issue #100". The label is
// given without punctuation ("Implements", "Answer"), and both bold
// spellings the fleet writes are accepted — the colon inside the bold
// ("**Answer:**") and outside it ("**Answer**:").
//
// The value has HTML comments stripped, so a field the author has not
// filled in is found with an empty value rather than returning the
// template's placeholder. That distinction is the point of the second
// return: "the document has no Answer line" and "the document has an
// empty Answer line" are different states, and only the first is a
// missing field.
func Field(region []byte, label string) (string, bool) {
	inside := "**" + label + ":**"
	outside := "**" + label + "**:"

	for _, line := range bodyLines(region) {
		trimmed := strings.TrimSpace(line)

		// A blockquote or a bullet may carry a field; the marker is not
		// part of the label.
		trimmed = strings.TrimLeft(trimmed, "> ")
		trimmed = strings.TrimPrefix(trimmed, "- ")

		for _, form := range []string{inside, outside} {
			rest, ok := strings.CutPrefix(trimmed, form)
			if !ok {
				continue
			}

			return strings.TrimSpace(stripComments(rest)), true
		}
	}

	return "", false
}
