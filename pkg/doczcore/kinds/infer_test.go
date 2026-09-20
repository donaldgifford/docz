package kinds_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/doctemplate"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// markerLine matches a whole marker line including its newline, so removing
// one leaves the document's own lines untouched.
var markerLine = regexp.MustCompile(`(?m)^<!--docz:[a-z0-9-]+:(?:start|end)-->\n`)

// TestInferenceEqualsMarkers is the proof DESIGN-0015 §6 rests on. Strip a
// marked template's markers and inference must find the same regions, in the
// same order, at the same depths, holding the same content.
//
// Without it the heuristic is a guess, and a document read one way before
// migration and another way after would make `docz validate --fix` a
// behaviour change rather than a rewrite.
//
// The ToC region is the one exception: it is a pair of markers with no
// heading, so nothing can infer it. That is why it keeps its legacy spelling
// in every document rather than becoming a heading-backed kind.
func TestInferenceEqualsMarkers(t *testing.T) {
	t.Parallel()

	for _, name := range config.DocTypeNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			marked, err := doctemplate.EmbeddedDocumentTemplate(config.DocType(name))
			if err != nil {
				t.Fatalf("EmbeddedDocumentTemplate(%q): %v", name, err)
			}

			spec := kinds.SpecFromTemplate([]byte(marked))
			bare := markerLine.ReplaceAllString(marked, "")

			inferred := kinds.InferRegions([]byte(bare), spec)

			var want []docparse.Region

			for _, r := range docparse.Regions([]byte(marked)) {
				if r.Kind != docparse.TocKind {
					want = append(want, r)
				}
			}

			if len(inferred) != len(want) {
				t.Fatalf("inferred %d regions, markers declare %d\ninferred: %s\nmarked:   %s",
					len(inferred), len(want), kindsOf(inferred), kindsOf(want))
			}

			for i := range want {
				if inferred[i].Kind != want[i].Kind || inferred[i].Depth != want[i].Depth {
					t.Errorf("region %d: inferred %s/%d, markers say %s/%d",
						i, inferred[i].Kind, inferred[i].Depth, want[i].Kind, want[i].Depth)

					continue
				}

				// Compared on content lines rather than byte for byte. A
				// marked span carries the blank line before its end marker and
				// a blank where each nested marker stood; an inferred one
				// trims both. What has to be equal is the lines that say
				// something, and that is the claim inference makes.
				got := contentLines(kinds.RegionBytes([]byte(bare), inferred[i]))
				if exp := contentLines(kinds.RegionBytes([]byte(marked), want[i])); got != exp {
					t.Errorf("region %d (%s) content differs\ninferred: %q\nmarked:   %q",
						i, want[i].Kind, got, exp)
				}
			}
		})
	}
}

func kindsOf(regions []docparse.Region) string {
	out := make([]string, 0, len(regions))
	for _, r := range regions {
		out = append(out, r.Kind)
	}

	return strings.Join(out, " ")
}

// A document that names even one region is telling docz to read its markers.
// Mixing the two would make the same document parse differently depending on
// how much of it the author had got round to marking.
func TestInferRegions_MarkersWin(t *testing.T) {
	t.Parallel()

	spec := kinds.HeadingSpec{{Kind: "summary", Level: 2, Text: "summary"}}

	doc := []byte("# T\n\n<!--docz:summary:start-->\n## Summary\n\ntext\n<!--docz:summary:end-->\n")
	if got := kinds.InferRegions(doc, spec); got != nil {
		t.Errorf("InferRegions() = %+v on a marked document, want nil", got)
	}

	regions, inferred := kinds.ResolveRegions(doc, spec)
	if inferred {
		t.Error("ResolveRegions reported inference for a marked document")
	}

	if len(regions) != 1 || regions[0].Kind != "summary" {
		t.Errorf("ResolveRegions() = %+v", regions)
	}
}

