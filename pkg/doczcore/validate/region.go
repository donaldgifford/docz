package validate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// checkRegions reports the region-level findings: a kind the schema
// requires and the document does not have, a second region of a singleton
// kind under one parent, and a region at the wrong depth.
func checkRegions(regions []docparse.Region, schema Schema) []Finding {
	present := make(map[SchemaRegion][]docparse.Region, len(regions))

	for i, r := range regions {
		entry := SchemaRegion{Kind: r.Kind, Parent: parentKind(regions, i)}
		present[entry] = append(present[entry], r)
	}

	out := checkMissing(present, schema)
	out = append(out, checkSingletons(regions)...)

	return append(out, checkParents(present, schema)...)
}

// checkMissing reports each schema kind absent under the parent the schema
// places it under.
//
// The two spans docz splices are left to their own families. A missing ToC
// is toc.missing and a missing README table is an index finding; reporting
// region.missing as well would give one absence two findings at two
// severities, and the specific one is the one that tells an author which
// command fixes it.
func checkMissing(present map[SchemaRegion][]docparse.Region, schema Schema) []Finding {
	var out []Finding

	for _, want := range schema.Regions {
		if len(present[want]) > 0 {
			continue
		}

		if want.Kind == docparse.TocKind || want.Kind == docparse.IndexKind {
			continue
		}

		out = append(out, Finding{
			Code:     "region.missing",
			Severity: Error,
			Kind:     want.Kind,
			Detail:   missingDetail(want),
		})
	}

	return out
}

func missingDetail(want SchemaRegion) string {
	if want.Parent == "" {
		return fmt.Sprintf("the schema requires a %q region", want.Kind)
	}

	return fmt.Sprintf("the schema requires a %q region inside %q", want.Kind, want.Parent)
}

// checkSingletons reports a second region of a kind the catalogue says is a
// singleton, scoped to the one parent region it sits in.
//
// Scoped to the parent *instance*, not to the parent kind. An IMPL has one
// tasks region inside each of its phases, and grouping by kind would report
// the second and third phases' tasks as duplicates of the first's — which is
// how this was wrong first time round, caught by the zero-findings golden
// over the IMPL template. Two tasks regions inside one phase is still a
// finding.
//
// The finding points at the duplicate rather than the original, since the
// original is the one the readers will use.
func checkSingletons(regions []docparse.Region) []Finding {
	type scope struct {
		kind        string
		parentStart int
	}

	seen := make(map[scope]int, len(regions))

	var out []Finding

	for i, r := range regions {
		if !catalogue[r.Kind].Singleton {
			continue
		}

		key := scope{kind: r.Kind, parentStart: parentStart(regions, i)}

		first, dup := seen[key]
		if !dup {
			seen[key] = r.Start

			continue
		}

		out = append(out, Finding{
			Code:     "region.duplicate-singleton",
			Severity: Warning,
			Line:     r.Start,
			Kind:     r.Kind,
			Detail: fmt.Sprintf("a document holds one %q region%s; the first is at line %d",
				r.Kind, inside(parentKind(regions, i)), first),
		})
	}

	sortFindings(out)

	return out
}

// parentStart is the start line of the region enclosing the one at index i,
// or 0 when it is top-level. It identifies the parent instance, which is what
// singleton scoping needs.
func parentStart(regions []docparse.Region, i int) int {
	r := regions[i]
	if r.Depth == 0 {
		return 0
	}

	for j := i - 1; j >= 0; j-- {
		if regions[j].Depth == r.Depth-1 && regions[j].Start < r.Start && regions[j].End >= r.End {
			return regions[j].Start
		}
	}

	return 0
}

// checkParents reports a region whose kind the schema nests, found
// somewhere else.
//
// This is an error rather than a warning because the readers go by span: an
// IMPL's tasks region outside a phase belongs to no phase, so its tasks are
// attributed to nothing, and a consumer counting progress silently loses
// them.
func checkParents(present map[SchemaRegion][]docparse.Region, schema Schema) []Finding {
	wantParent := make(map[string]string, len(schema.Regions))
	for _, want := range schema.Regions {
		wantParent[want.Kind] = want.Parent
	}

	var out []Finding

	for entry, found := range present {
		want, known := wantParent[entry.Kind]
		if !known || want == entry.Parent {
			continue
		}

		for _, r := range found {
			out = append(out, Finding{
				Code:     "region.wrong-parent",
				Severity: Error,
				Line:     r.Start,
				Kind:     entry.Kind,
				Detail: fmt.Sprintf("%q belongs%s, but this one is%s",
					entry.Kind, inside(want), inside(entry.Parent)),
			})
		}
	}

	sortFindings(out)

	return out
}

// inside renders a parent kind for a message, or "at the top level".
func inside(parent string) string {
	if parent == "" {
		return " at the top level"
	}

	return fmt.Sprintf(" inside %q", parent)
}

// checkContent runs each region's content rule from the catalogue.
//
// An unclosed region is skipped. Its span is whatever the walker could
// salvage, so any content finding over it would be about text the author
// did not put there; the marker.unclosed error already says what to fix.
func checkContent(content []byte, regions []docparse.Region) []Finding {
	var out []Finding

	for _, r := range regions {
		rule, known := catalogue[r.Kind]
		if !known || rule.Check == nil || !r.Closed {
			continue
		}

		out = append(out, rule.Check(kinds.RegionBytes(content, r), r)...)
	}

	return out
}

// inferredFinding is the one region.inferred warning a run emits when a
// document's regions came from its headings rather than from markers.
//
// One warning, not one per region: a document with no markers has a single
// thing to say about it, and the type packages set Doc.Inferred on the same
// condition without emitting anything, so a caller that runs both tiers
// still reports it once (DESIGN-0015 §4).
//
// The Detail names what was inferred and, for each schema kind that was
// not, the heading that was looked for. That second half is the useful one:
// "docz looked for a level-2 heading called Scope" is a fix an author can
// act on, where "a region is missing" is not.
func inferredFinding(regions []docparse.Region, schema Schema, spec kinds.HeadingSpec) Finding {
	found := make(map[string]bool, len(regions))

	names := make([]string, 0, len(regions))

	for _, r := range regions {
		if found[r.Kind] {
			continue
		}

		found[r.Kind] = true
		names = append(names, r.Kind)
	}

	sort.Strings(names)

	detail := "regions inferred from headings"
	if len(names) > 0 {
		detail += ": " + strings.Join(names, ", ")
	}

	var looked []string

	for _, want := range schema.Regions {
		if found[want.Kind] {
			continue
		}

		if heading := headingFor(spec, want.Kind); heading != "" {
			looked = append(looked, fmt.Sprintf("%s (%s)", want.Kind, heading))
		}
	}

	if len(looked) > 0 {
		sort.Strings(looked)
		detail += "; not found, looked for " + strings.Join(looked, ", ")
	}

	return Finding{Code: "region.inferred", Severity: Warning, Detail: detail}
}

// headingFor describes the heading a spec rule looks for, so a finding can
// tell the author what to write.
func headingFor(spec kinds.HeadingSpec, kind string) string {
	for _, rule := range spec {
		if rule.Kind != kind {
			continue
		}

		hashes := strings.Repeat("#", rule.Level)

		if rule.Prefix != "" {
			return fmt.Sprintf("%s %s <token>:", hashes, rule.Prefix)
		}

		return fmt.Sprintf("%s %s", hashes, rule.Text)
	}

	return ""
}
