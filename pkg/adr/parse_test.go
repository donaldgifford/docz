package adr_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/adr"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// grammar is the marked fixture every assertion reads. One document rather
// than a literal per case: the sections interact — the consequence lists are
// nested inside consequences, a wrapped bullet ends where the next one begins
// — and a fixture a person can read is the only way to see that.
func grammar(t *testing.T) []byte {
	t.Helper()

	return readFixture(t, "grammar.md")
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	return content
}

// lineOf returns the 1-based line holding the only occurrence of want.
//
// Assertions name the line by its text rather than by number, so inserting a
// paragraph into the fixture does not renumber twenty expectations. It fails
// on a second match, because an ambiguous anchor would silently assert against
// the wrong line.
func lineOf(t *testing.T, doc []byte, want string) int {
	t.Helper()

	found := 0
	at := 0

	for i, line := range strings.Split(string(doc), "\n") {
		if strings.Contains(line, want) {
			found++
			at = i + 1
		}
	}

	switch found {
	case 1:
		return at
	case 0:
		t.Fatalf("fixture has no line containing %q", want)
	default:
		t.Fatalf("fixture has %d lines containing %q, want exactly one", found, want)
	}

	return 0
}

func parse(t *testing.T, doc []byte) adr.Doc {
	t.Helper()

	got, err := adr.Parse(doc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	return got
}

func TestParse_Frontmatter(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	for _, tt := range []struct{ field, got, want string }{
		{"ID", got.ID, "ADR-0002"},
		{"Title", got.Title, "Read ADR sections by region rather than by heading"},
		{"Status", string(got.Status), "Proposed"},
		{"Author", got.Author, "Test Author"},
		{"Created", got.Created, "2026-09-20"},
	} {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.field, tt.got, tt.want)
		}
	}

	if got.Inferred {
		t.Error("Inferred = true for a fully marked document")
	}
}

func TestParse_Prose(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	if !strings.HasPrefix(got.Summary, "Locate an ADR's sections") {
		t.Errorf("Summary = %q, want it to open with the first prose line", got.Summary)
	}

	if !strings.HasSuffix(got.Summary, "field.") {
		t.Errorf("Summary = %q, want it to end with the last prose line", got.Summary)
	}

	// The template's guidance to the author is an HTML comment, so a filled-in
	// section reads as its prose and an empty one reads as "".
	if !strings.HasPrefix(got.Context, "Every field of the typed model") {
		t.Errorf("Context = %q, want the comment stripped from the front", got.Context)
	}

	// Supporting Data stays in Decision: it is the evidence for the decision,
	// not a section beside it (DESIGN-0014 §2.9).
	if !strings.HasPrefix(got.Decision, "Regions are authoritative.") {
		t.Errorf("Decision = %q, want it to open with the decision itself", got.Decision)
	}

	if !strings.Contains(got.Decision, "### Supporting Data") {
		t.Errorf("Decision = %q, want the Supporting Data subsection included", got.Decision)
	}

	if !strings.HasSuffix(got.Decision, "and 3 do not.") {
		t.Errorf("Decision = %q, want it to run through the supporting data", got.Decision)
	}
}

// TestParse_Consequences pins the three nested regions: they are separate
// fields because a consumer reads them apart, and each is the top-level items
// of a region one level inside consequences.
func TestParse_Consequences(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t)).Consequences

	if len(got.Positive) != 2 {
		t.Fatalf("Positive has %d items, want 2", len(got.Positive))
	}

	if want := "One parser reads a marked document and a legacy one"; got.Positive[1].Text != want {
		t.Errorf("Positive[1].Text = %q, want %q", got.Positive[1].Text, want)
	}

	if len(got.Negative) != 1 {
		t.Fatalf("Negative has %d items, want 1", len(got.Negative))
	}

	// The wrapped second line folds into the item with one space.
	want := "Every document in the fleet eventually wants markers, " +
		"which is a migration nobody has scheduled"
	if got.Negative[0].Text != want {
		t.Errorf("Negative[0].Text = %q, want the wrapped lines folded:\n  %q",
			got.Negative[0].Text, want)
	}

	if len(got.Neutral) != 1 {
		t.Errorf("Neutral has %d items, want 1", len(got.Neutral))
	}
}

func TestParse_Alternatives(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t)).Alternatives

	if len(got) != 3 {
		t.Fatalf("got %d alternatives, want 3", len(got))
	}

	// The label sits inside the bold, which is how this repo's own ADRs write
	// one: "- **a. Match on heading text only.** Ship the table and stop."
	if got[0].Label != "a" || got[0].Title != "Match on heading text only." {
		t.Errorf("alternatives[0] = {Label:%q Title:%q}, want the label pulled out of the bold",
			got[0].Label, got[0].Title)
	}

	if !strings.HasPrefix(got[0].Text, "Ship the table and stop.") {
		t.Errorf("alternatives[0].Text = %q, want the prose after the bold", got[0].Text)
	}

	if got[1].Label != "b" || got[2].Label != "c" {
		t.Errorf("labels = %q, %q, want b and c", got[1].Label, got[2].Label)
	}
}

