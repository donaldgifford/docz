package docparse

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Heading is a single ATX markdown heading fact.
type Heading struct {
	// Level is the heading depth, 2 through 6. H1 headings are
	// excluded: in docz documents the H1 is the document title, which
	// is metadata rather than section structure.
	Level int

	// Text is the heading text with inline markdown (bold, italic,
	// inline code, links) and any closing sequence stripped. HTML
	// comments are kept: the built-in templates' placeholders are
	// comments in a heading, and consumers read them from here.
	Text string

	// Slug is the GitHub-compatible anchor for this heading: AnchorSlug
	// over the rendered text, which drops HTML comments and inline tags
	// but keeps the spaces around them. Duplicate slugs get "-1", "-2", …
	// suffixes in document order, matching GitHub's rendered anchors.
	// Suffix state is per Headings call, computed over the bytes given.
	Slug string

	// Line is the 1-based line number of the heading in the input,
	// counted by LF.
	Line int
}

// headingPattern matches ATX-style markdown headings (## through ######).
var headingPattern = regexp.MustCompile(`^(#{2,6})\s+(.+)$`)

// closingSequence matches an ATX heading's optional closing run of "#",
// which CommonMark requires to be preceded by a space.
var closingSequence = regexp.MustCompile(`[ \t]+#+[ \t]*$`)

// Inline markdown stripping patterns. An underscore opens or closes
// emphasis only at a word boundary (CommonMark's intraword rule), so
// "a_b_c" is text, not "a" + emphasised "b" + "c".
var (
	boldPattern       = regexp.MustCompile(`\*\*(.+?)\*\*|__(.+?)__`)
	starItalicPattern = regexp.MustCompile(`\*(.+?)\*`)
	underItalic       = regexp.MustCompile(`(^|[^\p{L}\p{N}_])_([^_]+?)_($|[^\p{L}\p{N}_])`)
	codePattern       = regexp.MustCompile("`(.+?)`")
	linkPattern       = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	commentPattern    = regexp.MustCompile(`<!--[\s\S]*?-->`)
	inlineTagPattern  = regexp.MustCompile(`</?[A-Za-z][A-Za-z0-9-]*(?:\s[^<>]*)?/?>`)
)

// AnchorSlug converts rendered heading text to the anchor GitHub gives it,
// suitable for the fragment of an in-page link (e.g.
// `[See section](#api-rate-limiting)`). The contract is github-slugger's,
// which markdownlint's MD051 and docz-site's rehype-slug implement too:
// lowercase the text, keep Unicode letters, marks, numbers, connector
// punctuation ("_"), spaces, and "-", drop everything else, and turn each
// space into "-". Nothing is trimmed, so "Phase 1: " gives "phase-1-".
//
// AnchorSlug does not apply duplicate suffixes; Headings layers those
// on top in document order.
func AnchorSlug(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-',
			unicode.IsLetter(r),
			unicode.IsMark(r),
			unicode.IsNumber(r),
			unicode.Is(unicode.Pc, r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// stripInlineMarkdown removes bold, italic, inline code, and link formatting
// from heading text, keeping the visible text content, and trims it.
func stripInlineMarkdown(text string) string {
	return strings.TrimSpace(stripInline(text))
}

// stripInline is stripInlineMarkdown without the trim, which the slug
// input needs: a heading whose text ends in a comment keeps the space
// before it, and github-slugger turns that space into a trailing "-".
func stripInline(text string) string {
	// Strip links first (before bold/italic to avoid partial matches).
	text = linkPattern.ReplaceAllString(text, "$1")

	// Strip inline code.
	text = codePattern.ReplaceAllString(text, "$1")

	// Strip bold before italic (** before *).
	text = boldPattern.ReplaceAllStringFunc(text, func(m string) string {
		sub := boldPattern.FindStringSubmatch(m)
		if sub[1] != "" {
			return sub[1]
		}
		return sub[2]
	})

	text = starItalicPattern.ReplaceAllString(text, "$1")

	// A match consumes the boundary character after it, which can be the
	// boundary the next emphasis needs, so repeat until nothing changes.
	for {
		next := underItalic.ReplaceAllString(text, "${1}${2}${3}")
		if next == text {
			return text
		}
		text = next
	}
}

// slugSource is the text github-slugger sees for a heading: its HTML
// comments and inline tags removed, with the spaces around them kept,
// and its inline markdown stripped. Inside a code span a tag is literal
// text, so "`<type>`" keeps its "type".
func slugSource(raw string) string {
	var b strings.Builder
	for raw != "" {
		loc := codePattern.FindStringIndex(raw)
		if loc == nil {
			b.WriteString(dropHTML(raw))
			break
		}
		b.WriteString(dropHTML(raw[:loc[0]]))
		b.WriteString(raw[loc[0]:loc[1]])
		raw = raw[loc[1]:]
	}
	return stripInline(b.String())
}

// dropHTML removes HTML comments and inline tags from text outside code
// spans.
func dropHTML(s string) string {
	s = commentPattern.ReplaceAllString(s, "")
	return inlineTagPattern.ReplaceAllString(s, "")
}

// isFenceToggle reports whether line opens or closes a fenced code
// block: its first non-whitespace characters are ```. Only backtick
// fences toggle — tilde (~~~) fences do not. This deliberately matches
// the historical internal ToC walker so slugs and heading sets stay
// byte-identical across the promotion (IMPL-0014 Phase 2/4).
func isFenceToggle(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "```")
}

// Headings extracts every ATX heading (## through ######, H2–H6) from
// content. H1 headings are excluded. Headings inside fenced code blocks
// and HTML comment blocks are skipped (see blockSkipper). A closing
// sequence ("## Title ##") is not part of the text.
//
// Duplicate slugs get -1, -2, … suffixes in document order, following
// github-slugger: a suffix already taken, by an earlier duplicate or by a
// heading whose own text slugs to it, is skipped, and every slug handed
// out is recorded as taken.
//
// The entire input is walked. Callers that want to exclude a region —
// as the sibling toc package does for its ToC marker block — slice the
// input first; Line values are then relative to the slice.
func Headings(content []byte) []Heading {
	lines := strings.Split(string(content), "\n")

	var headings []Heading
	var blocks blockSkipper
	taken := make(map[string]int)

	for i, line := range lines {
		if blocks.skip(line) {
			continue
		}

		matches := headingPattern.FindStringSubmatch(strings.TrimSpace(line))
		if matches == nil {
			continue
		}

		raw := closingSequence.ReplaceAllString(strings.TrimSpace(matches[2]), "")

		headings = append(headings, Heading{
			Level: len(matches[1]),
			Text:  stripInlineMarkdown(raw),
			Slug:  uniqueSlug(taken, AnchorSlug(slugSource(raw))),
			Line:  i + 1,
		})
	}

	return headings
}

// uniqueSlug is github-slugger's duplicate rule. taken maps every slug
// handed out to the count of duplicates generated from it.
func uniqueSlug(taken map[string]int, slug string) string {
	result := slug
	for {
		if _, ok := taken[result]; !ok {
			break
		}
		taken[slug]++
		result = slug + "-" + strconv.Itoa(taken[slug])
	}
	taken[result] = 0
	return result
}
