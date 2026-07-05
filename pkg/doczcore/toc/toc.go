// Package toc generates markdown tables of contents and splices them
// between <!--toc:start--> and <!--toc:end--> markers. It owns only the
// splice concern: every heading walk delegates to the sibling docparse
// package (ADR-0001 — one heading walker in the public API), and the
// generated entries link to docparse's GitHub-compatible anchor slugs.
package toc

import (
	"strings"

	"github.com/donaldgifford/docz/pkg/doczcore/docparse"
)

// Markers used to delimit the ToC region in a document.
const (
	BeginMarker = "<!--toc:start-->"
	EndMarker   = "<!--toc:end-->"
)

// parseHeadings walks the headings that belong in a document's ToC:
// everything after the first EndMarker line (so the ToC region and the
// preamble above it are excluded), or the whole document when no marker
// is present. The walk itself is docparse.Headings; only the slice
// point is toc policy.
func parseHeadings(content string) []docparse.Heading {
	off := 0
	for off < len(content) {
		next := len(content)
		line := content[off:]
		if i := strings.IndexByte(line, '\n'); i >= 0 {
			line = line[:i]
			next = off + i + 1
		}
		if strings.TrimSpace(line) == EndMarker {
			return docparse.Headings([]byte(content[next:]))
		}
		off = next
	}
	return docparse.Headings([]byte(content))
}

// GenerateToC builds a markdown table of contents from headings. It uses
// relative indentation based on the shallowest heading level found, with
// 2-space indent per level. Returns an empty string if the number of headings
// is below minHeadings.
func GenerateToC(headings []docparse.Heading, minHeadings int) string {
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

// UpdateResult is what UpdateToC returns: the updated content, the
// parsed headings (so callers don't have to walk the document a second
// time), and whether the ToC markers were found in the input.
//
// Headings is the same slice UpdateToC used internally to build the
// ToC. When Found is false, both Updated and Headings reflect the
// original input (Updated == content; Headings is nil).
type UpdateResult struct {
	Updated  string
	Headings []docparse.Heading
	Found    bool
}

// UpdateToC replaces the content between ToC markers in a document with
// a freshly generated table of contents. If the markers are not present
// the input is returned untouched with Found=false.
//
// Only headings after the EndMarker line are included — the ToC never
// lists its own region or the title block above it. The parsed headings
// are surfaced via UpdateResult.Headings so callers that need the
// metadata (notably the `docz update --dry-run` summary) can read them
// directly instead of walking the same content again — see IMPL-0007
// Phase 4 / Decisions §5.
func UpdateToC(content string, minHeadings int) UpdateResult {
	before, afterBegin, foundBegin := strings.Cut(content, BeginMarker)
	if !foundBegin {
		return UpdateResult{Updated: content}
	}

	_, afterEnd, foundEnd := strings.Cut(afterBegin, EndMarker)
	if !foundEnd {
		return UpdateResult{Updated: content}
	}

	headings := parseHeadings(content)
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

	return UpdateResult{
		Updated:  sb.String(),
		Headings: headings,
		Found:    true,
	}
}
