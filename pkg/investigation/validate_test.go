package investigation_test

import (
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
	"github.com/donaldgifford/docz/v2/pkg/investigation"
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

// TestValidate_CleanDocument is the guard that keeps the rules from turning
// into noise. The grammar fixture is a correct, concluded INV document, so it
// must report nothing at all.
func TestValidate_CleanDocument(t *testing.T) {
	t.Parallel()

	got := investigation.Validate(grammar(t))
	if len(got) != 0 {
		t.Errorf("Validate() = %+v, want no findings for a correct document", got)
	}
}

// TestValidate_Rules is one case per code: a document that trips it and the
// codes it produces.
func TestValidate_Rules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		edit func(*testing.T, []byte) []byte
		want []string
	}{
		{
			name: "a context region with no Triggered by field",
			edit: withoutTrigger,
			want: []string{investigation.CodeContextNoTrigger},
		},
		{
			name: "a concluded document with no Answer field",
			edit: withoutAnswer,
			want: []string{investigation.CodeConclusionNoAnswer},
		},
		{
			name: "a status of Inconclusive with no Answer field",
			edit: both(replace("status: Concluded", "status: Inconclusive"), withoutAnswer),
			want: []string{investigation.CodeConclusionNoAnswer},
		},
		{
			name: "an answer whose first word is not a verdict",
			edit: answer("Partly, and only for a document that carries markers."),
			want: []string{investigation.CodeConclusionVerdict},
		},
		{
			name: "an answer line with nothing on it",
			edit: answer(""),
			want: []string{investigation.CodeConclusionNoAnswer},
		},
		{
			name: "both rules at once, in document order",
			edit: both(withoutTrigger, answer("Probably not.")),
			want: []string{
				investigation.CodeContextNoTrigger,
				investigation.CodeConclusionVerdict,
			},
		},
		{
			// The status gate: an investigation still running is allowed to have
			// no answer yet, which is the state it spends most of its life in.
			name: "an unfinished document with no Answer field",
			edit: both(replace("status: Concluded", "status: In Progress"), withoutAnswer),
			want: nil,
		},
		{
			// A document with no context region at all is region.missing in the
			// generic tier, so this tier says nothing about it.
			name: "no context region",
			edit: dropRegion("context"),
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := investigation.Validate(tt.edit(t, grammar(t)))

			if !equal(codes(got), tt.want) {
				t.Errorf("Validate() = %v, want %v\nfindings: %+v", codes(got), tt.want, got)
			}
		})
	}
}

// TestValidate_Severities pins which of the three rules gates a CI run. Only
// a concluded document with no answer is an error: the other two describe a
// document that reads correctly and says less than it could.
func TestValidate_Severities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		edit func(*testing.T, []byte) []byte
		code string
		want validate.Severity
	}{
		{
			name: "no trigger",
			edit: withoutTrigger,
			code: investigation.CodeContextNoTrigger,
			want: validate.Warning,
		},
		{
			name: "no answer",
			edit: withoutAnswer,
			code: investigation.CodeConclusionNoAnswer,
			want: validate.Error,
		},
		{
			name: "no verdict",
			edit: answer("Probably not."),
			code: investigation.CodeConclusionVerdict,
			want: validate.Warning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := investigation.Validate(tt.edit(t, grammar(t)))

			if len(got) != 1 {
				t.Fatalf("Validate() = %+v, want one finding", got)
			}

			if got[0].Code != tt.code {
				t.Fatalf("Code = %q, want %q", got[0].Code, tt.code)
			}

			if got[0].Severity != tt.want {
				t.Errorf("Severity = %v, want %v", got[0].Severity, tt.want)
			}

			if got[0].Detail == "" {
				t.Error("Detail is empty: a finding has to say what is wrong")
			}
		})
	}
}

// TestValidate_FindingLines pins that a finding points at the region a person
// has to edit. A code with the wrong line is worse than no code: it sends the
// reader somewhere correct.
func TestValidate_FindingLines(t *testing.T) {
	t.Parallel()

	doc := withoutTrigger(t, grammar(t))

	got := investigation.Validate(doc)
	if len(got) != 1 {
		t.Fatalf("Validate() = %+v, want one finding", got)
	}

	if want := lineOf(t, doc, "<!--docz:context:start-->"); got[0].Line != want {
		t.Errorf("Line = %d, want the context region at %d", got[0].Line, want)
	}

	if got[0].Kind != "context" {
		t.Errorf("Kind = %q, want the region the finding is about", got[0].Kind)
	}

	noAnswer := withoutAnswer(t, grammar(t))

	conclusion := investigation.Validate(noAnswer)
	if len(conclusion) != 1 {
		t.Fatalf("Validate() = %+v, want one finding", conclusion)
	}

	if want := lineOf(t, noAnswer, "<!--docz:conclusion:start-->"); conclusion[0].Line != want {
		t.Errorf("Line = %d, want the conclusion region at %d", conclusion[0].Line, want)
	}
}

