package design_test

import (
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/design"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
)

// codes returns the findings' codes in order, which is what a table case
// asserts: the wording of a Detail is not a contract, the code is.
func codes(findings []validate.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.Code)
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

// replace makes a one-substitution edit, failing loudly at use time if the
// fixture no longer contains the text.
func replace(old, with string) func(string) string {
	return func(doc string) string {
		if !strings.Contains(doc, old) {
			panic("fixture has no " + old)
		}

		return strings.Replace(doc, old, with, 1)
	}
}

// TestValidate_CleanDocument is the guard that keeps the rules from turning
// into noise. The grammar fixture is a correct DESIGN document — a Draft with
// one question resolved, one still open, and a decisions table that agrees with
// both — so it earns nothing at all.
func TestValidate_CleanDocument(t *testing.T) {
	t.Parallel()

	if got := design.Validate(grammar(t)); len(got) != 0 {
		t.Errorf("Validate() = %+v, want no findings", got)
	}
}

// TestValidate_Rules is one case per code: a document that trips it and the
// codes it produces.
func TestValidate_Rules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		edit func(string) string
		want []string
	}{
		{
			name: "a goals region with no items",
			edit: replace("- Read every section of the DESIGN template into its own field\n"+
				"- Keep the detailed design region whole, numbered subsections and all\n", ""),
			want: []string{design.CodeGoalsEmpty},
		},
		{
			// Approved with question 2 still open. Question 1 is resolved, so the
			// rule reports one finding rather than one per question.
			name: "an open question past approval",
			edit: replace("status: Draft", "status: Approved"),
			want: []string{design.CodeStatusOpenQuestion},
		},
		{
			// Folded, because the rule is about what the word means and a repo may
			// spell its statuses in its own case.
			name: "a status compared folded",
			edit: replace("status: Draft", "status: implemented"),
			want: []string{design.CodeStatusOpenQuestion},
		},
		{
			name: "a status that is not settled",
			edit: replace("status: Draft", "status: In Review"),
			want: nil,
		},
		{
			name: "a resolved question with no decisions row",
			edit: replace(
				"| 1 | Does the region keep its subsections? | Yes, the body is reported whole |\n", ""),
			want: []string{design.CodeDecisionsMismatch},
		},
		{
			name: "a decisions row with no question",
			edit: replace("| 1 | Does the region keep its subsections? | Yes, the body is reported whole |",
				"| 1 | Does the region keep its subsections? | Yes, the body is reported whole |\n"+
					"| 9 | A question nobody asked | Nothing |"),
			want: []string{design.CodeDecisionsMismatch},
		},
		{
			// A row the corpus writes with an em dash in the number column: it
			// records an amendment rather than answering a numbered question, so it
			// matches nothing and cannot be quietly attached to the first question.
			name: "a decisions row with no number",
			edit: replace("| 1 | Does the region keep its subsections? | Yes, the body is reported whole |",
				"| 1 | Does the region keep its subsections? | Yes, the body is reported whole |\n"+
					"| — | An amendment | Noted |"),
			want: []string{design.CodeDecisionsMismatch},
		},
		{
			name: "a row and a question that miss each other",
			edit: replace("| 1 | Does the region keep its subsections?",
				"| 9 | Does the region keep its subsections?"),
			want: []string{design.CodeDecisionsMismatch, design.CodeDecisionsMismatch},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := design.Validate([]byte(tt.edit(string(grammar(t)))))

			if !equal(codes(got), tt.want) {
				t.Errorf("Validate() = %v, want %v\nfindings: %+v", codes(got), tt.want, got)
			}
		})
	}
}

// TestValidate_NoDecisionsRegionIsSilent pins the reason the mismatch rule
// resolves the regions a second time. The DESIGN template ships no decisions
// section, so a design that keeps its resolutions only in the questions'
// blockquotes is written correctly — and Doc cannot tell that apart from a
// present-but-empty table, because both leave Decisions nil.
func TestValidate_NoDecisionsRegionIsSilent(t *testing.T) {
	t.Parallel()

	doc := dropRegion(grammar(t), "decisions")

	// The precondition the case rests on: question 1 is resolved, so the
	// question side of the rule would fire if the region counted as present.
	parsed := parse(t, doc)
	if len(parsed.OpenQuestions) == 0 || parsed.OpenQuestions[0].Resolved == nil {
		t.Fatal("fixture no longer has a resolved question, so this case proves nothing")
	}

	if got := design.Validate(doc); len(got) != 0 {
		t.Errorf("Validate() = %+v, want no findings for a document with no decisions region", got)
	}
}

