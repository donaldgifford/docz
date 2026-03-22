// Package toc provides table of contents generation for markdown documents.
// It parses headings, generates GitHub-compatible anchor slugs, and splices
// the resulting ToC between <!--toc:start--> and <!--toc:end--> markers.
package toc

import (
	"regexp"
	"strings"
)

// Markers used to delimit the ToC region in a document.
const (
	BeginMarker = "<!--toc:start-->"
	EndMarker   = "<!--toc:end-->"
)

// Heading represents a parsed markdown heading.
type Heading struct {
	Level int    // 2-6 (H1 is excluded)
	Text  string // heading text with inline markdown stripped
	Slug  string // GitHub-compatible anchor
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

// Slugify converts heading text to a GitHub-compatible anchor slug.
// It lowercases the text, keeps only letters, digits, spaces, and hyphens,
// replaces spaces with hyphens, collapses multiple hyphens, and trims
// leading/trailing hyphens.
func Slugify(text string) string {
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

// ParseHeadings extracts headings from markdown content. Only headings after
// the ToC end marker are included. H1 headings are excluded. Headings inside
// fenced code blocks are skipped. Duplicate slugs get -1, -2 suffixes matching
// GitHub behavior.
func ParseHeadings(content string) []Heading {
	lines := strings.Split(content, "\n")

	// Find the end marker line index. If no end marker, parse from the start.
	startIdx := 0
	for i, line := range lines {
		if strings.TrimSpace(line) == EndMarker {
			startIdx = i + 1
			break
		}
	}

	var headings []Heading
	inCodeBlock := false
	slugCounts := make(map[string]int)

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Track fenced code blocks.
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}

		if inCodeBlock {
			continue
		}

		matches := headingPattern.FindStringSubmatch(trimmed)
		if matches == nil {
			continue
		}

		level := len(matches[1])
		text := stripInlineMarkdown(strings.TrimSpace(matches[2]))
		slug := Slugify(text)

		// Apply duplicate suffix.
		slugCounts[slug]++
		if slugCounts[slug] > 1 {
			slug = slug + "-" + itoa(slugCounts[slug]-1)
		}

		headings = append(headings, Heading{
			Level: level,
			Text:  text,
			Slug:  slug,
		})
	}

	return headings
}

// GenerateToC builds a markdown table of contents from headings. It uses
// relative indentation based on the shallowest heading level found, with
// 2-space indent per level. Returns an empty string if the number of headings
// is below minHeadings.
func GenerateToC(headings []Heading, minHeadings int) string {
	if len(headings) < minHeadings {
		return ""
	}

	// Find the minimum heading level for relative indentation.
	minLevel := headings[0].Level
	for _, h := range headings[1:] {
		if h.Level < minLevel {
			minLevel = h.Level
		}
	}

	var sb strings.Builder
	for _, h := range headings {
		indent := strings.Repeat("  ", h.Level-minLevel)
		sb.WriteString(indent)
		sb.WriteString("- [")
		sb.WriteString(h.Text)
		sb.WriteString("](#")
		sb.WriteString(h.Slug)
		sb.WriteString(")\n")
	}

	return sb.String()
}

// UpdateToC replaces the content between ToC markers in a document with a
// freshly generated table of contents. Returns the updated content and true
// if markers were found. If no markers are found, returns the original content
// and false.
func UpdateToC(content string, minHeadings int) (string, bool) {
	before, afterBegin, foundBegin := strings.Cut(content, BeginMarker)
	if !foundBegin {
		return content, false
	}

	_, afterEnd, foundEnd := strings.Cut(afterBegin, EndMarker)
	if !foundEnd {
		return content, false
	}

	headings := ParseHeadings(content)
	toc := GenerateToC(headings, minHeadings)

	var sb strings.Builder
	sb.WriteString(before)
	sb.WriteString(BeginMarker)
	sb.WriteString("\n")
	if toc != "" {
		sb.WriteString(toc)
	}
	sb.WriteString(EndMarker)
	sb.WriteString(afterEnd)

	return sb.String(), true
}

// itoa converts a small integer to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
