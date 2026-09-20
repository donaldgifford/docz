// Package kinds reads the region kinds that more than one document type
// shares, so five type packages do not carry five copies of the
// open-question grammar (DESIGN-0014 §2.12).
//
// Every reader takes the bytes of one region — heading included — and
// returns values. None touches the filesystem, returns an error, or is
// told which document type it is reading: a region's kind is the only
// thing that selects a reader, which is what keeps the grammar a contract
// over kinds rather than over types (ADR-0002 R7). The validate package's
// content rules call these same readers, so a finding and a parsed value
// can never disagree about what a region says.
//
// The spans themselves come from the sibling docparse package, either
// from real markers (docparse.Regions) or, for a document that carries
// none, from InferRegions over a HeadingSpec. Inference is permanent, not
// a migration aid (DESIGN-0015 §6): the fleet's existing documents are
// read by heading and will not all be re-marked.
package kinds

import (
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

// A region's own heading is line 1 of the bytes a reader is handed. Every
// reader drops it: a heading names the span, it is not content in it.
//
// Dropping by line rather than by matching means a region whose first line
// is not a heading — an inferred region always opens on one, a marked
// region need not — loses that line too. That is deliberate: the caller
// passes a region, and a region's first line is its heading or its
// marker, never something a reader should report.
func bodyLines(region []byte) []string {
	lines := strings.Split(strings.TrimSuffix(string(region), "\n"), "\n")
	if len(lines) == 0 {
		return nil
	}

	return lines[1:]
}

// commentPattern is not a regexp: an HTML comment can span lines, and the
// templates' guidance comments do. stripComments walks instead.
//
// The templates put their instructions to the author in comments, so a
// reader that kept them would report the instruction as the content. An
// unterminated comment swallows the rest of the input, which is what a
// markdown renderer does with it too.
func stripComments(s string) string {
	var sb strings.Builder

	for {
		open := strings.Index(s, "<!--")
		if open < 0 {
			sb.WriteString(s)

			return sb.String()
		}

		sb.WriteString(s[:open])

		rest := s[open+4:]

		shut := strings.Index(rest, "-->")
		if shut < 0 {
			return sb.String()
		}

		s = rest[shut+3:]
	}
}

// foldedItem is a list item with its continuation lines folded in: the
// shape Items, Criteria, References, Alternatives, and an open question's
// options all need, so the folding rule has one definition.
type foldedItem struct {
	Text    string
	Ordered bool
	Indent  int
	Line    int
	EndLine int
}

// foldItems returns the region's list items with continuation lines
// folded into Text, joined by single spaces.
//
// A continuation line is non-blank, indented deeper than its bullet, and
// not itself a list item. That is the rule the corpus needs: 48 of the 56
// tasks in IMPL-0017 wrap (INV-0010). A blank line ends an item, so two
// paragraphs under one bullet keep only the first — a reader reports what
// the item says, not everything filed beneath it.
func foldItems(region []byte) []foldedItem {
	return foldLines(bodyLines(region))
}

// foldLines is foldItems over lines already cut from a region. Line numbers
// are 1-based within the slice, so a caller that also indexes those lines —
// OpenQuestions walks them for its blockquotes — reads one numbering.
func foldLines(lines []string) []foldedItem {
	items := docparse.ListItems([]byte(strings.Join(lines, "\n")))

	if len(items) == 0 {
		return nil
	}

	starts := make(map[int]bool, len(items))
	for _, it := range items {
		starts[it.Line] = true
	}

	out := make([]foldedItem, 0, len(items))

	for _, it := range items {
		folded := foldedItem{
			Text:    it.Text,
			Ordered: it.Ordered,
			Indent:  it.Indent,
			Line:    it.Line,
			EndLine: it.Line,
		}

		parts := []string{it.Text}

		for n := it.Line + 1; n <= len(lines); n++ {
			line := lines[n-1]
			if strings.TrimSpace(line) == "" || starts[n] {
				break
			}

			if indentOf(line) <= it.Indent {
				break
			}

			parts = append(parts, strings.TrimSpace(line))
			folded.EndLine = n
		}

		folded.Text = strings.TrimSpace(strings.Join(parts, " "))
		out = append(out, folded)
	}

	return out
}

// indentOf counts leading whitespace characters. Tabs count as one and are
// not expanded, matching docparse.ListItem.Indent so the two agree about
// which lines are deeper than a bullet.
func indentOf(line string) int {
	for i, r := range line {
		if r != ' ' && r != '\t' {
			return i
		}
	}

	return len(line)
}

// topLevel keeps the items at indent 0. A nested bullet is part of what
// its parent says, not a sibling of it; every reader that reports "the
// bullets in this region" means the top-level ones.
func topLevel(items []foldedItem) []foldedItem {
	out := make([]foldedItem, 0, len(items))

	for _, it := range items {
		if it.Indent == 0 {
			out = append(out, it)
		}
	}

	return out
}

// backtickSpan returns the contents of the first backtick span in s, and
// whether s starts with one. The two answers come together because the
// callers need both: Criterion.Executable is "starts with a span" and
// Criterion.Command is "the span's contents".
func backtickSpan(s string) (string, bool) {
	open := strings.Index(s, "`")
	if open < 0 {
		return "", false
	}

	rest := s[open+1:]

	end := strings.Index(rest, "`")
	if end < 0 {
		return "", false
	}

	return rest[:end], open == 0
}
