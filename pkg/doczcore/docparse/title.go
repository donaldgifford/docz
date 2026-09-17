package docparse

import (
	"regexp"
	"strings"
)

// titlePattern matches an ATX-style H1. Requiring whitespace right after
// the single "#" is what keeps it to level 1: in "## Two" the next
// character is another "#", so the match fails. It is the exact
// complement of headingPattern on hash count.
var titlePattern = regexp.MustCompile(`^#\s+(.+)$`)

// setextPattern matches a setext H1 underline: a line of nothing but "="
// characters.
var setextPattern = regexp.MustCompile(`^=+$`)

// blockMarkerPattern matches the leading characters that make a line
// something other than a paragraph — an ATX heading, a blockquote, a
// bullet or ordered list item, a table row, or raw HTML. None of these
// can be the text of a setext heading, and without the check a line of
// "=" under any of them would promote it to the document's title.
var blockMarkerPattern = regexp.MustCompile(`^(#|>|[-*+]\s|\d+[.)]\s|\||<)`)

// Title returns the document's title: the text of its first H1, with
// inline markdown stripped. It returns "" when the document has no H1.
//
// The empty string is a normal outcome, not an error — plenty of
// markdown has no H1 at all. It is also what an H1 whose entire content
// is markup ("# _ _") reduces to, so "" means "no title to show" rather
// than strictly "no H1 present". A consumer that needs something to
// display supplies its own fallback, typically the title-cased filename.
// This is why there is no error return, matching the rest of the package.
//
// Title exists because the H1 is the only title signal a markdown file
// without frontmatter carries, and consumers reading a repo's
// CONTRIBUTING.md or docs/examples/README.md have nothing else to show
// (DESIGN-0011, INV-0007 F2). Documents docz itself writes carry the
// title in frontmatter; read that instead, via the document package.
//
// The rules, each of which differs deliberately from Headings:
//
//   - H1 only. Headings excludes H1 for the opposite reason — there, the
//     title is metadata polluting a table of contents; here it is the
//     whole point.
//   - Setext H1 counts: a paragraph line underlined by a line of only
//     "=". Headings has no setext support and gains none. Title reads
//     markdown docz did not write, where setext titles are common, and a
//     CONTRIBUTING.md written that way should not fall back to its
//     filename. A setext heading spanning several lines yields only the
//     last of them.
//   - A leading YAML frontmatter block is skipped. A "---" appearing
//     later is a horizontal rule and is treated as ordinary content.
//
// Shared with Headings: an H1 inside a fenced code block is skipped
// under the same fence rule (a trimmed line starting with ``` toggles;
// ~~~ does not), leading whitespace is trimmed before matching, and the
// text is run through the same inline-markdown stripping, so
// "# **Bold** Title" yields "Bold Title". Also shared: indented code
// blocks are not modeled, so a four-space-indented "# x" is a heading to
// both walkers.
//
// The result is a single line — it can never contain "\n" — but it is
// otherwise the document's bytes as written. Markdown from an untrusted
// repo can therefore put control characters in it; a consumer rendering
// the result is responsible for escaping it, as it would be for any
// other document text.
//
// The first H1 wins; later ones are ignored.
func Title(content []byte) string {
	lines := skipFrontmatter(strings.Split(string(content), "\n"))

	inCodeBlock := false
	for i, line := range lines {
		if isFenceToggle(line) {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}

		trimmed := strings.TrimSpace(line)
		if matches := titlePattern.FindStringSubmatch(trimmed); matches != nil {
			return stripInlineMarkdown(strings.TrimSpace(matches[1]))
		}

		if isSetextTitle(trimmed, lines, i) {
			return stripInlineMarkdown(trimmed)
		}
	}

	return ""
}

// isSetextTitle reports whether the line at index i is a setext H1: a
// paragraph line whose successor is nothing but "=".
func isSetextTitle(trimmed string, lines []string, i int) bool {
	if !isSetextCandidate(trimmed) || i+1 >= len(lines) {
		return false
	}

	return setextPattern.MatchString(strings.TrimSpace(lines[i+1]))
}

// isSetextCandidate reports whether trimmed could be the text of a setext
// heading — that is, whether it is a paragraph.
//
// The setext branch is the only one that can return arbitrary line
// content: the ATX branch at least guarantees the line looked like a
// heading. Without this check "- alpha" underlined by "===" becomes a
// title with its own bullet marker in it, and so does a blockquote, a
// table row, or a run of HTML. A line that is itself punctuation —
// "---", "===", "***" — is a thematic break or an underline rather than
// text, and is excluded for the same reason.
func isSetextCandidate(trimmed string) bool {
	if trimmed == "" || blockMarkerPattern.MatchString(trimmed) {
		return false
	}

	return strings.TrimLeft(trimmed, "-*_=") != ""
}

// skipFrontmatter returns lines with a leading YAML frontmatter block
// removed, or unchanged when the document does not open with one.
//
// "Opens with one" follows document.ParseFrontmatter: leading blank
// lines are ignored, and the "---" delimiters — both of them — must sit
// at column 0. That column rule is what keeps an indented "---" inside a
// YAML block scalar from ending the block early and exposing the rest of
// the frontmatter as body text, which would let a document's title be
// whatever string a key happens to hold.
//
// An opener whose next line is blank is read as a thematic break rather
// than frontmatter, since real frontmatter opens with a key. Otherwise a
// document that begins with a rule and closes with another would have
// everything between them — including its title — swallowed.
//
// An unterminated block is treated as no block at all. That is the
// reading that degrades gracefully: a document whose leading "---" is
// really a rule still gets its title, and a genuinely truncated
// frontmatter block has no H1 to find either way.
func skipFrontmatter(lines []string) []string {
	start := 0
	for start < len(lines) && isBlankLine(lines[start]) {
		start++
	}

	if start >= len(lines) || !isFrontmatterDelimiter(lines[start]) {
		return lines
	}
	if start+1 >= len(lines) || isBlankLine(lines[start+1]) {
		return lines
	}

	for i := start + 1; i < len(lines); i++ {
		if isFrontmatterDelimiter(lines[i]) {
			return lines[i+1:]
		}
	}

	return lines
}

// isFrontmatterDelimiter reports whether line is a frontmatter "---"
// delimiter: the three dashes at column 0, with nothing after them but
// trailing whitespace.
func isFrontmatterDelimiter(line string) bool {
	return strings.TrimRight(line, " \t\r") == "---"
}

// isBlankLine reports whether line is empty once a CRLF carriage return
// is discounted, matching the leading-newline trim
// document.ParseFrontmatter applies before looking for its delimiter.
func isBlankLine(line string) bool {
	return strings.Trim(line, "\r") == ""
}
