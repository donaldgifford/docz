package kinds

import (
	"regexp"
	"sort"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

// HeadingRule maps one heading shape to a region kind.
type HeadingRule struct {
	// Kind is the region kind a matching heading opens.
	Kind string

	// Level is the heading level the rule matches, 2 through 6.
	Level int

	// Text is an exact match, folded: trimmed, lower-cased, inline
	// markdown and HTML comments stripped. Empty when the rule matches by
	// Prefix instead.
	Text string

	// Prefix matches a heading that names a token — the IMPL template's
	// "### Phase 1: <!-- Foundation -->" generalises to the prefix
	// "phase", which matches "Phase 1: Foundation", "Phase 2:", and
	// "Phase A: Groundwork" alike. A rule sets Text or Prefix, never both.
	Prefix string

	// Parent is the kind a matching heading nests under, or "" at the top
	// level. A rule with a parent only matches inside that parent's span.
	Parent string
}

// HeadingSpec is the heading-to-kind map for one document type.
//
// A spec comes either from a marked template through SpecFromTemplate or
// from a type package's own table, which is the same data pinned to the
// embedded template by a test. The second path is why a type package needs
// to import nothing above the facts layer to read an unmarked document.
type HeadingSpec []HeadingRule

// sharedDefaults are the kinds any type may carry whether or not its
// template ships them, so a document that grew a References or Open
// Questions section by hand still has it read.
var sharedDefaults = HeadingSpec{
	{Kind: "references", Level: 2, Text: "references"},
	{Kind: "open-questions", Level: 2, Text: "open questions"},
	{Kind: "decisions", Level: 2, Text: "decisions"},
}

// placeholderHeading matches a template heading that names a token it
// expects the author to replace: everything before the last space, then a
// token, then a colon.
var placeholderHeading = regexp.MustCompile(`^(.*\S)\s+\S+:\s*$`)

// SpecFromTemplate derives a heading spec from a template that carries
// region markers: the first heading inside each region becomes that kind's
// rule, and an enclosing region becomes the rule's parent.
//
// A heading whose text ends in a placeholder comment generalises to a
// prefix rule. The IMPL template's "### Phase 1: <!-- Foundation -->" must
// match a document's "### Phase 3: CI Readiness", so the number is not
// part of the rule.
//
// The shared kinds' defaults are always added, so a template that ships no
// References section does not stop a document that has one from being read.
// A rule the template already provides for a kind wins, because the
// template is the type's own spelling of it.
func SpecFromTemplate(tmpl []byte) HeadingSpec {
	regions := docparse.Regions(tmpl)
	heads := docparse.Headings(tmpl)

	var spec HeadingSpec

	seen := make(map[string]bool, len(regions))

	for i, r := range regions {
		if seen[r.Kind] {
			continue
		}

		h, ok := firstHeadingIn(heads, r)
		if !ok {
			continue
		}

		seen[r.Kind] = true

		rule := HeadingRule{Kind: r.Kind, Level: h.Level, Parent: parentKind(regions, i)}

		folded := headingText(h)
		if m := placeholderHeading.FindStringSubmatch(folded); m != nil &&
			strings.Contains(h.Text, "<!--") {
			rule.Prefix = strings.TrimSpace(m[1])
		} else {
			rule.Text = folded
		}

		spec = append(spec, rule)
	}

	for _, d := range sharedDefaults {
		if !seen[d.Kind] {
			spec = append(spec, d)
		}
	}

	return spec
}

// firstHeadingIn returns the first heading strictly inside a region.
func firstHeadingIn(heads []docparse.Heading, r docparse.Region) (docparse.Heading, bool) {
	for _, h := range heads {
		if h.Line > r.Start && h.Line < r.End {
			return h, true
		}
	}

	return docparse.Heading{}, false
}

// parentKind returns the kind of the innermost region enclosing the one at
// index i, or "" when it is top-level.
func parentKind(regions []docparse.Region, i int) string {
	r := regions[i]
	if r.Depth == 0 {
		return ""
	}

	for j := i - 1; j >= 0; j-- {
		if regions[j].Depth == r.Depth-1 && regions[j].Start < r.Start && regions[j].End >= r.End {
			return regions[j].Kind
		}
	}

	return ""
}

// ResolveRegions returns a document's regions and whether they were
// inferred: the markers when the document carries them, otherwise the spans
// InferRegions derives from its headings.
//
// This is the one entry point a type package's Parse needs. Making the
// fallback a return value rather than a silent substitution is what lets a
// caller surface it — every Doc reports Inferred, and validate emits one
// region.inferred warning rather than one per region.
func ResolveRegions(doc []byte, spec HeadingSpec) (regions []docparse.Region, inferred bool) {
	if marked(doc) {
		return docparse.Regions(doc), false
	}

	return InferRegions(doc, spec), true
}

// RegionBytes returns the bytes of a region: the lines strictly between its
// start and end, which is the heading and the content under it.
//
// Every reader in this package takes what this returns. Inferred regions
// use the same arithmetic as marked ones — an inferred span opens on the
// line before its heading — so a reader cannot tell which path produced it.
func RegionBytes(doc []byte, r docparse.Region) []byte {
	lines := strings.Split(string(doc), "\n")

	from := max(r.Start, 0)

	through := min(r.End-1, len(lines))
	if through <= from {
		return nil
	}

	return []byte(strings.Join(lines[from:through], "\n") + "\n")
}

// marked reports whether a document carries region markers of its own.
//
// The legacy ToC and README index pairs do not count. Every v1 document has
// a ToC pair, and inference exists for exactly those documents, so counting
// it would mean nothing is ever inferred. Any other marker does count,
// including a half-marked document: markers, once present, are
// authoritative (DESIGN-0015 §6), and a document that names three of its
// regions is telling docz to read three.
func marked(doc []byte) bool {
	for _, m := range docparse.Markers(doc) {
		if m.Kind != docparse.TocKind && m.Kind != docparse.IndexKind {
			return true
		}
	}

	return false
}

// InferRegions derives regions from a document's headings, for a document
// that carries no markers of its own. It returns nil when the document does
// carry them, so a caller that skips the check still gets the right answer.
//
// This is the module's one heading heuristic, and it is permanent rather
// than a migration aid (DESIGN-0015 §6): the fleet's documents are read by
// heading today and will not all be re-marked. A span runs from its heading
// to the next heading of the same or shallower level, minus trailing blank
// lines and a trailing thematic break, and a parent's span runs to the end
// of its last child.
func InferRegions(doc []byte, spec HeadingSpec) []docparse.Region {
	if marked(doc) {
		return nil
	}

	lines := strings.Split(strings.TrimSuffix(string(doc), "\n"), "\n")
	heads := docparse.Headings(doc)

	var out []docparse.Region

	for i, h := range heads {
		rule, ok := matchRule(spec, h)
		if !ok {
			continue
		}

		end := spanEnd(lines, heads, i, h.Level, spec, rule.Kind)

		// An inferred region opens on the line before its heading, so the
		// heading lands strictly inside the span exactly as it does inside a
		// marked one.
		out = append(out, docparse.Region{
			Kind:   rule.Kind,
			Start:  h.Line - 1,
			End:    end + 1,
			Depth:  0,
			Closed: true,
		})
	}

	out = nest(out, spec)

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Start != out[j].Start {
			return out[i].Start < out[j].Start
		}

		return out[i].Depth < out[j].Depth
	})

	return out
}

