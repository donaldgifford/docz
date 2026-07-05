package docparse

import (
	"regexp"
	"strconv"
	"strings"
)

// Heading is a single ATX markdown heading fact.
type Heading struct {
	// Level is the heading depth, 2 through 6. H1 headings are
	// excluded: in docz documents the H1 is the document title, which
	// is metadata rather than section structure.
	Level int

	// Text is the heading text with inline markdown (bold, italic,
	// inline code, links) stripped — the visible text GitHub derives
	// the anchor from.
	Text string

	// Slug is the GitHub-compatible anchor for this heading, derived
	// from Text via AnchorSlug. Duplicate slugs get "-1", "-2", …
	// suffixes in document order, matching GitHub's rendered anchors.
	// Suffix state is per Headings call, computed over the bytes given.
	Slug string

	// Line is the 1-based line number of the heading in the input,
	// counted by LF.
	Line int
}

// headingPattern matches ATX-style markdown headings (## through ######).
var headingPattern = regexp.MustCompile(`^(#{2,6})\s+(.+)$`)

// Inline markdown stripping patterns.
var (
	boldPattern   = regexp.MustCompile(`\*\*(.+?)\*\*|__(.+?)__`)
	italicPattern = regexp.MustCompile(`\*(.+?)\*|_(.+?)_`)
	codePattern   = regexp.MustCompile("`(.+?)`")
	linkPattern   = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
)

// AnchorSlug converts heading text to a GitHub-compatible anchor slug
// suitable for the fragment portion of an in-page link (e.g.
// `[See section](#api-rate-limiting)`). It mirrors the algorithm GitHub
// uses for auto-generated heading anchors: lowercase the text, keep
// only ASCII letters/digits/spaces/hyphens, replace spaces with
// hyphens, and trim leading/trailing hyphens.
//
// AnchorSlug does not apply duplicate suffixes; Headings layers those
// on top in document order.
func AnchorSlug(text string) string {
	s := strings.ToLower(text)

	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= '0' && r <= '9',
			r == ' ',
			r == '-':
			b.WriteRune(r)
		}
	}
	s = b.String()

	s = strings.ReplaceAll(s, " ", "-")
	s = strings.Trim(s, "-")
	return s
}

// stripInlineMarkdown removes bold, italic, inline code, and link formatting
// from heading text, keeping the visible text content.
func stripInlineMarkdown(text string) string {
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

	text = italicPattern.ReplaceAllStringFunc(text, func(m string) string {
		sub := italicPattern.FindStringSubmatch(m)
		if sub[1] != "" {
			return sub[1]
		}
		return sub[2]
	})

	return strings.TrimSpace(text)
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
// are skipped (see isFenceToggle for the fence rule). Duplicate slugs
// get -1, -2, … suffixes matching GitHub behavior.
//
// The entire input is walked. Callers that want to exclude a region —
// as the sibling toc package does for its ToC marker block — slice the
// input first; Line values are then relative to the slice.
func Headings(content []byte) []Heading {
	lines := strings.Split(string(content), "\n")

	var headings []Heading
	inCodeBlock := false
	slugCounts := make(map[string]int)

	for i, line := range lines {
		if isFenceToggle(line) {
			inCodeBlock = !inCodeBlock
			continue
		}

		if inCodeBlock {
			continue
		}

		matches := headingPattern.FindStringSubmatch(strings.TrimSpace(line))
		if matches == nil {
			continue
		}

		level := len(matches[1])
		text := stripInlineMarkdown(strings.TrimSpace(matches[2]))
		slug := AnchorSlug(text)

		// Apply duplicate suffix.
		slugCounts[slug]++
		if slugCounts[slug] > 1 {
			slug = slug + "-" + strconv.Itoa(slugCounts[slug]-1)
		}

		headings = append(headings, Heading{
			Level: level,
			Text:  text,
			Slug:  slug,
			Line:  i + 1,
		})
	}

	return headings
}
