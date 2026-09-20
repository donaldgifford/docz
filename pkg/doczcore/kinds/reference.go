package kinds

import "strings"

// Reference is one entry of a references section.
type Reference struct {
	// Text is the bullet with wrapped lines folded, inline markdown kept.
	Text string

	// URL is the target of the first markdown link in the bullet, or ""
	// when the bullet has none. An empty URL is what validate's content
	// rule for the references kind reports, so the finding and the parsed
	// value read the same bullet.
	URL string

	// Line is the 1-based line of the bullet, counted from the start of
	// the region.
	Line int
}

// References returns the region's top-level bullets as references.
//
// A bullet with no link is still a reference. Reporting it with an empty
// URL rather than dropping it is what lets validate say which bullet is
// missing a link instead of only that one is.
func References(region []byte) []Reference {
	folded := topLevel(foldItems(region))

	out := make([]Reference, 0, len(folded))

	for _, it := range folded {
		if it.Text == "" {
			continue
		}

		out = append(out, Reference{Text: it.Text, URL: firstLinkURL(it.Text), Line: it.Line})
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

// firstLinkURL returns the target of the first markdown inline link in s,
// or "" when there is none.
//
// Nested brackets in the label are handled by scanning for the "](" that
// closes the label rather than the first "]", so "[see [ADR-0002]](x.md)"
// yields x.md. A bare autolink in angle brackets counts too: the corpus
// writes both forms.
func firstLinkURL(s string) string {
	if open := strings.Index(s, "["); open >= 0 {
		if mid := strings.Index(s[open:], "]("); mid >= 0 {
			rest := s[open+mid+2:]
			if end := strings.Index(rest, ")"); end >= 0 {
				// A title after the URL ("(url \"t\")") is not part of it.
				url, _, _ := strings.Cut(strings.TrimSpace(rest[:end]), " ")

				return url
			}
		}
	}

	open := strings.Index(s, "<")
	if open < 0 {
		return ""
	}

	rest := s[open+1:]

	end := strings.Index(rest, ">")
	if end < 0 {
		return ""
	}

	url := rest[:end]
	if strings.ContainsAny(url, " \t") || !strings.Contains(url, ":") {
		return ""
	}

	return url
}