// matchRule finds the rule a heading satisfies. Text rules are tried before
// prefix rules, so a document whose phase is literally titled by a text
// rule is not captured by the looser one.
func matchRule(spec HeadingSpec, h docparse.Heading) (HeadingRule, bool) {
	folded := headingText(h)

	for _, r := range spec {
		if r.Level == h.Level && r.Text != "" && r.Text == folded {
			return r, true
		}
	}

	for _, r := range spec {
		if r.Level != h.Level || r.Prefix == "" {
			continue
		}

		rest, ok := strings.CutPrefix(folded, r.Prefix)
		if !ok {
			continue
		}

		// The prefix is a whole word: "Phases: overview" is not a phase.
		if rest != "" && !strings.ContainsAny(rest[:1], " \t") {
			continue
		}

		rest = strings.TrimSpace(rest)

		token, _, found := strings.Cut(rest, ":")
		if !found || token == "" || strings.ContainsAny(token, " /") {
			continue
		}

		return r, true
	}

	return HeadingRule{}, false
}

// spanEnd returns the last line of the span opened by heads[i], trimmed of
// trailing blank lines and a trailing thematic break.
//
// The break goes because the IMPL template puts one between phases: it
// separates the sections, it is not the tail of the one above it.
//
// A span ends at the next heading of the same or shallower level, and also at
// any deeper heading that opens a region this one cannot contain. The second
// rule is what keeps two regions from overlapping: sdk-booty-sh's messy
// fixture puts "### Phase A:" directly under "## Objective" with no level-2
// heading between them, and by level alone the objective would run to the end
// of the document and swallow the phase. Two depth-0 regions cannot overlap —
// no arrangement of markers expresses it — so a document that was read that
// way could not be migrated, and its phase would come back nested one level
// too deep. A declared child is exempt: an in-scope region is inside its
// scope, which is the whole point of the parent field.
func spanEnd(
	lines []string, heads []docparse.Heading, i, level int,
	spec HeadingSpec, kind string,
) int {
	end := len(lines)

	for _, next := range heads[i+1:] {
		if next.Level <= level {
			end = next.Line - 1

			break
		}

		if rule, ok := matchRule(spec, next); ok && !descends(spec, rule.Kind, kind) {
			end = next.Line - 1

			break
		}
	}

	for end > heads[i].Line {
		trimmed := strings.TrimSpace(lines[end-1])
		if trimmed != "" && !isThematicBreak(trimmed) {
			break
		}

		end--
	}

	return end
}