// The legacy ToC pair does not make a document marked. Every v1 document has
// one, and inference exists for exactly those documents.
func TestInferRegions_LegacyTocIsNotAMarker(t *testing.T) {
	t.Parallel()

	spec := kinds.HeadingSpec{{Kind: "summary", Level: 2, Text: "summary"}}
	doc := []byte("# T\n\n<!--toc:start-->\n<!--toc:end-->\n\n## Summary\n\ntext\n")

	regions, inferred := kinds.ResolveRegions(doc, spec)
	if !inferred {
		t.Fatal("a document with only a ToC pair was treated as marked")
	}

	if len(regions) != 1 || regions[0].Kind != "summary" {
		t.Fatalf("ResolveRegions() = %+v", regions)
	}

	if got := kinds.Body(kinds.RegionBytes(doc, regions[0])); got != "text" {
		t.Errorf("region body = %q, want %q", got, "text")
	}
}

func TestInferRegions_Spans(t *testing.T) {
	t.Parallel()

	spec := kinds.HeadingSpec{
		{Kind: "phase", Level: 3, Prefix: "phase"},
		{Kind: "tasks", Level: 4, Text: "tasks", Parent: "phase"},
		{Kind: "summary", Level: 2, Text: "summary"},
	}

	t.Run("a trailing thematic break is not part of the span", func(t *testing.T) {
		t.Parallel()

		doc := []byte("### Phase 1: Setup\n\n#### Tasks\n\n- [ ] a\n\n---\n\n### Phase 2: Core\n")

		regions := kinds.InferRegions(doc, spec)
		if len(regions) != 3 {
			t.Fatalf("got %d regions: %s", len(regions), kindsOf(regions))
		}

		body := string(kinds.RegionBytes(doc, regions[0]))
		if strings.Contains(body, "---") {
			t.Errorf("the break between phases was kept: %q", body)
		}
	})

	t.Run("a child heading outside its parent is not a region", func(t *testing.T) {
		t.Parallel()

		doc := []byte("## Summary\n\ntext\n\n#### Tasks\n\n- [ ] not a task\n")

		regions := kinds.InferRegions(doc, spec)
		if len(regions) != 1 || regions[0].Kind != "summary" {
			t.Errorf("got %s, want summary alone", kindsOf(regions))
		}
	})

	t.Run("a prefix rule needs a whole word", func(t *testing.T) {
		t.Parallel()

		doc := []byte("### Phases: an overview\n\ntext\n")

		if regions := kinds.InferRegions(doc, spec); len(regions) != 0 {
			t.Errorf("'Phases:' matched the phase prefix: %s", kindsOf(regions))
		}
	})

	t.Run("a heading at the wrong level does not match", func(t *testing.T) {
		t.Parallel()

		doc := []byte("### Summary\n\ntext\n")

		if regions := kinds.InferRegions(doc, spec); len(regions) != 0 {
			t.Errorf("a level-3 Summary matched a level-2 rule: %s", kindsOf(regions))
		}
	})
}

// A type package pins its own heading table against the embedded template
// with this equality, which is how it reads an unmarked document without
// importing anything above the facts layer.
func TestSpecFromTemplate_IsStableData(t *testing.T) {
	t.Parallel()

	marked, err := doctemplate.EmbeddedDocumentTemplate(config.DocType("impl"))
	if err != nil {
		t.Fatal(err)
	}

	spec := kinds.SpecFromTemplate([]byte(marked))

	byKind := make(map[string]kinds.HeadingRule, len(spec))
	for _, r := range spec {
		if _, dup := byKind[r.Kind]; dup {
			t.Errorf("two rules for kind %q", r.Kind)
		}

		byKind[r.Kind] = r

		if r.Text != "" && r.Prefix != "" {
			t.Errorf("rule for %q sets both Text and Prefix: %+v", r.Kind, r)
		}

		if r.Text == "" && r.Prefix == "" {
			t.Errorf("rule for %q matches nothing: %+v", r.Kind, r)
		}
	}

	// The IMPL template's placeholder phase heading must generalise, or a
	// document's "Phase 3: CI Readiness" is not a phase.
	if got := byKind["phase"]; got.Prefix != "phase" || got.Text != "" {
		t.Errorf("phase rule = %+v, want a prefix rule on 'phase'", got)
	}

	if got := byKind["tasks"]; got.Parent != "phase" || got.Level != 4 {
		t.Errorf("tasks rule = %+v, want level 4 under phase", got)
	}

	// Shared kinds the template does not ship are still readable.
	if got := byKind["open-questions"]; got.Text != "open questions" {
		t.Errorf("open-questions rule = %+v", got)
	}
}

