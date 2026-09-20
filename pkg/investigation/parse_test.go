package investigation_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/investigation"
)

// grammar is the marked fixture every grammar assertion reads. One document
// rather than a literal per case: the rules interact — a field line sits
// inside the region whose body also holds it, a findings heading is a section
// and not a region — and a fixture a person can read is the only way to see
// that.
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

func parse(t *testing.T, doc []byte) investigation.Doc {
	t.Helper()

	got, err := investigation.Parse(doc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	return got
}

func TestParse_Frontmatter(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	for _, tt := range []struct{ field, got, want string }{
		{"ID", got.ID, "INV-0001"},
		{"Title", got.Title, "Grammar fixture"},
		{"Status", string(got.Status), "Concluded"},
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

func TestParse_StringFields(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	for _, tt := range []struct{ field, got, want string }{
		{
			"Question",
			got.Question,
			"Can a type package report document line numbers for a region it read through\n" +
				"the kinds readers, without the offset being applied twice?",
		},
		{
			"Hypothesis",
			got.Hypothesis,
			"It can. Every reader numbers lines from the start of the bytes it was handed,\n" +
				"so one addition of the region's start is enough at any depth.",
		},
		{"TriggeredBy", got.TriggeredBy, "DESIGN-0014 / issue #101"},
		{"Answer", got.Answer, "Yes, with the shift applied by the caller."},
		{
			"Recommendation",
			got.Recommendation,
			"Give kinds a Shift helper per value type and have every type package call it,\n" +
				"so the conversion has one definition instead of five.",
		},
	} {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.field, tt.got, tt.want)
		}
	}

	// The field lines stay in their region's body. A string field is its
	// region's body (DESIGN-0014 §2.9) and the field is the same bytes read a
	// second way, not a line cut out of the prose.
	if !strings.HasSuffix(got.Context, "**Triggered by:** DESIGN-0014 / issue #101") {
		t.Errorf("Context = %q, want it to end with the Triggered by field", got.Context)
	}

	if !strings.HasPrefix(got.Context, "The five type packages") {
		t.Errorf("Context = %q, want it to open with the prose", got.Context)
	}

	if !strings.HasSuffix(got.Conclusion, "**Answer:** Yes, with the shift applied by the caller.") {
		t.Errorf("Conclusion = %q, want it to end with the Answer field", got.Conclusion)
	}
}

func TestParse_ListFields(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	if len(got.Approach) != 3 {
		t.Fatalf("Approach has %d items, want 3", len(got.Approach))
	}

	if want := "Reproduce the drift with a fixture whose regions nest two deep."; got.Approach[0].Text != want {
		t.Errorf("Approach[0].Text = %q, want %q", got.Approach[0].Text, want)
	}

	if len(got.References) != 1 || got.References[0].URL == "" {
		t.Errorf("References = %+v, want one entry with a link", got.References)
	}

	if len(got.OpenQuestions) != 2 {
		t.Fatalf("OpenQuestions has %d entries, want 2", len(got.OpenQuestions))
	}

	first, second := got.OpenQuestions[0], got.OpenQuestions[1]

	if first.Resolved == nil || first.Resolved.Choice != "a" {
		t.Errorf("OpenQuestions[0].Resolved = %+v, want the (a) resolution", first.Resolved)
	}

	if second.Resolved != nil {
		t.Errorf("OpenQuestions[1].Resolved = %+v, want nil for an open question", second.Resolved)
	}

	if len(got.Decisions) != 1 {
		t.Fatalf("Decisions has %d rows, want 1", len(got.Decisions))
	}

	if got.Decisions[0].Number != 1 || got.Decisions[0].Resolution != "No, the caller shifts them" {
		t.Errorf("Decisions[0] = %+v, want the row for question 1", got.Decisions[0])
	}
}

// TestParse_Environment pins the table field: columns by position, and the
// template's empty placeholder row dropped.
func TestParse_Environment(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	got := parse(t, doc)

	// Four rows, one of them the template's empty placeholder.
	if len(got.Environment) != 3 {
		t.Fatalf("Environment has %d rows, want 3: the empty row is dropped", len(got.Environment))
	}

	first := got.Environment[0]

	if first.Component != "Go" || first.Value != "1.25.1" {
		t.Errorf("Environment[0] = %+v, want {Go 1.25.1}", first)
	}

	if want := lineOf(t, doc, "| Platform |"); got.Environment[2].Line != want {
		t.Errorf("Environment[2].Line = %d, want %d", got.Environment[2].Line, want)
	}
}

// TestParse_Findings pins that a finding is a level-3 heading whatever it is
// called. The template ships "Observation 1" and "Observation 2", and the
// fleet writes what it actually found, so nothing may depend on the wording.
func TestParse_Findings(t *testing.T) {
	t.Parallel()

	got := parse(t, grammar(t))

	if len(got.Findings) != 2 {
		t.Fatalf("Findings has %d sections, want one per level-3 heading", len(got.Findings))
	}

	if want := "The offset belongs to the caller"; got.Findings[0].Title != want {
		t.Errorf("Findings[0].Title = %q, want %q", got.Findings[0].Title, want)
	}

	if want := "Adding it in the reader double-counts a nested region"; got.Findings[1].Title != want {
		t.Errorf("Findings[1].Title = %q, want %q", got.Findings[1].Title, want)
	}

	if !strings.HasPrefix(got.Findings[0].Body, "Each reader reported a line one short") {
		t.Errorf("Findings[0].Body = %q, want the evidence under the heading", got.Findings[0].Body)
	}

	if strings.Contains(got.Findings[0].Body, "double-counts") {
		t.Errorf("Findings[0].Body = %q, want it to stop at the next heading", got.Findings[0].Body)
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
		{"Approach[0].Line", got.Approach[0].Line, "1. Reproduce the drift"},
		{"Environment[0].Line", got.Environment[0].Line, "| Go |"},
		{"Findings[0].Line", got.Findings[0].Line, "### The offset belongs"},
		{"OpenQuestions[0].Line", got.OpenQuestions[0].Line, "### 1. Should the readers"},
		{
			"OpenQuestions[0].Options[0].Line",
			got.OpenQuestions[0].Options[0].Line,
			"- a. No, the caller",
		},
		{
			"OpenQuestions[0].Resolved.Line",
			got.OpenQuestions[0].Resolved.Line,
			"> **Resolved 2026-09-20",
		},
		{"Decisions[0].Line", got.Decisions[0].Line, "| 1 | Should the readers"},
		{"References[0].Line", got.References[0].Line, "- [DESIGN-0014]"},
	}

	for _, tt := range tests {
		if want := lineOf(t, doc, tt.anchor); tt.got != want {
			t.Errorf("%s = %d, want %d (%q)", tt.field, tt.got, want, tt.anchor)
		}
	}
}

// TestParse_Verdict pins the one interpretation this package adds to the
// shared field rules: the answer's first word, read through its decoration.
func TestParse_Verdict(t *testing.T) {
	t.Parallel()

	doc := grammar(t)

	tests := []struct {
		name   string
		answer string
		want   investigation.Verdict
		render string
	}{
		{
			name:   "yes with the sentence after it",
			answer: "Yes, with the shift applied by the caller.",
			want:   investigation.VerdictYes,
			render: "yes",
		},
		{
			name:   "no with a trailing period",
			answer: "no.",
			want:   investigation.VerdictNo,
			render: "no",
		},
		{
			name:   "inconclusive, emphasised, with an em dash after it",
			answer: "**Inconclusive** — the benchmark never finished.",
			want:   investigation.VerdictInconclusive,
			render: "inconclusive",
		},
		{
			name:   "a first word that is not a verdict",
			answer: "Partly, and only for a document that carries markers.",
			want:   investigation.VerdictUnknown,
			render: "unknown",
		},
		{
			name:   "an answer line with nothing on it",
			answer: "",
			want:   investigation.VerdictUnknown,
			render: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parse(t, withAnswer(t, doc, tt.answer))

			if got.Verdict != tt.want {
				t.Errorf("Verdict = %v (%d), want %v", got.Verdict, int(got.Verdict), tt.want)
			}

			if got.Verdict.String() != tt.render {
				t.Errorf("Verdict.String() = %q, want %q", got.Verdict.String(), tt.render)
			}

			// The answer keeps its text, decoration and all: the sentence after
			// the verdict is usually the useful half of it.
			if got.Answer != tt.answer {
				t.Errorf("Answer = %q, want %q", got.Answer, tt.answer)
			}
		})
	}
}

