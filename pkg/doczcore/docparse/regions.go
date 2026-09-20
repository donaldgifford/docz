package docparse

import (
	"regexp"
	"strings"
)

// Role distinguishes the two ends of a region marker.
type Role int

const (
	// Start opens a region.
	Start Role = iota + 1
	// End closes the innermost open region of the same kind.
	End
)

// String returns the role as it is spelled in a marker.
func (r Role) String() string {
	switch r {
	case Start:
		return "start"
	case End:
		return "end"
	default:
		return "unknown"
	}
}

// Marker is every marker line the walker recognizes, in document order,
// including stray and non-canonical ones. Validation reasons over these
// to explain why a region is missing or malformed; a consumer that only
// wants the spans reads Regions instead.
type Marker struct {
	// Kind is the region kind the marker names, lower-cased. The legacy
	// ToC pair reports "toc" and the README index pair reports "index",
	// so one vocabulary covers every span docz owns.
	Kind string

	// Role is Start or End.
	Role Role

	// Line is the 1-based line number of the marker, counted by LF.
	Line int

	// Canonical is false for a lenient-spelling match: a marker docz
	// read but would rewrite. INV-0009 Finding 4 is why leniency exists
	// at all — a marker spelled with a stray space used to make docz
	// skip the file and say nothing.
	Canonical bool
}

// Region is a paired start and end. Depth is 0 at the top level.
type Region struct {
	// Kind is the region kind, lower-cased.
	Kind string

	// Start is the line of the start marker, End the line of the end
	// marker. Both are 1-based and inclusive of the marker lines, so the
	// content of a region is the lines strictly between them.
	Start int
	End   int

	// Depth is 0 at the top level and one more for each enclosing region.
	Depth int

	// Closed is false when no end marker paired with the start: the
	// region was closed by end of file, or by an enclosing region's end
	// marker arriving first. End then holds the line it was cut off at,
	// so a span is always usable even when the document is malformed.
	Closed bool
}

// Marker spellings. The canonical form has no interior whitespace and is
// what the fixer writes; the lenient pattern is what the reader accepts.
const (
	canonicalPrefix = "<!--docz:"
	canonicalSuffix = "-->"

	legacyTocStart = "<!--toc:start-->"
	legacyTocEnd   = "<!--toc:end-->"

	legacyIndexStart = "<!-- BEGIN DOCZ AUTO-GENERATED -->"
	legacyIndexEnd   = "<!-- END DOCZ AUTO-GENERATED -->"

	// TocKind is the kind the legacy ToC pair reports.
	TocKind = "toc"
	// IndexKind is the kind the README index pair reports.
	IndexKind = "index"
)

// doczMarkerPattern matches a docz region marker leniently: whitespace is
// allowed after the comment opener, around the docz token and the colons,
// and before the closer. A kind is [a-z][a-z0-9-]*.
var doczMarkerPattern = regexp.MustCompile(
	`^<!--[ \t]*docz[ \t]*:[ \t]*([a-z][a-z0-9-]*)[ \t]*:[ \t]*(start|end)[ \t]*-->$`,
)

// legacyTocPattern and legacyIndexPattern accept the same whitespace
// leniency for the two pairs that predate the docz: namespace.
var (
	legacyTocPattern = regexp.MustCompile(
		`^<!--[ \t]*toc[ \t]*:[ \t]*(start|end)[ \t]*-->$`,
	)
	legacyIndexPattern = regexp.MustCompile(
		`^<!--[ \t]*(BEGIN|END)[ \t]+DOCZ[ \t]+AUTO-GENERATED[ \t]*-->$`,
	)
)

// Markers extracts every region marker from content, in document order,
// including stray end markers and non-canonical spellings. A marker
// inside a fenced code block is text, under the same fence rule as
// Headings and TaskItems.
//
// The whole trimmed line must be the marker: trailing text makes it not a
// marker, which is what lets a document discuss markers in prose.
//
// Lines are counted by LF and a trailing carriage return is trimmed with
// the rest of the surrounding whitespace, so a CRLF document's markers are
// still found. That is deliberate: refusing CRLF is the write side's job
// (docwrite returns ErrUnsupportedLineEndings) and a reader that silently
// saw no markers would report the file as unstructured instead.
func Markers(content []byte) []Marker {
	lines := strings.Split(string(content), "\n")

	var out []Marker

	inCodeBlock := false

	for i, line := range lines {
		if isFenceToggle(line) {
			inCodeBlock = !inCodeBlock

			continue
		}

		if inCodeBlock {
			continue
		}

		if m, ok := parseMarker(line, i+1); ok {
			out = append(out, m)
		}
	}

	return out
}

