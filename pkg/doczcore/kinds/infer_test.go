package kinds_test

import (
	"regexp"
	"strings"
	"testing"

	doctemplate "github.com/donaldgifford/docz/v2/internal/template"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
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
