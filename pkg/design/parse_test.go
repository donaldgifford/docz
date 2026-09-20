package design_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/design"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// grammar is the marked fixture every assertion reads. One document rather
// than a literal per case: the sections interact — an unmarked "Goals and
// Non-Goals" heading sits between two regions, a detailed design carries
// level-3 headings that are not sections of its own — and a fixture a person
// can read is the only way to see that.
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

func parse(t *testing.T, doc []byte) design.Doc {
	t.Helper()

	got, err := design.Parse(doc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	return got
}

func TestParse_Frontmatter(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	for _, tt := range []struct{ field, got, want string }{
		{"ID", got.ID, "DESIGN-0001"},
		{"Title", got.Title, "Grammar fixture"},
		{"Status", string(got.Status), "Draft"},
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

// TestParse_StringFields pins the shared field rule for every string field: a
// string is its region's body, heading and HTML comments removed, trimmed.
func TestParse_StringFields(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	tests := []struct {
		field  string
		got    string
		prefix string
		suffix string
	}{
		{
			field:  "Overview",
			got:    got.Overview,
			prefix: "A design document that exercises",
			suffix: "something a person can read.",
		},
		{
			field:  "Background",
			got:    got.Background,
			prefix: "Five type packages share one shape",
			suffix: "no structure docz reads.",
		},
		{
			field:  "APIChanges",
			got:    got.APIChanges,
			prefix: "Three exported functions",
			suffix: "value they are about.",
		},
		{
			field:  "DataModel",
			got:    got.DataModel,
			prefix: "`Doc` holds one field per section",
			suffix: "no schema to migrate.",
		},
		{
			field:  "Testing",
			got:    got.Testing,
			prefix: "A marked fixture",
			suffix: "one case per\nfinding code.",
		},
		{
			field:  "Rollout",
			got:    got.Rollout,
			prefix: "The package ships with the v2 line",
			suffix: "no existing document has to change.",
		},
	}

	for _, tt := range tests {
		if !strings.HasPrefix(tt.got, tt.prefix) {
			t.Errorf("%s = %q, want it to open with %q", tt.field, tt.got, tt.prefix)
		}

		if !strings.HasSuffix(tt.got, tt.suffix) {
			t.Errorf("%s = %q, want it to end with %q", tt.field, tt.got, tt.suffix)
		}
	}
}

// TestParse_DetailedDesignKeepsItsSubsections is the one field with a rule of
// its own, and the rule is that there is no rule: the numbered level-3
// headings are the author's structure, so they stay in the body rather than
// being split into anything.
func TestParse_DetailedDesignKeepsItsSubsections(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t)).DetailedDesign

	if !strings.HasPrefix(got, "The package is a switch over region kinds") {
		t.Errorf("DetailedDesign = %q, want it to open with the region's own prose", got)
	}

	for _, want := range []string{"### 1. Parsing", "### 2. Validating"} {
		if !strings.Contains(got, want) {
			t.Errorf("DetailedDesign = %q, want it to contain the subsection heading %q", got, want)
		}
	}

	if !strings.HasSuffix(got, "reports the three rules that need a typed model.") {
		t.Errorf("DetailedDesign = %q, want it to run to the end of the last subsection", got)
	}
}

func TestParse_ListFields(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	if len(got.Goals) != 2 {
		t.Errorf("Goals has %d items, want 2: %+v", len(got.Goals), got.Goals)
	}

	if want := "Read every section of the DESIGN template into its own field"; got.Goals[0].Text != want {
		t.Errorf("Goals[0].Text = %q, want %q", got.Goals[0].Text, want)
	}

	if len(got.NonGoals) != 1 {
		t.Errorf("NonGoals has %d items, want 1: %+v", len(got.NonGoals), got.NonGoals)
	}

	if len(got.References) != 1 || got.References[0].URL == "" {
		t.Errorf("References = %+v, want one entry with a link", got.References)
	}
}

func TestParse_OpenQuestions(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	if len(got.OpenQuestions) != 2 {
		t.Fatalf("got %d questions, want 2: %+v", len(got.OpenQuestions), got.OpenQuestions)
	}

	first, second := got.OpenQuestions[0], got.OpenQuestions[1]

	if first.Number != 1 || first.Title != "Does the detailed design region keep its subsections?" {
		t.Errorf("question 1 = {Number:%d Title:%q}, want the numbered heading's text",
			first.Number, first.Title)
	}

	if len(first.Options) != 3 || first.Options[0].Letter != "a" {
		t.Errorf("question 1 options = %+v, want three lettered bullets", first.Options)
	}

	if !first.Options[0].Recommended {
		t.Error("question 1 option a is not Recommended, want the marker read")
	}

	if first.Resolved == nil {
		t.Fatal("question 1 Resolved = nil, want the resolution blockquote")
	}

	if first.Resolved.Date != "2026-09-20" || first.Resolved.Choice != "a" {
		t.Errorf("question 1 resolution = {Date:%q Choice:%q}, want {2026-09-20 a}",
			first.Resolved.Date, first.Resolved.Choice)
	}

	// The second question is the open one, which is what the status rule reads.
	if second.Number != 2 || second.Resolved != nil {
		t.Errorf("question 2 = {Number:%d Resolved:%+v}, want an unresolved question 2",
			second.Number, second.Resolved)
	}
}

func TestParse_Decisions(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	if len(got.Decisions) != 1 {
		t.Fatalf("got %d decisions, want 1: %+v", len(got.Decisions), got.Decisions)
	}

	row := got.Decisions[0]

	if row.Number != 1 {
		t.Errorf("Number = %d, want 1: the row answers question 1", row.Number)
	}

	if row.Question != "Does the region keep its subsections?" {
		t.Errorf("Question = %q, want the cell verbatim", row.Question)
	}

	if row.Resolution != "Yes, the body is reported whole" {
		t.Errorf("Resolution = %q, want the decision cell verbatim", row.Resolution)
	}
}

// TestParse_LinesAreDocumentLines pins the conversion every field goes
// through: the kinds readers number from the start of the region they were
// handed, and a Doc's Line is an address in the file the caller read.
func TestParse_LinesAreDocumentLines(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	got := parse(t, doc)

	tests := []struct {
		field  string
		got    int
		anchor string
	}{
		// Goals and Non-Goals are the offsets worth checking twice: they sit
		// under a heading the template does not mark, so a table that gave them
		// a parent would drop them and one that miscounted the span would move
		// every line in them.
		{"Goals[0].Line", got.Goals[0].Line, "- Read every section of the DESIGN template"},
		{"NonGoals[0].Line", got.NonGoals[0].Line, "- A grammar for the numbered subsections"},
		{"References[0].Line", got.References[0].Line, "- [DESIGN-0014]"},
		{
			"OpenQuestions[0].Line",
			got.OpenQuestions[0].Line,
			"### 1. Does the detailed design region keep",
		},
		{
			"OpenQuestions[0].Options[0].Line",
			got.OpenQuestions[0].Options[0].Line,
			"- a. Report the region's body whole",
		},
		{
			"OpenQuestions[0].Resolved.Line",
			got.OpenQuestions[0].Resolved.Line,
			"> **Resolved 2026-09-20: (a)**",
		},
		{"Decisions[0].Line", got.Decisions[0].Line, "| 1 | Does the region keep its subsections?"},
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
// two can never be updated apart. Only Inferred differs, which is the flag that
// exists to say so.
func TestParse_Inferred(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	marked := parse(t, doc)
	bare := parse(t, unmark(doc))

	if !bare.Inferred {
		t.Error("Inferred = false for a document with no markers")
	}

	for _, tt := range []struct{ field, got, want string }{
		{"Overview", bare.Overview, marked.Overview},
		{"Background", bare.Background, marked.Background},
		{"DetailedDesign", bare.DetailedDesign, marked.DetailedDesign},
		{"APIChanges", bare.APIChanges, marked.APIChanges},
		{"DataModel", bare.DataModel, marked.DataModel},
		{"Testing", bare.Testing, marked.Testing},
		{"Rollout", bare.Rollout, marked.Rollout},
	} {
		if tt.got != tt.want {
			t.Errorf("%s = %q, marked %q", tt.field, tt.got, tt.want)
		}
	}

	if !sameItems(bare.Goals, marked.Goals) {
		t.Errorf("Goals = %+v, marked %+v", bare.Goals, marked.Goals)
	}

	if !sameItems(bare.NonGoals, marked.NonGoals) {
		t.Errorf("NonGoals = %+v, marked %+v", bare.NonGoals, marked.NonGoals)
	}

	if len(bare.References) != len(marked.References) ||
		bare.References[0].Text != marked.References[0].Text {
		t.Errorf("References = %+v, marked %+v", bare.References, marked.References)
	}

	if len(bare.Decisions) != len(marked.Decisions) ||
		bare.Decisions[0].Number != marked.Decisions[0].Number {
		t.Errorf("Decisions = %+v, marked %+v", bare.Decisions, marked.Decisions)
	}

	if len(bare.OpenQuestions) != len(marked.OpenQuestions) {
		t.Fatalf("inferred %d questions, marked %d",
			len(bare.OpenQuestions), len(marked.OpenQuestions))
	}

	for i := range bare.OpenQuestions {
		got, want := bare.OpenQuestions[i], marked.OpenQuestions[i]

		if got.Number != want.Number || got.Title != want.Title {
			t.Errorf("question %d = {%d %q}, marked {%d %q}",
				i, got.Number, got.Title, want.Number, want.Title)
		}

		if len(got.Options) != len(want.Options) {
			t.Errorf("question %d has %d options, marked %d",
				i, len(got.Options), len(want.Options))
		}

		if (got.Resolved == nil) != (want.Resolved == nil) {
			t.Errorf("question %d resolved = %v, marked %v",
				i, got.Resolved != nil, want.Resolved != nil)
		}
	}
}

// TestParse_PartlyMarkedIsNotInferred pins the amended rule: markers, once
// present, are authoritative. A document that names two of its regions is read
// as naming two, and the rest are validate's region.missing findings.
func TestParse_PartlyMarkedIsNotInferred(t *testing.T) {
	t.Parallel()

	// Keep only the goals and non-goals markers, so the overview has a heading
	// and no marker.
	kept := keepMarkers(grammar(t), "goals", "non-goals")

	got := parse(t, kept)

	if got.Inferred {
		t.Error("Inferred = true for a partly marked document")
	}

	if got.Overview != "" {
		t.Errorf("Overview = %q, want empty: its region is not marked", got.Overview)
	}

	if got.DetailedDesign != "" {
		t.Errorf("DetailedDesign = %q, want empty: its region is not marked", got.DetailedDesign)
	}

	if len(got.Goals) != 2 {
		t.Errorf("Goals has %d items, want 2: its region is marked", len(got.Goals))
	}

	if len(got.NonGoals) != 1 {
		t.Errorf("NonGoals has %d items, want 1: its region is marked", len(got.NonGoals))
	}
}

// TestParse_MissingRegionIsNotAnError pins the division of labour: a section a
// document does not have leaves its field zero, and saying so is
// validate.Document's job rather than Parse's.
func TestParse_MissingRegionIsNotAnError(t *testing.T) {
	t.Parallel()

	got := parse(t, dropRegion(grammar(t), "decisions"))

	if got.Decisions != nil {
		t.Errorf("Decisions = %+v, want nil for a document with no decisions region", got.Decisions)
	}

	if len(got.Goals) != 2 {
		t.Errorf("Goals has %d items, want 2: dropping one region does not touch another",
			len(got.Goals))
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
			doc:  []byte("# DESIGN-0001\n\n## Overview\n\nSomething.\n"),
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

			if _, err := design.Parse(tt.doc); !errors.Is(err, tt.want) {
				t.Errorf("Parse error = %v, want %v", err, tt.want)
			}
		})
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

	if got.Title != "Grammar fixture" {
		t.Errorf("Title = %q after overwriting the input, want the parsed value", got.Title)
	}

	if !strings.HasPrefix(got.Overview, "A design document") {
		t.Errorf("Overview = %q, want the parsed value: a string field aliased the input", got.Overview)
	}

	if got.Goals[0].Text != "Read every section of the DESIGN template into its own field" {
		t.Errorf("Goals[0].Text = %q, want the parsed value", got.Goals[0].Text)
	}
}

// TestHeadings_IsACopy pins that a consumer cannot reshape what Parse infers.
func TestHeadings_IsACopy(t *testing.T) {
	t.Parallel()

	got := design.Headings()
	if len(got) == 0 {
		t.Fatal("Headings() is empty")
	}

	got[0].Kind = "mutated"

	if design.Headings()[0].Kind == "mutated" {
		t.Error("Headings() returned the package's own table, not a copy")
	}
}

// unmark removes every docz marker line, which is how a v1 document reads. The
// legacy ToC pair stays: it is in every v1 document, and it does not count as
// markers (kinds.ResolveRegions ignores it), which is what the inferred path
// is for.
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
func keepMarkers(doc []byte, want ...string) []byte {
	var out []string

	for _, line := range strings.Split(string(doc), "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "<!--docz:") {
			keep := false

			for _, kind := range want {
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

// dropRegion removes a whole region, markers and heading and content, so the
// document has no such section by either path.
func dropRegion(doc []byte, kind string) []byte {
	var out []string

	inside := false

	for _, line := range strings.Split(string(doc), "\n") {
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

	return []byte(strings.Join(out, "\n"))
}

// sameItems compares two item slices by text, which is what the inferred path
// has to agree with the marked one about. Lines differ by construction: the
// marker lines are gone.
func sameItems(got, want []kinds.Item) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i].Text != want[i].Text {
			return false
		}
	}

	return true
}