func TestParse_OpenQuestions(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t)).OpenQuestions

	if len(got) != 2 {
		t.Fatalf("got %d questions, want 2", len(got))
	}

	resolved, open := got[0], got[1]

	if resolved.Number != 1 || resolved.Title != "Where does the fallback heading table live?" {
		t.Errorf("questions[0] = {Number:%d Title:%q}", resolved.Number, resolved.Title)
	}

	if len(resolved.Options) != 2 {
		t.Fatalf("questions[0] has %d options, want 2", len(resolved.Options))
	}

	if resolved.Options[0].Letter != "a" || !resolved.Options[0].Recommended {
		t.Errorf("options[0] = {Letter:%q Recommended:%v}, want the recommended option a",
			resolved.Options[0].Letter, resolved.Options[0].Recommended)
	}

	if resolved.Resolved == nil {
		t.Fatal("questions[0].Resolved = nil, want the resolution blockquote")
	}

	if resolved.Resolved.Date != "2026-09-20" || resolved.Resolved.Choice != "a" {
		t.Errorf("Resolved = {Date:%q Choice:%q}, want 2026-09-20 and a",
			resolved.Resolved.Date, resolved.Resolved.Choice)
	}

	if open.Resolved != nil {
		t.Errorf("questions[1].Resolved = %+v, want nil while the question is open", open.Resolved)
	}
}

func TestParse_References(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t)).References

	if len(got) != 2 {
		t.Fatalf("got %d references, want 2", len(got))
	}

	for i, ref := range got {
		if ref.URL == "" {
			t.Errorf("references[%d] = %+v, want a link target", i, ref)
		}
	}

	if !strings.HasPrefix(got[0].Text, "[ADR-0001](") {
		t.Errorf("references[0].Text = %q, want the bullet verbatim", got[0].Text)
	}
}

// TestParse_LinesAreDocumentLines pins the conversion every field goes
// through: the kinds readers number from the start of the region they were
// handed, and a Doc's Line is an address in the file the caller read.
//
// The three consequence lists are the interesting rows. They come from regions
// nested one level inside consequences, which is where an offset is easiest to
// get wrong and hardest to see — a line one off still looks like a line.
func TestParse_LinesAreDocumentLines(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	got := parse(t, doc)

	tests := []struct {
		field  string
		got    int
		anchor string
	}{
		{
			"Consequences.Positive[0].Line",
			got.Consequences.Positive[0].Line,
			"- A renamed section becomes a finding",
		},
		{
			"Consequences.Negative[0].Line",
			got.Consequences.Negative[0].Line,
			"- Every document in the fleet",
		},
		{
			"Consequences.Neutral[0].Line",
			got.Consequences.Neutral[0].Line,
			"- The heading table is written out",
		},
		{
			"Alternatives[0].Line",
			got.Alternatives[0].Line,
			"- **a. Match on heading text only.**",
		},
		{
			"OpenQuestions[0].Line",
			got.OpenQuestions[0].Line,
			"### 1. Where does the fallback",
		},
		{
			"OpenQuestions[0].Options[0].Line",
			got.OpenQuestions[0].Options[0].Line,
			"- a. In the type package",
		},
		{
			"OpenQuestions[0].Resolved.Line",
			got.OpenQuestions[0].Resolved.Line,
			"> **Resolved 2026-09-20: (a).**",
		},
		{
			"References[0].Line",
			got.References[0].Line,
			"- [ADR-0001](",
		},
	}

	for _, tt := range tests {
		if want := lineOf(t, doc, tt.anchor); tt.got != want {
			t.Errorf("%s = %d, want %d (%q)", tt.field, tt.got, want, tt.anchor)
		}
	}
}

