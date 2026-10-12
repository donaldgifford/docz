package docparse

import "strings"

// blockSkipper tracks the two kinds of block whose lines are never
// markdown structure: fenced code and HTML comment blocks. A walker feeds
// it every line in order and skips the ones it reports.
//
// A comment block is CommonMark's HTML block type 2: a line whose first
// non-whitespace characters are "<!--" opens one when no "-->" follows on
// that line, and the block ends on the first line containing "-->". The
// fence test runs first, so "<!--" inside a fence opens nothing, and a
// single-line comment — every <!--docz:…--> marker among them — never
// opens a block (issue #96).
type blockSkipper struct {
	inFence   bool
	inComment bool
}

// skip reports whether line belongs to a fence or a comment block,
// including the lines that open and close one, and advances the state.
func (b *blockSkipper) skip(line string) bool {
	if b.inComment {
		if strings.Contains(line, "-->") {
			b.inComment = false
		}
		return true
	}

	if isFenceToggle(line) {
		b.inFence = !b.inFence
		return true
	}
	if b.inFence {
		return true
	}

	if opensCommentBlock(line) {
		b.inComment = true
		return true
	}

	return false
}

// opensCommentBlock reports whether line starts an HTML comment that does
// not close on the same line.
func opensCommentBlock(line string) bool {
	rest, ok := strings.CutPrefix(strings.TrimLeft(line, " \t"), "<!--")
	return ok && !strings.Contains(rest, "-->")
}
