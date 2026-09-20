package docparse

import (
	"regexp"
	"strings"
)

// ListItem is a single markdown list-item fact, bulleted or numbered.
//
// TaskItems is the checkbox subset of the same lines: every line
// TaskItems reports is also a ListItem, and the difference is what Text
// holds. A checkbox item's ListItem.Text keeps its "[ ]" or "[x]" prefix,
// because to a list walker that is simply the first thing the item says;
// TaskItems strips the marker and reports Checked instead. A consumer
// that wants checkbox semantics calls TaskItems, and one that wants "the
// bullets in this region" calls ListItems and gets both kinds.
type ListItem struct {
	// Text is the item text with the bullet or number and its trailing
	// whitespace stripped, then trimmed. Inline markdown is preserved
	// verbatim, as in TaskItem.Text, so a consumer can match Text back to
	// the raw line.
	Text string

	// Ordered is true for a numbered item ("1." or "1)") and false for a
	// bulleted one ("-", "*", or "+"). The number itself is not reported:
	// markdown renumbers ordered lists from the first value, so the
	// literal digits are not a fact about the document's meaning.
	Ordered bool

	// Indent is the number of leading whitespace characters before the
	// bullet — spaces and tabs each count as one; tabs are not expanded.
	// It is a raw fact: consumers apply their own nesting policy, which
	// is how impl decides a task is top-level and kinds decides an option
	// belongs to a question.
	Indent int

	// Line is the 1-based line number of the item, counted by LF.
	Line int
}

// listItemPattern matches a bulleted or numbered list item: optional
// leading whitespace, a bullet ("-", "*", "+") or a number followed by
// "." or ")", then whitespace and optional text.
//
// Whitespace after the marker is required, following GFM: "-text" is a
// paragraph and "1.text" is not a list. An empty item ("-" alone) is a
// list item with empty text, which is why the text group is optional.
var listItemPattern = regexp.MustCompile(
	`^([ \t]*)(?:([-*+])|(\d{1,9})[.)])(?:[ \t]+(.*))?$`,
)

// ListItems extracts every list item from content: bulleted with "-",
// "*", or "+", or numbered with "1." or "1)". Items inside fenced code
// blocks are skipped using the same fence rule as Headings.
//
// A thematic break ("---", "***") is not a list item even though it
// starts with a bullet character, because it carries no text and repeats
// the marker.
//
// Lines are counted by LF, matching the rest of the package.
func ListItems(content []byte) []ListItem {
	lines := strings.Split(string(content), "\n")

	var items []ListItem

	inCodeBlock := false

	for i, line := range lines {
		if isFenceToggle(line) {
			inCodeBlock = !inCodeBlock

			continue
		}

		if inCodeBlock {
			continue
		}

		if isThematicBreak(line) {
			continue
		}

		m := listItemPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		items = append(items, ListItem{
			Text:    strings.TrimSpace(m[4]),
			Ordered: m[3] != "",
			Indent:  len(m[1]),
			Line:    i + 1,
		})
	}

	return items
}

// isThematicBreak reports whether the line is a markdown horizontal rule:
// three or more of "-", "*", or "_", optionally spaced. The IMPL template
// puts "---" between phases, and reporting those as empty list items
// would put a phantom bullet in every phase list.
func isThematicBreak(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 3 {
		return false
	}

	marker := trimmed[0]
	if marker != '-' && marker != '*' && marker != '_' {
		return false
	}

	count := 0

	for i := range len(trimmed) {
		switch c := trimmed[i]; c {
		case marker:
			count++
		case ' ', '\t':
		default:
			return false
		}
	}

	return count >= 3
}