// parseMarker recognizes one line. The canonical check is a string
// comparison against the spelling the fixer writes, so "canonical" can
// never drift from what canonicalization produces.
func parseMarker(line string, num int) (Marker, bool) {
	trimmed := strings.TrimSpace(line)

	if !strings.HasPrefix(trimmed, "<!--") {
		return Marker{}, false
	}

	if m := doczMarkerPattern.FindStringSubmatch(trimmed); m != nil {
		kind, role := m[1], roleOf(m[2])

		return Marker{
			Kind:      kind,
			Role:      role,
			Line:      num,
			Canonical: trimmed == canonicalPrefix+kind+":"+role.String()+canonicalSuffix,
		}, true
	}

	if m := legacyTocPattern.FindStringSubmatch(trimmed); m != nil {
		role := roleOf(m[1])
		canonical := trimmed == legacyTocStart
		if role == End {
			canonical = trimmed == legacyTocEnd
		}

		return Marker{Kind: TocKind, Role: role, Line: num, Canonical: canonical}, true
	}

	if m := legacyIndexPattern.FindStringSubmatch(trimmed); m != nil {
		role := Start
		canonical := trimmed == legacyIndexStart

		if m[1] == "END" {
			role = End
			canonical = trimmed == legacyIndexEnd
		}

		return Marker{Kind: IndexKind, Role: role, Line: num, Canonical: canonical}, true
	}

	return Marker{}, false
}

func roleOf(s string) Role {
	if s == "end" || s == "END" {
		return End
	}

	return Start
}

// openRegion is one entry of the nesting stack.
type openRegion struct {
	kind  string
	start int
}

// Regions pairs the markers Markers reports into spans. Nesting is a
// stack: an end marker closes the innermost open region of the same kind,
// an end marker with no matching open region is stray and yields no
// region, and a region still open at end of file ends at the last line
// with Closed false.
//
// Regions come back in document order by start line. A kind may repeat at
// any depth — the IMPL template's phases do — and whether a repeat is
// allowed is the validator's question, not the walker's.
func Regions(content []byte) []Region {
	markers := Markers(content)
	if len(markers) == 0 {
		return nil
	}

	lastLine := strings.Count(string(content), "\n") + 1

	var (
		out   []Region
		stack []openRegion
	)

	for _, m := range markers {
		if m.Role == Start {
			stack = append(stack, openRegion{kind: m.Kind, start: m.Line})

			continue
		}

		at := innermost(stack, m.Kind)
		if at < 0 {
			// Stray end: reported by Markers, never a region.
			continue
		}

		// Anything opened inside the region being closed never got its own
		// end marker. Each is emitted unclosed at this line rather than
		// dropped, so a malformed document still yields usable spans.
		for i := len(stack) - 1; i > at; i-- {
			out = append(out, Region{
				Kind:   stack[i].kind,
				Start:  stack[i].start,
				End:    m.Line,
				Depth:  i,
				Closed: false,
			})
		}

		out = append(out, Region{
			Kind:   stack[at].kind,
			Start:  stack[at].start,
			End:    m.Line,
			Depth:  at,
			Closed: true,
		})

		stack = stack[:at]
	}

	for i := len(stack) - 1; i >= 0; i-- {
		out = append(out, Region{
			Kind:   stack[i].kind,
			Start:  stack[i].start,
			End:    lastLine,
			Depth:  i,
			Closed: false,
		})
	}

	sortRegions(out)

	return out
}

// innermost returns the index of the deepest open region of kind, or -1.
func innermost(stack []openRegion, kind string) int {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].kind == kind {
			return i
		}
	}

	return -1
}

// sortRegions puts regions in document order: by start line, then
// outermost first so a parent precedes a child that opens on the next
// line. Insertion sort keeps it allocation-free and stable, and a
// document has tens of regions, not thousands.
func sortRegions(rs []Region) {
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0; j-- {
			if rs[j-1].Start < rs[j].Start ||
				(rs[j-1].Start == rs[j].Start && rs[j-1].Depth <= rs[j].Depth) {
				break
			}

			rs[j-1], rs[j] = rs[j], rs[j-1]
		}
	}
}