// contentLines reduces a region to the lines that say something: HTML
// comments removed (which is what a marker line is), then blanks dropped and
// each line trimmed. Two regions with the same content lines hold the same
// document text however they were delimited.
func contentLines(region []byte) string {
	var out []string

	for _, line := range strings.Split(commentPattern.ReplaceAllString(string(region), ""), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return strings.Join(out, "\n")
}

// (?s) so a comment spanning lines goes whole, as the templates' guidance
// comments do.
var commentPattern = regexp.MustCompile(`(?s)<!--.*?-->`)

// A half-marked document is read by its markers, not by its headings. Mixing
// the two would make the same document parse differently depending on how much
// of it the author had got round to marking, and a region the author had not
// reached yet would appear and disappear as they worked.
func TestInferRegions_OneMarkerIsEnough(t *testing.T) {
	t.Parallel()

	spec := kinds.HeadingSpec{
		{Kind: "summary", Level: 2, Text: "summary"},
		{Kind: "context", Level: 2, Text: "context"},
		{Kind: "decision", Level: 2, Text: "decision"},
	}

	doc := []byte("# T\n\n<!--toc:start-->\n<!--toc:end-->\n\n" +
		"<!--docz:summary:start-->\n## Summary\n\none\n<!--docz:summary:end-->\n\n" +
		"## Context\n\ntwo\n\n## Decision\n\nthree\n")

	if got := kinds.InferRegions(doc, spec); got != nil {
		t.Errorf("InferRegions() = %s, want nil", kindsOf(got))
	}

	regions, inferred := kinds.ResolveRegions(doc, spec)
	if inferred {
		t.Error("ResolveRegions inferred for a half-marked document")
	}

	// The ToC pair and the one marked region, and neither unmarked heading.
	if got := kindsOf(regions); got != "toc summary" {
		t.Errorf("ResolveRegions() = %q, want \"toc summary\"", got)
	}
}

// Inference has to work on the corpus, not only on templates. ADR-0002 is a
// real hand-written document with no region markers, and it is the shape
// `docz validate --fix` will meet on every repo in the fleet.
func TestInferRegions_OverARealDocument(t *testing.T) {
	t.Parallel()

	marked, err := doctemplate.EmbeddedDocumentTemplate(config.DocType("adr"))
	if err != nil {
		t.Fatal(err)
	}

	paths, err := filepath.Glob(filepath.Join("..", "..", "..", "docs", "adr", "0002-*.md"))
	if err != nil || len(paths) != 1 {
		t.Skipf("ADR-0002 not found in this checkout: %v", err)
	}

	doc, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}

	regions, inferred := kinds.ResolveRegions(doc, kinds.SpecFromTemplate([]byte(marked)))
	if !inferred {
		t.Fatal("ADR-0002 carries region markers; pick another unmarked document")
	}

	found := make(map[string]bool, len(regions))
	for _, r := range regions {
		found[r.Kind] = true
	}

	// Every section ADR-0002 has, including the three nested under its
	// consequences, plus the Open Questions the ADR template does not ship —
	// that one comes from the shared defaults.
	for _, kind := range []string{
		"summary", "context", "decision", "consequences",
		"positive", "negative", "neutral",
		"alternatives", "open-questions", "references",
	} {
		if !found[kind] {
			t.Errorf("inference missed %q in ADR-0002: found %s", kind, kindsOf(regions))
		}
	}

	// ADR-0002 has no Decisions section: it records its decisions in a table
	// inside Open Questions. A spec rule with no matching heading must yield
	// no region, or every document would appear to have every kind.
	if found["decisions"] {
		t.Error("inference invented a decisions region ADR-0002 does not have")
	}

	// The regions must hold the document's text, not the template's.
	for _, r := range regions {
		if r.Kind != "summary" {
			continue
		}

		if body := kinds.Body(kinds.RegionBytes(doc, r)); body == "" {
			t.Error("the summary region is empty")
		}
	}
}