// TestValidate_ParseErrorIsOneFinding pins the contract for a document Parse
// rejects: one finding, never a guess at the rest.
func TestValidate_ParseErrorIsOneFinding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		doc  []byte
	}{
		{
			name: "no frontmatter",
			doc:  []byte("# INV-0001\n\n## Question\n\nCan it?\n"),
		},
		{
			name: "CRLF line endings",
			doc:  []byte(strings.ReplaceAll(string(grammar(t)), "\n", "\r\n")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := investigation.Validate(tt.doc)

			if len(got) != 1 {
				t.Fatalf("Validate() returned %d findings, want exactly 1: %+v", len(got), got)
			}

			if got[0].Code != investigation.CodeParse {
				t.Errorf("Code = %q, want %q", got[0].Code, investigation.CodeParse)
			}

			if got[0].Severity != validate.Error {
				t.Errorf("Severity = %v, want Error", got[0].Severity)
			}

			if got[0].Detail == "" {
				t.Error("Detail is empty: a rejected document must say why")
			}

			if strings.Contains(got[0].Detail, "investigation:") {
				t.Errorf("Detail = %q, want the package prefix trimmed", got[0].Detail)
			}
		})
	}
}

// TestValidate_Inferred pins that the rules read an unmarked document too. The
// findings are the same ones, because inference resolves the same regions.
func TestValidate_Inferred(t *testing.T) {
	t.Parallel()

	doc := unmark(withoutTrigger(t, grammar(t)))

	got := investigation.Validate(doc)

	if want := []string{investigation.CodeContextNoTrigger}; !equal(codes(got), want) {
		t.Fatalf("Validate() = %v, want %v", codes(got), want)
	}

	// For an inferred region the start is the line above the heading, which is
	// where the marker would go.
	if want := lineOf(t, doc, "## Context") - 1; got[0].Line != want {
		t.Errorf("Line = %d, want %d", got[0].Line, want)
	}
}

// TestValidate_CodesUseTheAliasPrefix pins the spelling DESIGN-0015 §4 chose:
// the package is named investigation, its codes say "inv.". A rename here
// would break every consumer's filter, so it is asserted rather than left to
// a reader's assumption that the two must match.
func TestValidate_CodesUseTheAliasPrefix(t *testing.T) {
	t.Parallel()

	for _, code := range []string{
		investigation.CodeParse,
		investigation.CodeContextNoTrigger,
		investigation.CodeConclusionNoAnswer,
		investigation.CodeConclusionVerdict,
	} {
		if !strings.HasPrefix(code, "inv.") {
			t.Errorf("code %q does not use the inv. prefix", code)
		}
	}
}

// answer makes an edit that rewrites the answer line.
func answer(text string) func(*testing.T, []byte) []byte {
	return func(t *testing.T, doc []byte) []byte {
		t.Helper()

		return withAnswer(t, doc, text)
	}
}

// replace makes a one-substitution edit, failing loudly if the fixture no
// longer contains the text.
func replace(old, with string) func(*testing.T, []byte) []byte {
	return func(t *testing.T, doc []byte) []byte {
		t.Helper()

		if !strings.Contains(string(doc), old) {
			t.Fatalf("fixture has no %q", old)
		}

		return []byte(strings.Replace(string(doc), old, with, 1))
	}
}

// withoutTrigger removes the "**Triggered by:**" line, leaving a context
// region that says why the investigation matters but not what prompted it.
func withoutTrigger(t *testing.T, doc []byte) []byte {
	t.Helper()

	return withoutLine(t, doc, "**Triggered by:**")
}

// withoutAnswer removes the answer line entirely, which is a document whose
// conclusion has no Answer field rather than an empty one.
func withoutAnswer(t *testing.T, doc []byte) []byte {
	t.Helper()

	return withoutLine(t, doc, "**Answer:**")
}

// withoutLine removes the one line carrying the given text, failing loudly if
// the fixture no longer holds exactly one.
func withoutLine(t *testing.T, doc []byte, text string) []byte {
	t.Helper()

	var out []string

	found := 0

	for _, line := range strings.Split(string(doc), "\n") {
		if strings.Contains(line, text) {
			found++

			continue
		}

		out = append(out, line)
	}

	if found != 1 {
		t.Fatalf("fixture has %d lines containing %q, want exactly one", found, text)
	}

	return []byte(strings.Join(out, "\n"))
}

// dropRegion removes a region entirely: its markers, its heading, and
// everything between them.
func dropRegion(kind string) func(*testing.T, []byte) []byte {
	return func(t *testing.T, doc []byte) []byte {
		t.Helper()

		var out []string

		inside := false
		found := false

		for _, line := range strings.Split(string(doc), "\n") {
			trimmed := strings.TrimSpace(line)

			switch {
			case trimmed == "<!--docz:"+kind+":start-->":
				inside = true
				found = true
			case trimmed == "<!--docz:"+kind+":end-->":
				inside = false
			case !inside:
				out = append(out, line)
			}
		}

		if !found {
			t.Fatalf("fixture has no %q region", kind)
		}

		return []byte(strings.Join(out, "\n"))
	}
}

// both applies two edits in order, for a case that needs a document wrong in
// two ways.
func both(first, second func(*testing.T, []byte) []byte) func(*testing.T, []byte) []byte {
	return func(t *testing.T, doc []byte) []byte {
		t.Helper()

		return second(t, first(t, doc))
	}
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