// TestParse_Inferred is the compatibility path: the same document with every
// marker removed parses to the same fields.
//
// It compares against the marked parse rather than against a literal, so the
// two can never be updated apart. Only Inferred differs, which is the flag
// that exists to say so. Lines are not compared, because removing the marker
// lines moves every line in the document.
func TestParse_Inferred(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	marked := parse(t, doc)
	bare := parse(t, unmark(doc))

	if !bare.Inferred {
		t.Error("Inferred = false for a document with no markers")
	}

	for _, tt := range []struct{ field, got, want string }{
		{"Summary", bare.Summary, marked.Summary},
		{"Context", bare.Context, marked.Context},
		{"Decision", bare.Decision, marked.Decision},
	} {
		if tt.got != tt.want {
			t.Errorf("%s = %q, marked %q", tt.field, tt.got, tt.want)
		}
	}

	// The nested lists are the reason this test exists: inference has to nest
	// positive, negative, and neutral inside consequences by the parent field
	// of the heading table, with no marker to tell it so.
	for _, tt := range []struct {
		field     string
		got, want []string
	}{
		{"Positive", itemTexts(bare.Consequences.Positive), itemTexts(marked.Consequences.Positive)},
		{"Negative", itemTexts(bare.Consequences.Negative), itemTexts(marked.Consequences.Negative)},
		{"Neutral", itemTexts(bare.Consequences.Neutral), itemTexts(marked.Consequences.Neutral)},
	} {
		if !equal(tt.got, tt.want) {
			t.Errorf("Consequences.%s = %v, marked %v", tt.field, tt.got, tt.want)
		}
	}

	if len(bare.Alternatives) != len(marked.Alternatives) {
		t.Fatalf("inferred %d alternatives, marked %d",
			len(bare.Alternatives), len(marked.Alternatives))
	}

	for i := range bare.Alternatives {
		if bare.Alternatives[i].Title != marked.Alternatives[i].Title ||
			bare.Alternatives[i].Text != marked.Alternatives[i].Text {
			t.Errorf("alternative %d = %+v, marked %+v",
				i, bare.Alternatives[i], marked.Alternatives[i])
		}
	}

	if len(bare.OpenQuestions) != len(marked.OpenQuestions) {
		t.Errorf("inferred %d questions, marked %d",
			len(bare.OpenQuestions), len(marked.OpenQuestions))
	}

	if len(bare.References) != len(marked.References) {
		t.Errorf("inferred %d references, marked %d", len(bare.References), len(marked.References))
	}
}

// TestParse_PartlyMarkedIsNotInferred pins the amended rule: markers, once
// present, are authoritative. A document that names one region is read as
// naming one, and the rest are validate's region.missing findings.
func TestParse_PartlyMarkedIsNotInferred(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	marked := parse(t, doc)

	// Keep only the decision markers, so every other section has a heading and
	// no marker.
	got := parse(t, keepMarkers(doc, "decision"))

	if got.Inferred {
		t.Error("Inferred = true for a partly marked document")
	}

	if got.Decision != marked.Decision {
		t.Errorf("Decision = %q, want the marked region's body %q", got.Decision, marked.Decision)
	}

	if got.Summary != "" {
		t.Errorf("Summary = %q, want empty: its region is not marked", got.Summary)
	}

	if got.Consequences.Positive != nil || got.Alternatives != nil || got.References != nil {
		t.Error("a field outside the one marked region is set, want every one of them zero")
	}
}