// TestParse_Inferred is the compatibility path: the same document with every
// marker removed parses to the same fields.
//
// It compares against the marked parse rather than against a literal, so the
// two can never be updated apart. Lines are not compared, because removing the
// marker lines moves every line in the file; that they are document lines at
// all is TestParse_LinesAreDocumentLines' job.
func TestParse_Inferred(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	marked := parse(t, doc)
	bare := parse(t, unmark(doc))

	if !bare.Inferred {
		t.Error("Inferred = false for a document with no markers")
	}

	for _, tt := range []struct{ field, got, want string }{
		{"Question", bare.Question, marked.Question},
		{"Hypothesis", bare.Hypothesis, marked.Hypothesis},
		{"Context", bare.Context, marked.Context},
		{"TriggeredBy", bare.TriggeredBy, marked.TriggeredBy},
		{"Conclusion", bare.Conclusion, marked.Conclusion},
		{"Answer", bare.Answer, marked.Answer},
		{"Recommendation", bare.Recommendation, marked.Recommendation},
	} {
		if tt.got != tt.want {
			t.Errorf("%s = %q, marked %q", tt.field, tt.got, tt.want)
		}
	}

	if bare.Verdict != marked.Verdict {
		t.Errorf("Verdict = %v, marked %v", bare.Verdict, marked.Verdict)
	}

	for _, tt := range []struct {
		field     string
		got, want int
	}{
		{"Approach", len(bare.Approach), len(marked.Approach)},
		{"Environment", len(bare.Environment), len(marked.Environment)},
		{"Findings", len(bare.Findings), len(marked.Findings)},
		{"OpenQuestions", len(bare.OpenQuestions), len(marked.OpenQuestions)},
		{"Decisions", len(bare.Decisions), len(marked.Decisions)},
		{"References", len(bare.References), len(marked.References)},
	} {
		if tt.got != tt.want {
			t.Fatalf("inferred %d %s, marked %d", tt.got, tt.field, tt.want)
		}
	}

	for i := range bare.Approach {
		if bare.Approach[i].Text != marked.Approach[i].Text {
			t.Errorf("Approach[%d].Text = %q, marked %q",
				i, bare.Approach[i].Text, marked.Approach[i].Text)
		}
	}

	// The cells rather than the whole row: Line differs by the marker lines
	// removed above it, which is what unmark does to every line in the file.
	for i := range bare.Environment {
		if bare.Environment[i].Component != marked.Environment[i].Component ||
			bare.Environment[i].Value != marked.Environment[i].Value {
			t.Errorf("Environment[%d] = %+v, marked %+v",
				i, bare.Environment[i], marked.Environment[i])
		}
	}

	for i := range bare.Findings {
		if bare.Findings[i].Title != marked.Findings[i].Title ||
			bare.Findings[i].Body != marked.Findings[i].Body {
			t.Errorf("Findings[%d] = {%q %q}, marked {%q %q}",
				i, bare.Findings[i].Title, bare.Findings[i].Body,
				marked.Findings[i].Title, marked.Findings[i].Body)
		}
	}

	for i := range bare.OpenQuestions {
		if bare.OpenQuestions[i].Number != marked.OpenQuestions[i].Number ||
			bare.OpenQuestions[i].Title != marked.OpenQuestions[i].Title ||
			len(bare.OpenQuestions[i].Options) != len(marked.OpenQuestions[i].Options) {
			t.Errorf("OpenQuestions[%d] = %+v, marked %+v",
				i, bare.OpenQuestions[i], marked.OpenQuestions[i])
		}
	}
}

