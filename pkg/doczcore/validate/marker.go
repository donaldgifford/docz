package validate

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

// markerShaped matches a marker line without caring whether the walker
// accepted it, so a marker docz ignored can still be reported. Used only
// for the in-fence check: inside a fence the walker sees text, and the
// author almost certainly did not mean to write one there.
var markerShaped = regexp.MustCompile(
	`^[ \t]*<!--[ \t]*docz[ \t]*:[ \t]*[a-zA-Z][a-zA-Z0-9-]*[ \t]*:[ \t]*(?:start|end)[ \t]*-->[ \t]*$`,
)

// checkMarkers reports the marker-level findings: an end with no start, a
// start never closed, a lenient spelling, and a marker inside a fence.
//
// Spelling is a warning, not an error, and the marker is still read. That
// is the INV-0009 lesson applied on day one: a marker with a stray space
// used to make docz skip the file and say nothing, so the file silently
// stopped being maintained. Reading it and saying so is strictly better
// than either refusing it or ignoring the difference.
func checkMarkers(content []byte) []Finding {
	var out []Finding

	open := make(map[string][]int)

	var stack []string

	for _, m := range docparse.Markers(content) {
		if !m.Canonical {
			out = append(out, Finding{
				Code:     CodeMarkerSpelling,
				Severity: Warning,
				Line:     m.Line,
				Kind:     m.Kind,
				Detail: fmt.Sprintf("marker is not in the canonical spelling; docz writes %s",
					canonicalSpelling(m)),
			})
		}

		if m.Role == docparse.Start {
			open[m.Kind] = append(open[m.Kind], m.Line)
			stack = append(stack, m.Kind)

			continue
		}

		lines := open[m.Kind]
		if len(lines) == 0 {
			out = append(out, Finding{
				Code:     "marker.stray-end",
				Severity: Error,
				Line:     m.Line,
				Kind:     m.Kind,
				Detail:   fmt.Sprintf("end marker for %q has no start", m.Kind),
			})

			continue
		}

		open[m.Kind] = lines[:len(lines)-1]

		// The innermost open region should be the one closing. Anything else
		// means the pairs are interleaved rather than nested, which the
		// walker resolves by cutting the inner one short.
		if len(stack) > 0 {
			stack = stack[:len(stack)-1]
		}
	}

	for kind, lines := range open {
		for _, line := range lines {
			out = append(out, Finding{
				Code:     "marker.unclosed",
				Severity: Error,
				Line:     line,
				Kind:     kind,
				Detail:   fmt.Sprintf("start marker for %q is never closed", kind),
			})
		}
	}

	return append(out, checkMarkersInFences(content)...)
}

// canonicalSpelling renders the marker docz would write in place of a
// lenient one, so the finding tells the author what to change it to.
func canonicalSpelling(m docparse.Marker) string {
	role := "start"
	if m.Role == docparse.End {
		role = "end"
	}

	if m.Kind == docparse.TocKind || m.Kind == docparse.IndexKind {
		// These two keep their legacy spelling everywhere (DESIGN-0015 §1),
		// so the canonical form of a lenient ToC marker is the legacy pair,
		// not a docz: one.
		return fmt.Sprintf("<!--%s:%s-->", m.Kind, role)
	}

	return fmt.Sprintf("<!--docz:%s:%s-->", m.Kind, role)
}

// checkMarkersInFences reports marker-shaped lines inside fenced code
// blocks, as one finding naming the first.
//
// One finding, not one per line, because the common case is a document
// explaining markers. DESIGN-0015 shows the skeletons in fenced blocks and
// has twenty-six such lines; reporting each would bury every other finding
// in the document and teach a reader to ignore the family. The walker is
// right to read them as text — a document must be able to show what a marker
// looks like — so this is a warning whose only job is to catch the other
// reading, an author who fenced a region by accident.
func checkMarkersInFences(content []byte) []Finding {
	inFence := false

	count, first := 0, 0

	for i, line := range strings.Split(string(content), "\n") {
		if isFenceToggle(line) {
			inFence = !inFence

			continue
		}

		if inFence && markerShaped.MatchString(line) {
			count++

			if first == 0 {
				first = i + 1
			}
		}
	}

	if count == 0 {
		return nil
	}

	detail := "marker inside a fenced block is text, not a region"
	if count > 1 {
		detail = fmt.Sprintf("%d marker-shaped lines inside fenced blocks are text, not regions",
			count)
	}

	return []Finding{{
		Code:     "marker.in-fence",
		Severity: Warning,
		Line:     first,
		Detail:   detail,
	}}
}

// isFenceToggle applies the module's fence rule: a trimmed line opening
// with three backticks toggles a fenced block. Duplicated from docparse
// rather than imported, the same way document.ParseChangelog duplicates it,
// so neither package's frozen behaviour is coupled to the other's.
func isFenceToggle(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "```")
}
