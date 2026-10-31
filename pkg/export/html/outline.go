package html

import (
	"slices"
	"strconv"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

// section is one span of the document that a tab shows: a depth-0 region,
// or an H2 section outside every region. Lines are 1-based and inclusive;
// a region's span is the lines strictly between its markers.
type section struct {
	// Kind is the region kind, or "" for text outside every region.
	Kind string
	// From and Through bound the span's lines.
	From, Through int
	// Heading is the span's first H2 to H6, nil when it has none.
	Heading *docparse.Heading
}

// tab is one tab and its panel.
type tab struct {
	// Key names the tab: data-tab and the panel's id after "p-".
	Key string
	// Label is the tab's text.
	Label string
	// Sections are the spans the panel shows, in document order.
	Sections []section
}

// outline cuts a document into tabs: one per depth-0 region in document
// order, the ToC dropped, and every H2 section outside a region a tab of
// its own, so a document with no regions at all gets one tab per H2. Text
// outside a region with no H2 above it joins the next tab, or the last
// when none follows; text that is only blank lines, an H1, comments, or a
// thematic break is dropped.
func outline(lines []string, regions []docparse.Region, headings []docparse.Heading) []tab {
	var spans []section

	gap := func(from, through int) {
		spans = append(spans, gapSections(lines, headings, from, through)...)
	}

	next := 1

	for _, r := range regions {
		if r.Depth != 0 {
			continue
		}

		gap(next, r.Start-1)

		next = r.End + 1
		if r.Kind == docparse.TocKind {
			continue
		}

		s := section{Kind: r.Kind, From: r.Start + 1, Through: r.End - 1}
		s.Heading = firstHeading(headings, s.From, s.Through)
		spans = append(spans, s)
	}

	gap(next, len(lines))

	return tabsOf(attachIntros(spans))
}

// gapSections splits text outside every region at its H2s. A piece that
// opens with an H2 is a section labelled by it; a piece before the first
// H2 is an intro section (Heading nil), kept only when it has content.
func gapSections(lines []string, headings []docparse.Heading, from, through int) []section {
	if from > through {
		return nil
	}

	var out []section

	start := from

	flush := func(end int, h *docparse.Heading) {
		if start > end {
			return
		}

		if h != nil || hasContent(lines, start, end) {
			out = append(out, section{From: start, Through: end, Heading: h})
		}
	}

	var open *docparse.Heading

	for i := range headings {
		h := &headings[i]
		if h.Level != 2 || h.Line < from || h.Line > through {
			continue
		}

		flush(h.Line-1, open)
		start, open = h.Line, h
	}

	flush(through, open)

	return out
}

// attachIntros folds each intro section (outside every region, with no
// heading) into the section after it, or the one before when it is last.
func attachIntros(spans []section) []section {
	var out []section

	var pending *section

	for i := range spans {
		s := spans[i]

		if s.Kind == "" && s.Heading == nil {
			if pending == nil {
				pending = &spans[i]
			}

			continue
		}

		if pending != nil {
			s.From = pending.From
			if s.Heading == nil {
				s.Heading = pending.Heading
			}

			pending = nil
		}

		out = append(out, s)
	}

	if pending != nil {
		if len(out) == 0 {
			return []section{*pending}
		}

		out[len(out)-1].Through = pending.Through
	}

	return out
}

// tabsOf gives each section a tab of its own, keyed uniquely.
func tabsOf(spans []section) []tab {
	taken := make(map[string]bool)
	out := make([]tab, 0, len(spans))

	for _, s := range spans {
		label := sectionLabel(s)
		out = append(out, tab{Key: uniqueKey(taken, tabKey(label)), Label: label, Sections: []section{s}})
	}

	return out
}

// sectionLabel is the section's heading text, else its kind as words.
func sectionLabel(s section) string {
	if s.Heading != nil && strings.TrimSpace(stripComments(s.Heading.Text)) != "" {
		return strings.TrimSpace(stripComments(s.Heading.Text))
	}

	if s.Kind == "" {
		return "Document"
	}

	words := strings.Split(s.Kind, "-")
	words[0] = strings.ToUpper(words[0][:1]) + words[0][1:]

	return strings.Join(words, " ")
}

// tabKey is a label as a tab name: lower case, runs of anything but a
// letter or digit as one "-".
func tabKey(label string) string {
	var b strings.Builder

	dash := false

	for _, r := range strings.ToLower(label) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)

			dash = false

			continue
		}

		if !dash && b.Len() > 0 {
			b.WriteByte('-')

			dash = true
		}
	}

	key := strings.TrimSuffix(b.String(), "-")
	if key == "" {
		return "tab"
	}

	return key
}

// uniqueKey suffixes a key already taken with -2, -3, and so on.
func uniqueKey(taken map[string]bool, key string) string {
	out := key
	for n := 2; taken[out]; n++ {
		out = key + "-" + strconv.Itoa(n)
	}

	taken[out] = true

	return out
}

// firstHeading is the first heading on lines from through through.
func firstHeading(headings []docparse.Heading, from, through int) *docparse.Heading {
	i := slices.IndexFunc(headings, func(h docparse.Heading) bool {
		return h.Line >= from && h.Line <= through
	})
	if i < 0 {
		return nil
	}

	return &headings[i]
}

// hasContent reports whether lines from through through hold anything a
// reader would see: not blank, not an H1, not only a comment, not a
// thematic break.
func hasContent(lines []string, from, through int) bool {
	for n := from; n <= through && n <= len(lines); n++ {
		t := strings.TrimSpace(lines[n-1])

		switch {
		case t == "", strings.HasPrefix(t, "# "), t == "#":
		case strings.HasPrefix(t, "<!--") && strings.HasSuffix(t, "-->"):
		case isThematicBreak(t):
		default:
			return true
		}
	}

	return false
}

// isThematicBreak reports a line of three or more -, *, or _ alone.
func isThematicBreak(t string) bool {
	t = strings.ReplaceAll(t, " ", "")
	if len(t) < 3 {
		return false
	}

	return strings.Trim(t, "-") == "" || strings.Trim(t, "*") == "" || strings.Trim(t, "_") == ""
}

// stripComments removes HTML comments from heading text, which keeps them:
// a template's placeholder heading "Phase 1: <!-- Name -->" reads "Phase 1:".
func stripComments(s string) string {
	for {
		i := strings.Index(s, "<!--")
		if i < 0 {
			return s
		}

		j := strings.Index(s[i:], "-->")
		if j < 0 {
			return s[:i]
		}

		s = s[:i] + s[i+j+3:]
	}
}
