package rfc_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/rfc"
)

// grammar is the marked fixture every grammar assertion reads. One document
// rather than a literal per case: the rules interact — an alternative's label
// sits inside its bold lead-in, a risks row with no mitigation is still a row —
// and a fixture a person can read is the only way to see that.
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
// paragraph into the fixture does not renumber thirty expectations. It fails
// on a second match, because an ambiguous anchor would silently assert
// against the wrong line.
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

func parse(t *testing.T, doc []byte) rfc.Doc {
	t.Helper()

	got, err := rfc.Parse(doc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	return got
}

func TestParse_Frontmatter(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	for _, tt := range []struct{ field, got, want string }{
		{"ID", got.ID, "RFC-0001"},
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

// TestParse_StringFields pins the three fields that are a region's body, and
// with them the one field rule that is not obvious: the problem statement
// keeps its Supporting Data subsection.
func TestParse_StringFields(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	if !strings.HasPrefix(got.Summary, "Publish a typed reader") {
		t.Errorf("Summary = %q, want it to open with the first prose line", got.Summary)
	}

	if !strings.HasSuffix(got.Summary, "walk itself.") {
		t.Errorf("Summary = %q, want the wrapped lines kept verbatim", got.Summary)
	}

	// The subsection is part of the statement, heading and all: the template
	// does not mark it, so there is nothing to read it separately with.
	if !strings.Contains(got.Problem, "### Supporting Data") {
		t.Errorf("Problem = %q, want the Supporting Data heading included", got.Problem)
	}

	if !strings.HasSuffix(got.Problem, "rather than the fourth.") {
		t.Errorf("Problem = %q, want it to run through the subsection's last line", got.Problem)
	}

	if !strings.HasPrefix(got.Proposal, "Ship `pkg/rfc`") {
		t.Errorf("Proposal = %q, want the inline markdown kept verbatim", got.Proposal)
	}

	// The template's guidance comments are not content, so a region that holds
	// only one yields "" rather than the instructions it shipped with.
	if strings.Contains(got.Summary, "<!--") {
		t.Errorf("Summary = %q, want HTML comments removed", got.Summary)
	}
}

func TestParse_Alternatives(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	if len(got.Alternatives) != 3 {
		t.Fatalf("got %d alternatives, want 3", len(got.Alternatives))
	}

	// The label sits inside the bold lead-in, which is how all three of this
	// repo's ADRs write one.
	first := got.Alternatives[0]
	if first.Label != "a" || first.Title != "Leave the parsing to each consumer." {
		t.Errorf("alternatives[0] = {Label:%q Title:%q}, want {\"a\" \"Leave the parsing to each consumer.\"}",
			first.Label, first.Title)
	}

	if !strings.HasPrefix(first.Text, "Rejected:") || !strings.HasSuffix(first.Text, "than the fix.") {
		t.Errorf("Text = %q, want the wrapped lines folded with one space", first.Text)
	}

	// A bullet with a bold lead-in and no label keeps the title and reports no
	// label, rather than reading two letters of the title as one.
	if second := got.Alternatives[1]; second.Label != "" ||
		second.Title != "Ship a generic markdown model." {
		t.Errorf("alternatives[1] = {Label:%q Title:%q}, want no label", second.Label, second.Title)
	}

	// A one-line alternative has no title of its own: the whole bullet is what
	// it says, and inventing a title by cutting at the first period would
	// mangle it.
	if third := got.Alternatives[2]; third.Title != "" ||
		third.Text != "Wait for the schema work to land first." {
		t.Errorf("alternatives[2] = {Title:%q Text:%q}, want the whole bullet as Text",
			third.Title, third.Text)
	}
}

func TestParse_Risks(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	got := parse(t, doc)

	// Four rows, one of them the template's empty placeholder.
	if len(got.Risks) != 3 {
		t.Fatalf("Risks has %d rows, want 3: the empty row is dropped", len(got.Risks))
	}

	first := got.Risks[0]

	if first.Risk != "The template is renamed" || first.Impact != "High" ||
		first.Likelihood != "Low" || first.Mitigation != "A test pins the heading table to it" {
		t.Errorf("Risks[0] = %+v, want the four cells mapped by column position", first)
	}

	// A row that names a risk and nothing else is kept: it is what
	// rfc.risks.no-mitigation reports, and the finding cannot be made about a
	// row the parser threw away.
	if last := got.Risks[2]; last.Risk == "" || last.Mitigation != "" {
		t.Errorf("Risks[2] = %+v, want the row with an empty mitigation kept", last)
	}

	if want := lineOf(t, doc, "| The template is renamed |"); first.Line != want {
		t.Errorf("Line = %d, want %d", first.Line, want)
	}
}

func TestParse_Criteria(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	if len(got.Criteria) != 2 {
		t.Fatalf("got %d criteria, want 2", len(got.Criteria))
	}

	// Executable iff the bullet opens with a code span: the criterion is a
	// command someone can run, not one that merely mentions a filename.
	if first := got.Criteria[0]; !first.Executable || first.Command != "go test ./pkg/rfc/..." {
		t.Errorf("criteria[0] = {Executable:%v Command:%q}, want {true \"go test ./pkg/rfc/...\"}",
			first.Executable, first.Command)
	}

	if second := got.Criteria[1]; second.Executable || second.Command != "" {
		t.Errorf("criteria[1] = {Executable:%v Command:%q}, want a prose criterion",
			second.Executable, second.Command)
	}
}

func TestParse_OpenQuestions(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	if len(got.OpenQuestions) != 2 {
		t.Fatalf("got %d open questions, want 2", len(got.OpenQuestions))
	}

	resolved, open := got.OpenQuestions[0], got.OpenQuestions[1]

	if resolved.Number != 1 || resolved.Title != "Should the risks table be read by column name?" {
		t.Errorf("questions[0] = {Number:%d Title:%q}, want the numbered heading read",
			resolved.Number, resolved.Title)
	}

	if resolved.Resolved == nil {
		t.Fatal("questions[0].Resolved = nil, want the resolution blockquote")
	}

	if resolved.Resolved.Date != "2026-09-20" || resolved.Resolved.Choice != "a" {
		t.Errorf("Resolved = {Date:%q Choice:%q}, want {\"2026-09-20\" \"a\"}",
			resolved.Resolved.Date, resolved.Resolved.Choice)
	}

	if len(resolved.Options) != 2 || !resolved.Options[0].Recommended {
		t.Errorf("questions[0].Options = %+v, want two, the first recommended", resolved.Options)
	}

	// The second question is what rfc.status.open-question reads: Resolved is
	// nil while the question is open.
	if open.Number != 2 || open.Resolved != nil {
		t.Errorf("questions[1] = {Number:%d Resolved:%+v}, want an unresolved question 2",
			open.Number, open.Resolved)
	}
}

func TestParse_References(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	if len(got.References) != 2 {
		t.Fatalf("got %d references, want 2", len(got.References))
	}

	if url := got.References[0].URL; url != "../design/0014-the-docz-api-as-one-unit.md" {
		t.Errorf("References[0].URL = %q, want the link target", url)
	}

	// A bullet with no link is still a reference, reported with an empty URL.
	if got.References[1].URL != "" {
		t.Errorf("References[1].URL = %q, want empty", got.References[1].URL)
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
		{"Alternatives[0].Line", got.Alternatives[0].Line, "- **A. Leave the parsing"},
		{"Risks[0].Line", got.Risks[0].Line, "| The template is renamed |"},
		{"Criteria[0].Line", got.Criteria[0].Line, "- `go test ./pkg/rfc/...` passes"},
		{"OpenQuestions[0].Line", got.OpenQuestions[0].Line, "### 1. Should the risks table"},
		{"OpenQuestions[0].Options[0].Line", got.OpenQuestions[0].Options[0].Line, "- a. No, by position"},
		{"OpenQuestions[0].Resolved.Line", got.OpenQuestions[0].Resolved.Line, "> **Resolved 2026-09-20:"},
		{"References[0].Line", got.References[0].Line, "- [DESIGN-0014]"},
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
// two can never be updated apart. Only Inferred and the line numbers differ —
// the lines because removing the marker lines moves every line of the
// document — and clearLines takes both out of the comparison.
func TestParse_Inferred(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	marked := parse(t, doc)
	bare := parse(t, unmark(doc))

	if !bare.Inferred {
		t.Error("Inferred = false for a document with no markers")
	}

	clearLines(&marked)
	clearLines(&bare)

	if !reflect.DeepEqual(bare, marked) {
		t.Errorf("inferred parse differs from the marked one:\ninferred %+v\nmarked   %+v",
			bare, marked)
	}
}

// TestParse_InferredLinesAreStillDocumentLines pins the other half of the
// inference path: an inferred span opens on the line before its heading, so a
// Line that was right for a marked document has to stay right for a bare one.
func TestParse_InferredLinesAreStillDocumentLines(t *testing.T) {
	t.Parallel()

	doc := unmark(grammar(t))
	got := parse(t, doc)

	if want := lineOf(t, doc, "| The template is renamed |"); got.Risks[0].Line != want {
		t.Errorf("Risks[0].Line = %d, want %d", got.Risks[0].Line, want)
	}

	if want := lineOf(t, doc, "- a. No, by position"); got.OpenQuestions[0].Options[0].Line != want {
		t.Errorf("Options[0].Line = %d, want %d", got.OpenQuestions[0].Options[0].Line, want)
	}
}

// TestParse_PartlyMarkedIsNotInferred pins the amended rule: markers, once
// present, are authoritative. A document that names two of its regions is read
// as naming two, and the rest are validate's region.missing findings.
func TestParse_PartlyMarkedIsNotInferred(t *testing.T) {
	t.Parallel()

	// Keep only the risks and criteria markers, so the summary has a heading
	// and no marker.
	kept := keepMarkers(grammar(t), "risks", "criteria")

	got := parse(t, kept)

	if got.Inferred {
		t.Error("Inferred = true for a partly marked document")
	}

	if got.Summary != "" {
		t.Errorf("Summary = %q, want empty: its region is not marked", got.Summary)
	}

	if got.Alternatives != nil {
		t.Errorf("Alternatives = %+v, want nil: its region is not marked", got.Alternatives)
	}

	if len(got.Risks) != 3 {
		t.Errorf("Risks has %d rows, want 3: its region is marked", len(got.Risks))
	}

	if len(got.Criteria) != 2 {
		t.Errorf("Criteria has %d entries, want 2: its region is marked", len(got.Criteria))
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
			doc:  []byte("# RFC-0001\n\n## Summary\n\nA proposal.\n"),
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

			if _, err := rfc.Parse(tt.doc); !errors.Is(err, tt.want) {
				t.Errorf("Parse error = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestParse_MissingRegionIsNotAnError pins the division of labour: a document
// missing a region parses with that field zero. There is no rfc equivalent of
// impl.ErrNoPhases, because an RFC with nothing but a summary is an early
// draft rather than a document with nothing of the type in it.
func TestParse_MissingRegionIsNotAnError(t *testing.T) {
	t.Parallel()

	got := parse(t, keepMarkers(grammar(t), "summary"))

	if got.Summary == "" {
		t.Error("Summary is empty, want the one marked region read")
	}

	for _, tt := range []struct {
		field string
		empty bool
	}{
		{"Problem", got.Problem == ""},
		{"Proposal", got.Proposal == ""},
		{"Alternatives", got.Alternatives == nil},
		{"Risks", got.Risks == nil},
		{"Criteria", got.Criteria == nil},
		{"OpenQuestions", got.OpenQuestions == nil},
		{"References", got.References == nil},
	} {
		if !tt.empty {
			t.Errorf("%s is set, want zero for an unmarked region", tt.field)
		}
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

	if got.Risks[0].Risk != "The template is renamed" {
		t.Errorf("Risks[0].Risk = %q, want the parsed value: a cell aliased the input",
			got.Risks[0].Risk)
	}

	if got.Alternatives[0].Title != "Leave the parsing to each consumer." {
		t.Error("an alternative's Title aliased the input")
	}
}

// clearLines zeroes every Line and clears Inferred, so two parses of the same
// content can be compared for everything but where the bytes sat.
//
// A pointer because Doc is well past the linter's size threshold, and in place
// because the only caller has no use for the original afterwards.
func clearLines(d *rfc.Doc) {
	d.Inferred = false

	for i := range d.Alternatives {
		d.Alternatives[i].Line = 0
	}

	for i := range d.Risks {
		d.Risks[i].Line = 0
	}

	for i := range d.Criteria {
		d.Criteria[i].Line = 0
	}

	for i := range d.References {
		d.References[i].Line = 0
	}

	for i := range d.OpenQuestions {
		question := &d.OpenQuestions[i]
		question.Line = 0

		for k := range question.Options {
			question.Options[k].Line = 0
		}

		if question.Resolved != nil {
			resolution := *question.Resolved
			resolution.Line = 0
			question.Resolved = &resolution
		}
	}
}

// unmark removes every docz marker line, which is how a v1 document reads. The
// legacy ToC pair is left alone: every v1 document has one, and inference
// exists for exactly those documents.
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
func keepMarkers(doc []byte, keep ...string) []byte {
	var out []string

	for _, line := range strings.Split(string(doc), "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "<!--docz:") {
			kept := false

			for _, kind := range keep {
				if strings.HasPrefix(trimmed, "<!--docz:"+kind+":") {
					kept = true
				}
			}

			if !kept {
				continue
			}
		}

		out = append(out, line)
	}

	return []byte(strings.Join(out, "\n"))
}
