package html

import (
	"html"
	"io"
	"regexp"
	"slices"
	"strings"
)

// allowedTags is the raw HTML that passes through, the same list as
// pkg/export/confluence's allowedTags (DESIGN-0020 §2, DESIGN-0025 §2).
// It is a copy rather than an import: the two exports are siblings, and
// neither depends on the other.
var allowedTags = map[string]bool{
	"br": true, "kbd": true, "sub": true, "sup": true,
	"span": true, "details": true, "summary": true,
}

// tagPattern matches one tag, capturing whether it closes and its name.
var tagPattern = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9-]*)\b[^<>]*>`)

// rebuild turns a raw HTML fragment into safe HTML. A fragment whose every
// tag is allowed is rebuilt: each tag written bare, with no attribute, so
// no handler or URL a document carries survives, and every byte between
// tags escaped. Anything else is escaped whole, so a <status> placeholder in
// prose stays text. A closing tag that closes nothing open is dropped, and
// the tags left open are closed at the end of the panel (closeOpen), so an
// unclosed <details> cannot swallow the panels after it.
func (nr *nodeRenderer) rebuild(raw string) (string, bool) {
	tags := tagPattern.FindAllStringSubmatchIndex(raw, -1)
	if len(tags) == 0 {
		return html.EscapeString(raw), false
	}

	for _, m := range tags {
		if !allowedTags[strings.ToLower(raw[m[4]:m[5]])] {
			return html.EscapeString(raw), false
		}
	}

	var b strings.Builder

	at := 0

	for _, m := range tags {
		b.WriteString(html.EscapeString(raw[at:m[0]]))
		at = m[1]

		name := strings.ToLower(raw[m[4]:m[5]])
		closing := m[3] > m[2]

		switch {
		case name == "br":
			b.WriteString("<br>")
		case !closing:
			b.WriteString("<" + name + ">")
			nr.open = append(nr.open, name)
		default:
			if i := slices.Index(nr.open, name); i >= 0 {
				// Close everything opened inside it too.
				for j := len(nr.open) - 1; j >= i; j-- {
					b.WriteString("</" + nr.open[j] + ">")
				}

				nr.open = nr.open[:i]
			}
		}
	}

	b.WriteString(html.EscapeString(raw[at:]))

	return b.String(), true
}

// closeOpen closes every allowed tag still open, innermost first.
func (nr *nodeRenderer) closeOpen(w io.Writer) {
	for i := len(nr.open) - 1; i >= 0; i-- {
		put(w, "</"+nr.open[i]+">")
	}

	nr.open = nr.open[:0]
}
