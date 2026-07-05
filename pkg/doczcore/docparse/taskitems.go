package docparse

import (
	"regexp"
	"strings"
)

// TaskItem is a single GitHub-style checkbox list-item fact.
type TaskItem struct {
	// Text is the item text with the bullet and checkbox marker
	// stripped and surrounding whitespace trimmed. Inline markdown is
	// preserved verbatim — unlike Heading.Text, which strips it to
	// derive the anchor slug — so consumers can match Text back to the
	// raw line.
	Text string

	// Checked reports the checkbox state: false for "[ ]", true for
	// "[x]" or "[X]". List items with any other bracket content (e.g.
	// "[-]") are not task items and are omitted entirely, so a future
	// version can assign them meaning without changing what Checked
	// means for the two states it documents.
	Checked bool

	// Indent is the number of leading whitespace characters before the
	// bullet — spaces and tabs each count as one; tabs are not
	// expanded. It is a raw fact: consumers apply their own
	// top-level/nesting policy.
	Indent int

	// Line is the 1-based line number of the item in the input,
	// counted by LF. It is byte-accurate against the input, so a
	// writer can splice the checkbox at exactly this line (the
	// sibling docwrite package's write contract).
	Line int
}

// taskItemPattern matches a GitHub-style checkbox task-list item:
// optional leading whitespace, a "-" or "*" bullet, whitespace, a
// single-character checkbox, and optional whitespace-separated text.
// Following GFM, a checkbox with no whitespace before its text
// ("- [x]done") is not a task item.
var taskItemPattern = regexp.MustCompile(`^([ \t]*)[-*][ \t]+\[([ xX])\](?:[ \t]+(.*))?$`)

// TaskItems extracts every checkbox task-list item from content: a "-"
// or "*" bullet followed by "[ ]", "[x]", or "[X]" and optional item
// text. Items inside fenced code blocks are skipped using the same
// fence rule as Headings. Ordinary list items without a checkbox are
// not task items.
//
// Content is expected to use LF line endings (the docz write contract);
// under CRLF input a trailing carriage return is treated as ordinary
// text.
func TaskItems(content []byte) []TaskItem {
	lines := strings.Split(string(content), "\n")

	var items []TaskItem
	inCodeBlock := false

	for i, line := range lines {
		if isFenceToggle(line) {
			inCodeBlock = !inCodeBlock
			continue
		}

		if inCodeBlock {
			continue
		}

		matches := taskItemPattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		items = append(items, TaskItem{
			Text:    strings.TrimSpace(matches[3]),
			Checked: matches[2] == "x" || matches[2] == "X",
			Indent:  len(matches[1]),
			Line:    i + 1,
		})
	}

	return items
}