// TestValidate_FindingLines pins that a finding points at the line a person has
// to edit. A code with the wrong line is worse than no code: it sends the
// reader somewhere correct.
func TestValidate_FindingLines(t *testing.T) {
	t.Parallel()

	doc := grammar(t)

	goalsEmpty := []byte(replace(
		"- Read every section of the DESIGN template into its own field\n"+
			"- Keep the detailed design region whole, numbered subsections and all\n", "")(string(doc)))

	got := design.Validate(goalsEmpty)
	if len(got) != 1 {
		t.Fatalf("Validate() = %+v, want one finding", got)
	}

	// The heading, not the marker line above it: the heading is what a reader
	// recognises, and an inferred region has no marker at all.
	if want := lineOf(t, goalsEmpty, "### Goals"); got[0].Line != want {
		t.Errorf("goals.empty Line = %d, want the heading at %d", got[0].Line, want)
	}

	if got[0].Severity != validate.Warning || got[0].Kind != "goals" {
		t.Errorf("goals.empty = {Severity:%v Kind:%q}, want {warning goals}",
			got[0].Severity, got[0].Kind)
	}

	approved := []byte(replace("status: Draft", "status: Approved")(string(doc)))

	got = design.Validate(approved)
	if len(got) != 1 {
		t.Fatalf("Validate() = %+v, want one finding", got)
	}

	if want := lineOf(t, approved, "### 2. Where does an unresolved question"); got[0].Line != want {
		t.Errorf("status.open-question Line = %d, want the question heading at %d", got[0].Line, want)
	}

	if got[0].Severity != validate.Error {
		t.Errorf("status.open-question Severity = %v, want Error", got[0].Severity)
	}
}

// TestValidate_MismatchLines pins the two directions of the mismatch rule at
// the two different lines they belong to: the question for a resolution the
// table does not record, the row for a row nobody asked for.
func TestValidate_MismatchLines(t *testing.T) {
	t.Parallel()

	edited := []byte(replace("| 1 | Does the region keep its subsections?",
		"| 9 | Does the region keep its subsections?")(string(grammar(t))))

	got := design.Validate(edited)
	if len(got) != 2 {
		t.Fatalf("Validate() = %+v, want two findings", got)
	}

	if want := lineOf(t, edited, "### 1. Does the detailed design region keep"); got[0].Line != want {
		t.Errorf("question-side Line = %d, want the question heading at %d", got[0].Line, want)
	}

	if got[0].Kind != "open-questions" {
		t.Errorf("question-side Kind = %q, want open-questions", got[0].Kind)
	}

	if want := lineOf(t, edited, "| 9 | Does the region keep its subsections?"); got[1].Line != want {
		t.Errorf("row-side Line = %d, want the table row at %d", got[1].Line, want)
	}

	if got[1].Kind != "decisions" {
		t.Errorf("row-side Kind = %q, want decisions", got[1].Kind)
	}

	for _, f := range got {
		if f.Severity != validate.Warning {
			t.Errorf("Severity = %v, want Warning: the table is a convenience", f.Severity)
		}
	}
}

// TestValidate_ParseErrorIsOneFinding pins the contract for a document Parse
// rejects: one finding, never a guess at the rest.
func TestValidate_ParseErrorIsOneFinding(t *testing.T) {
	t.Parallel()

	doc := grammar(t)

	tests := []struct {
		name string
		doc  []byte
	}{
		{
			name: "no frontmatter",
			doc:  []byte("## Overview\n\nSomething.\n\n### Goals\n\n- One\n"),
		},
		{
			name: "CR line endings",
			doc:  []byte(strings.ReplaceAll(string(doc), "\n", "\r\n")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := design.Validate(tt.doc)

			if len(got) != 1 {
				t.Fatalf("Validate() returned %d findings, want exactly 1: %+v", len(got), got)
			}

			if got[0].Code != design.CodeParse {
				t.Errorf("Code = %q, want %q", got[0].Code, design.CodeParse)
			}

			if got[0].Severity != validate.Error {
				t.Errorf("Severity = %v, want Error", got[0].Severity)
			}

			if got[0].Detail == "" {
				t.Error("Detail is empty: a rejected document must say why")
			}

			if strings.HasPrefix(got[0].Detail, "design: ") {
				t.Errorf("Detail = %q, want the package prefix trimmed", got[0].Detail)
			}
		})
	}
}

// TestValidate_InferredDocumentIsValidatedToo pins that the rules do not depend
// on markers. Every v1-created design reads through the inferred path, so a
// rule that only fired on a marked document would be silent for the whole
// existing fleet.
func TestValidate_InferredDocumentIsValidatedToo(t *testing.T) {
	t.Parallel()

	bare := unmark([]byte(replace("status: Draft", "status: Approved")(string(grammar(t)))))

	if !parse(t, bare).Inferred {
		t.Fatal("the fixture is not inferred, so this case proves nothing")
	}

	got := design.Validate(bare)

	want := []string{design.CodeStatusOpenQuestion}
	if !equal(codes(got), want) {
		t.Errorf("Validate() = %v, want %v\nfindings: %+v", codes(got), want, got)
	}

	if w := lineOf(t, bare, "### 2. Where does an unresolved question"); got[0].Line != w {
		t.Errorf("Line = %d, want the question heading at %d", got[0].Line, w)
	}
}