func TestParse_Errors(t *testing.T) {
	t.Parallel()

	doc := grammar(t)

	tests := []struct {
		name string
		doc  []byte
		want error
	}{
		{
			name: "no frontmatter",
			doc:  []byte("# ADR-0001\n\n## Decision\n\nDo the thing.\n"),
			want: document.ErrNoFrontmatter,
		},
		{
			name: "CR line endings",
			doc:  []byte(strings.ReplaceAll(string(doc), "\n", "\r\n")),
			want: document.ErrUnsupportedLineEndings,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := adr.Parse(tt.doc); !errors.Is(err, tt.want) {
				t.Errorf("Parse error = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestParse_MissingRegionIsNotAnError pins the other half of that contract:
// those two are the only failures. An ADR with no decision at all still
// parses, because a document under review is normally half-written and the
// missing section is validate's finding rather than the parser's error.
func TestParse_MissingRegionIsNotAnError(t *testing.T) {
	t.Parallel()

	got := parse(t, []byte(dropRegion(string(grammar(t)), "decision")))

	if got.Decision != "" {
		t.Errorf("Decision = %q, want empty", got.Decision)
	}

	if got.Summary == "" {
		t.Error("Summary is empty: the rest of the document still parses")
	}
}

// TestParse_CopiesEveryString pins the copy contract: a caller may reuse the
// bytes it passed in.
func TestParse_CopiesEveryString(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	got := parse(t, doc)

	for i := range doc {
		doc[i] = 'x'
	}

	if !strings.HasPrefix(got.Decision, "Regions are authoritative.") {
		t.Errorf("Decision = %q after overwriting the input, want the parsed value", got.Decision)
	}

	want := "A renamed section becomes a finding instead of a silently empty field"
	if got.Consequences.Positive[0].Text != want {
		t.Errorf("Consequences.Positive[0].Text = %q after overwriting the input, want %q",
			got.Consequences.Positive[0].Text, want)
	}
}

// TestHeadings returns a copy, so a consumer cannot reshape what Parse does.
func TestHeadings(t *testing.T) {
	t.Parallel()

	got := adr.Headings()
	if len(got) == 0 {
		t.Fatal("Headings() is empty")
	}

	got[0].Kind = "clobbered"

	if adr.Headings()[0].Kind == "clobbered" {
		t.Error("Headings() shares its backing array with the package's table")
	}
}

// unmark removes every docz marker line, which is how a v1 document reads.
func unmark(doc []byte) []byte {
	var out []string

	for _, line := range strings.Split(string(doc), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "<!--docz:") {
			continue
		}

		out = append(out, line)
	}

	return []byte(strings.Join(out, "\n"))
}

// keepMarkers removes every docz marker except the named kinds.
func keepMarkers(doc []byte, names ...string) []byte {
	var out []string

	for _, line := range strings.Split(string(doc), "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "<!--docz:") {
			keep := false

			for _, kind := range names {
				if strings.HasPrefix(trimmed, "<!--docz:"+kind+":") {
					keep = true
				}
			}

			if !keep {
				continue
			}
		}

		out = append(out, line)
	}

	return []byte(strings.Join(out, "\n"))
}

// dropRegion removes a region entirely, markers and all, leaving a document
// with every other section intact.
func dropRegion(doc, kind string) string {
	var out []string

	inside := false

	for _, line := range strings.Split(doc, "\n") {
		trimmed := strings.TrimSpace(line)

		switch {
		case trimmed == "<!--docz:"+kind+":start-->":
			inside = true
		case trimmed == "<!--docz:"+kind+":end-->":
			inside = false
		case !inside:
			out = append(out, line)
		}
	}

	return strings.Join(out, "\n")
}

// emptyRegion strips a region's content, keeping its markers and its own
// heading: the shape of a section nobody has written yet. Only the first
// heading survives, so a subsection's heading does not leave the region with a
// body.
func emptyRegion(doc, kind string) string {
	var out []string

	inside, kept := false, false

	for _, line := range strings.Split(doc, "\n") {
		trimmed := strings.TrimSpace(line)

		switch {
		case trimmed == "<!--docz:"+kind+":start-->":
			inside, kept = true, false

			out = append(out, line)
		case trimmed == "<!--docz:"+kind+":end-->":
			inside = false

			out = append(out, line)
		case inside && !kept && strings.HasPrefix(trimmed, "#"):
			kept = true

			out = append(out, line)
		case !inside:
			out = append(out, line)
		}
	}

	return strings.Join(out, "\n")
}

func itemTexts(items []kinds.Item) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Text)
	}

	return out
}

func equal(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}

// TestParse_RenderedTemplate parses what `docz create adr` writes.
//
// The point is not that a fresh document is full — it is that every field is
// zero for a reason somebody stated, because the template's placeholders are
// HTML comments and bare bullets. A field that started coming back non-empty
// would mean a placeholder had leaked into the parsed model, which is exactly
// what a consumer rendering a new ADR would show its reader.
func TestParse_RenderedTemplate(t *testing.T) {
	t.Parallel()

	got := parse(t, renderedTemplate(t))

	if got.Inferred {
		t.Error("Inferred = true: the template carries markers")
	}

	// The frontmatter is the one part the template really fills in.
	if got.ID != "ADR-0001" || got.Title != "A placeholder title" {
		t.Errorf("ID/Title = %q/%q, want ADR-0001/A placeholder title", got.ID, got.Title)
	}

	// Decision is the one non-empty body field, and it carries no decision:
	// the region holds a guidance comment and a "### Supporting Data"
	// subsection, and the subsection is deliberately part of the decision
	// (DESIGN-0014 §2.9), so stripping the comments leaves the heading alone.
	// adr.decision.empty therefore does not fire on a fresh document, which is
	// right — its status is Proposed, not Accepted.
	if want := "### Supporting Data"; got.Decision != want {
		t.Errorf("Decision = %q, want %q", got.Decision, want)
	}

	cons := got.Consequences

	for _, tt := range []struct {
		field, why string
		empty      bool
	}{
		{"Summary", "the placeholder is an HTML comment", got.Summary == ""},
		{"Context", "the placeholder is an HTML comment", got.Context == ""},
		{"Consequences.Positive", "the placeholder is a bare \"-\" bullet", len(cons.Positive) == 0},
		{"Consequences.Negative", "the placeholder is a bare \"-\" bullet", len(cons.Negative) == 0},
		{"Consequences.Neutral", "the placeholder is a bare \"-\" bullet", len(cons.Neutral) == 0},
		{"Alternatives", "the placeholder is an HTML comment", len(got.Alternatives) == 0},
		{"References", "the placeholder is an HTML comment", len(got.References) == 0},
		{"OpenQuestions", "the template ships no such section", got.OpenQuestions == nil},
	} {
		if !tt.empty {
			t.Errorf("%s is filled, want empty: %s", tt.field, tt.why)
		}
	}
}