// TestParse_PartlyMarkedIsNotInferred pins the amended rule: markers, once
// present, are authoritative. A document that names two regions is read as
// naming two, and the rest are validate's region.missing findings.
func TestParse_PartlyMarkedIsNotInferred(t *testing.T) {
	t.Parallel()

	doc := grammar(t)

	// Keep the context and conclusion markers, so the question has a heading
	// and no marker.
	kept := keepMarkers(doc, "context", "conclusion")

	got := parse(t, kept)

	if got.Inferred {
		t.Error("Inferred = true for a partly marked document")
	}

	if got.Question != "" {
		t.Errorf("Question = %q, want empty: its region is not marked", got.Question)
	}

	if got.Findings != nil {
		t.Errorf("Findings = %+v, want nil: the findings region is not marked", got.Findings)
	}

	if want := "DESIGN-0014 / issue #101"; got.TriggeredBy != want {
		t.Errorf("TriggeredBy = %q, want %q: its region is marked", got.TriggeredBy, want)
	}

	if got.Verdict != investigation.VerdictYes {
		t.Errorf("Verdict = %v, want yes: the conclusion region is marked", got.Verdict)
	}
}

// TestParse_Errors pins the two things Parse fails for, and that everything
// else is a zero field rather than an error: an investigation is parsed while
// it is still being written.
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
			doc:  []byte("# INV-0001\n\n## Question\n\nCan it?\n"),
			want: document.ErrNoFrontmatter,
		},
		{
			name: "CRLF line endings",
			doc:  []byte(strings.ReplaceAll(string(doc), "\n", "\r\n")),
			want: document.ErrUnsupportedLineEndings,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := investigation.Parse(tt.doc); !errors.Is(err, tt.want) {
				t.Errorf("Parse error = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestParse_NoRegionsIsNotAnError is the other half of the rule above. There
// is no investigation equivalent of impl.ErrNoPhases: a document with the
// frontmatter and nothing else is a new investigation, not a broken one.
func TestParse_NoRegionsIsNotAnError(t *testing.T) {
	t.Parallel()

	got := parse(t, []byte("---\nid: INV-0002\ntitle: Empty\nstatus: Open\n---\n\n# INV-0002\n"))

	if got.ID != "INV-0002" {
		t.Errorf("ID = %q, want INV-0002", got.ID)
	}

	if got.Question != "" || got.Findings != nil || got.Verdict != investigation.VerdictUnknown {
		t.Errorf("Doc = %+v, want every content field zero", got)
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

	if got.Answer != "Yes, with the shift applied by the caller." {
		t.Errorf("Answer = %q after overwriting the input, want the parsed value", got.Answer)
	}

	if len(got.Findings) == 0 || got.Findings[0].Title != "The offset belongs to the caller" {
		t.Error("a finding's Title aliased the input")
	}
}

// Headings is the table a consumer runs validate.Document with, and a copy so
// a caller cannot reshape what Parse does.
func TestHeadings_IsACopy(t *testing.T) {
	t.Parallel()

	first := investigation.Headings()
	if len(first) == 0 {
		t.Fatal("Headings() is empty")
	}

	first[0].Text = "mutated"

	if investigation.Headings()[0].Text == "mutated" {
		t.Error("Headings() returned the package's own table")
	}
}

// withAnswer rewrites the fixture's answer line, so a verdict case names only
// the answer it is about.
func withAnswer(t *testing.T, doc []byte, answer string) []byte {
	t.Helper()

	lines := strings.Split(string(doc), "\n")
	found := 0

	for i, line := range lines {
		if strings.HasPrefix(line, "**Answer:**") {
			lines[i] = strings.TrimSpace("**Answer:** " + answer)
			found++
		}
	}

	if found != 1 {
		t.Fatalf("fixture has %d answer lines, want exactly one", found)
	}

	return []byte(strings.Join(lines, "\n"))
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
func keepMarkers(doc []byte, kinds ...string) []byte {
	var out []string

	for _, line := range strings.Split(string(doc), "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "<!--docz:") {
			keep := false

			for _, kind := range kinds {
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

// TestParse_RenderedTemplate parses what `docz create inv` writes.
//
// Two fields come back non-empty and neither carries information: the template
// ships the bold labels with nothing after them, and two placeholder findings
// headings. Everything else is zero for a stated reason. A field that started
// coming back filled would mean a placeholder had leaked into the parsed model.
func TestParse_RenderedTemplate(t *testing.T) {
	t.Parallel()

	got := parse(t, renderedTemplate(t))

	if got.Inferred {
		t.Error("Inferred = true: the template carries markers")
	}

	if got.ID != "INV-0001" || got.Title != "A placeholder title" {
		t.Errorf("ID/Title = %q/%q, want INV-0001/A placeholder title", got.ID, got.Title)
	}

	// The label is there and its value is not, which is the state kinds.Field's
	// second return exists to distinguish. Both fields are found-and-empty, so
	// inv.context.no-trigger does not fire on a fresh document while
	// inv.conclusion.no-answer does not either — the status is Open.
	if want := "**Triggered by:**"; got.Context != want {
		t.Errorf("Context = %q, want %q: the label with no value", got.Context, want)
	}

	if want := "**Answer:**"; got.Conclusion != want {
		t.Errorf("Conclusion = %q, want %q: the label with no value", got.Conclusion, want)
	}

	if got.TriggeredBy != "" || got.Answer != "" {
		t.Errorf("TriggeredBy/Answer = %q/%q, want both empty", got.TriggeredBy, got.Answer)
	}

	if got.Verdict != investigation.VerdictUnknown {
		t.Errorf("Verdict = %v, want VerdictUnknown for an empty answer", got.Verdict)
	}

	// The findings region ships two level-3 placeholder headings, and they are
	// sections with no body. Findings headings are the author's own, so the
	// reader cannot tell a placeholder from a real one — validate does not
	// either, and that is why there is no inv.findings rule.
	if len(got.Findings) != 2 {
		t.Fatalf("len(Findings) = %d, want the template's 2", len(got.Findings))
	}

	for i, section := range got.Findings {
		if section.Body != "" {
			t.Errorf("Findings[%d].Body = %q, want empty: the placeholder is a comment",
				i, section.Body)
		}
	}

	for _, tt := range []struct {
		field, why string
		empty      bool
	}{
		{"Question", "the placeholder is an HTML comment", got.Question == ""},
		{"Hypothesis", "the placeholder is an HTML comment", got.Hypothesis == ""},
		{"Approach", "the placeholder is a bare \"-\" bullet", len(got.Approach) == 0},
		{"Environment", "its rows have no component and no value", len(got.Environment) == 0},
		{"Recommendation", "the placeholder is an HTML comment", got.Recommendation == ""},
		{"References", "the placeholder is an HTML comment", len(got.References) == 0},
		{"OpenQuestions", "the template ships no such section", got.OpenQuestions == nil},
		{"Decisions", "the template ships no such section", got.Decisions == nil},
	} {
		if !tt.empty {
			t.Errorf("%s is filled, want empty: %s", tt.field, tt.why)
		}
	}
}