// isThematicBreak reports whether a trimmed line is a horizontal rule.
func isThematicBreak(trimmed string) bool {
	if len(trimmed) < 3 {
		return false
	}

	marker := trimmed[0]
	if marker != '-' && marker != '*' && marker != '_' {
		return false
	}

	return strings.Trim(trimmed, string(marker)+" \t") == ""
}

// nest assigns depth and re-cuts parents. A rule with a parent only holds
// inside that parent's span, and a parent runs to the end of its last
// child: an ADR's consequences region ends where its neutral list does, not
// where the next level-2 heading starts, so the two agree with the marked
// template.
func nest(regions []docparse.Region, spec HeadingSpec) []docparse.Region {
	parentOf := make(map[string]string, len(spec))
	for _, r := range spec {
		parentOf[r.Kind] = r.Parent
	}

	for i := range regions {
		parent := parentOf[regions[i].Kind]
		if parent == "" {
			continue
		}

		for j := range regions {
			if regions[j].Kind != parent {
				continue
			}

			if regions[j].Start > regions[i].Start || regions[j].End < regions[i].Start {
				continue
			}

			regions[i].Depth = regions[j].Depth + 1

			if regions[j].End < regions[i].End {
				regions[j].End = regions[i].End
			}

			break
		}
	}

	// A child whose parent never matched is not a region: an IMPL's tasks
	// heading outside a phase is a heading, not a task list (DESIGN-0014 §3).
	out := make([]docparse.Region, 0, len(regions))

	for _, r := range regions {
		if parentOf[r.Kind] != "" && r.Depth == 0 {
			continue
		}

		out = append(out, r)
	}

	return out
}

// descends reports whether kind is ancestor, or nests inside it through the
// spec's parent links.
//
// Walked rather than looked up one level, so a spec that nests three deep
// behaves. The loop is bounded by the spec's length because a cycle in the
// parent links — which a hand-written table could have — must not hang a
// parse.
func descends(spec HeadingSpec, kind, ancestor string) bool {
	parentOf := make(map[string]string, len(spec))
	for _, r := range spec {
		parentOf[r.Kind] = r.Parent
	}

	for range len(spec) + 1 {
		if kind == ancestor {
			return true
		}

		parent, ok := parentOf[kind]
		if !ok || parent == "" {
			return false
		}

		kind = parent
	}

	return false
}
