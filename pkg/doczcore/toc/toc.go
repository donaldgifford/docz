// Package toc generates markdown tables of contents and splices them
// between <!--toc:start--> and <!--toc:end--> markers. It owns only the
// splice concern: every heading walk delegates to the sibling docparse
// package (ADR-0001 — one heading walker in the public API), and the
// generated entries link to docparse's GitHub-compatible anchor slugs.
package toc

import (
	"bytes"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

// Markers used to delimit the ToC region in a document.
const (
	BeginMarker = "<!--toc:start-->"
	EndMarker   = "<!--toc:end-->"
)

// parseHeadings walks the headings that belong in a document's ToC:
// everything after the ToC region ends (so the ToC itself and the
// preamble above it are excluded), or the whole document when there is no
// ToC region. The walk itself is docparse.Headings and the region comes
// from docparse.Regions; only the slice point is toc policy.
//
// Locating the region through the walker rather than scanning for the
// literal end marker means this agrees with the rest of docz about what a
// marker is: a lenient spelling is found and canonicalized on write, and
// a marker inside a fenced block is text, so a document that shows a ToC
// pair in an example no longer loses every heading above it.
// tocEnd is the 1-based line of the document's ToC end marker, or 0 when
// the document has no closed ToC region.
func parseHeadings(content []byte, tocEnd int) []docparse.Heading {
	if tocEnd <= 0 {
		return docparse.Headings(content)
	}

	return docparse.Headings(linesFrom(content, tocEnd+1))
}

// linesFrom returns content from the start of the given 1-based line. An
// out-of-range line yields empty, which is what a ToC region ending on
// the last line should produce.
func linesFrom(content []byte, line int) []byte {
	if line <= 1 {
		return content
	}

	off := 0

	for n := 1; n < line; n++ {
		i := bytes.IndexByte(content[off:], '\n')
		if i < 0 {
			return nil
		}

		off += i + 1
	}

	return content[off:]
}

// GenerateToC builds a markdown table of contents from headings. It uses
// relative indentation based on the shallowest heading level found, with
// 2-space indent per level. Returns an empty string if the number of headings
// is below minHeadings.
func GenerateToC(headings []docparse.Heading, minHeadings int) string {
	// The explicit empty check matters when minHeadings <= 0: the
	// threshold guard alone would fall through to headings[0] below.
	if len(headings) == 0 || len(headings) < minHeadings {
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
	b := []byte(content)

	region, ok := tocRegion(b)
	if !ok {
		return UpdateResult{Updated: content}
	}

	headings := parseHeadings(b, region.End)
	toc := GenerateToC(headings, minHeadings)

	var sb strings.Builder

	sb.Write(linesUpTo(b, region.Start))
	sb.WriteString(BeginMarker)
	sb.WriteString("\n")

	if toc != "" {
		sb.WriteString(toc)
	}

	sb.WriteString(EndMarker)
	sb.Write(fromLineEnd(b, region.End))

	return UpdateResult{
		Updated:  sb.String(),
		Headings: headings,
		Found:    true,
	}
}

// tocRegion returns the document's first closed ToC region. An unclosed
// one is not a splice target: without an end marker there is no span to
// replace, and writing one would swallow the rest of the document.
func tocRegion(content []byte) (docparse.Region, bool) {
	for _, r := range docparse.Regions(content) {
		if r.Kind == docparse.TocKind {
			return r, r.Closed
		}
	}

	return docparse.Region{}, false
}

// linesUpTo returns everything before the given 1-based line.
func linesUpTo(content []byte, line int) []byte {
	off := 0

	for n := 1; n < line; n++ {
		i := bytes.IndexByte(content[off:], '\n')
		if i < 0 {
			return content
		}

		off += i + 1
	}

	return content[:off]
}

// fromLineEnd returns content from the newline that terminates the given
// 1-based line, inclusive, so the caller's own text is followed by the
// document's original line break. Empty when that line is the last.
func fromLineEnd(content []byte, line int) []byte {
	off := 0

	for n := 1; n < line; n++ {
		i := bytes.IndexByte(content[off:], '\n')
		if i < 0 {
			return nil
		}

		off += i + 1
	}

	i := bytes.IndexByte(content[off:], '\n')
	if i < 0 {
		return nil
	}

	return content[off+i:]
}
