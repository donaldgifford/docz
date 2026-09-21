package impl_test

import (
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
	"github.com/donaldgifford/docz/v2/pkg/impl"
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
// into noise. The grammar fixture is a correct IMPL document, so the only
// finding it may produce is the one its own verify line earns.
func TestValidate_CleanDocument(t *testing.T) {
	t.Parallel()

	got := impl.Validate(grammar(t))

	want := []string{impl.CodeVerifyNoCommand}
	if !equal(codes(got), want) {
		t.Errorf("Validate() = %v, want %v", codes(got), want)
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
			name: "a phase with no title",
			edit: replace("### Phase 2B: Cleanup", "### Phase 2B:"),
			want: []string{impl.CodeNoTitle, impl.CodeVerifyNoCommand},
		},
		{
			name: "a phase with no tasks",
			edit: replace("- [x] ~~Delete the compatibility shim~~ — skipped: the shim shipped\n"+
				"- [ ] Update the docs\n      **verify:** run the linter by hand\n", ""),
			want: []string{impl.CodeNoTasks},
		},
		{
			name: "a task with no text",
			edit: replace("- [x] Write the parser", "- [ ]"),
			want: []string{impl.CodeTaskEmpty, impl.CodeVerifyNoCommand},
		},
		{
			name: "a skipped task with no reason",
			edit: replace("— skipped: the shim shipped", "— skipped:"),
			want: []string{impl.CodeSkippedNoNote, impl.CodeVerifyNoCommand},
		},
		{
			name: "a phase region whose heading is not a phase heading",
			edit: replace("### Phase 2B: Cleanup", "### Cleanup"),
			want: []string{impl.CodeNoHeading},
		},
		{
			// The verify line that is already in the fixture, plus a second.
			name: "a verify line with no command",
			edit: replace("verify: `go test ./pkg/impl/...`", "verify: run the tests"),
			want: []string{impl.CodeVerifyNoCommand, impl.CodeVerifyNoCommand},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := impl.Validate([]byte(tt.edit(string(grammar(t)))))

			if !equal(codes(got), tt.want) {
				t.Errorf("Validate() = %v, want %v\nfindings: %+v", codes(got), tt.want, got)
			}
		})
	}
}

// TestValidate_FindingLines pins that a finding points at the line a person
// has to edit. A code with the wrong line is worse than no code: it sends the
// reader somewhere correct.
func TestValidate_FindingLines(t *testing.T) {
	t.Parallel()

	doc := grammar(t)

	got := impl.Validate(doc)
	if len(got) != 1 {
		t.Fatalf("Validate() = %+v, want one finding", got)
	}

	// The verify-no-command finding points at the checkbox, not the verify
	// line: the task is the unit a reader fixes.
	if want := lineOf(t, doc, "- [ ] Update the docs"); got[0].Line != want {
		t.Errorf("Line = %d, want the task at %d", got[0].Line, want)
	}

	noTitle := impl.Validate([]byte(strings.ReplaceAll(string(doc),
		"### Phase 1: Foundations", "### Phase 1:")))

	if len(noTitle) == 0 || noTitle[0].Code != impl.CodeNoTitle {
		t.Fatalf("Validate() = %+v, want the no-title finding first", noTitle)
	}

	if want := lineOf(t, doc, "### Phase 1: Foundations"); noTitle[0].Line != want {
		t.Errorf("Line = %d, want the phase heading at %d", noTitle[0].Line, want)
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
		code string
	}{
		{
			name: "no frontmatter",
			doc:  []byte("### Phase 1: One\n\n#### Tasks\n\n- [ ] a\n"),
			code: impl.CodeParse,
		},
		{
			name: "CR line endings",
			doc:  []byte(strings.ReplaceAll(string(doc), "\n", "\r\n")),
			code: impl.CodeParse,
		},
		{
			name: "no phases",
			doc:  dropPhases(doc),
			code: impl.CodeParse,
		},
		{
			name: "duplicate phase token",
			doc: []byte(strings.ReplaceAll(string(doc),
				"### Phase 2B: Cleanup", "### Phase 1: Cleanup")),
			code: impl.CodeDuplicateToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := impl.Validate(tt.doc)

			if len(got) != 1 {
				t.Fatalf("Validate() returned %d findings, want exactly 1: %+v", len(got), got)
			}

			if got[0].Code != tt.code {
				t.Errorf("Code = %q, want %q", got[0].Code, tt.code)
			}

			if got[0].Severity != validate.Error {
				t.Errorf("Severity = %v, want Error", got[0].Severity)
			}

			if got[0].Detail == "" {
				t.Error("Detail is empty: a rejected document must say why")
			}
		})
	}
}

// TestValidate_DuplicateTokenPointsAtTheSecondPhase pins which of the two
// colliding phases the finding names. The first is where the token belongs;
// the second is the one to rename.
func TestValidate_DuplicateTokenPointsAtTheSecondPhase(t *testing.T) {
	t.Parallel()

	doc := grammar(t)
	edited := strings.ReplaceAll(string(doc), "### Phase 2B: Cleanup", "### Phase 1: Cleanup")

	got := impl.Validate([]byte(edited))

	want := lineOf(t, []byte(edited), "### Phase 1: Cleanup")
	if got[0].Line != want {
		t.Errorf("Line = %d, want the second phase at %d", got[0].Line, want)
	}
}

// TestValidate_Template pins what the freshly created document reports: three
// phases whose titles are still the template's placeholder comments, and
// nothing else.
//
// Not zero findings, unlike the generic tier's template guard: DESIGN-0014 §3
// names the placeholder as exactly the case impl.phase.no-title exists for. So
// the assertion is that the template trips that rule three times and no other
// rule at all — anything more would be a rule firing on correct markdown.
func TestValidate_Template(t *testing.T) {
	t.Parallel()

	got := impl.Validate(renderedTemplate(t))

	for _, f := range got {
		if f.Code != impl.CodeNoTitle {
			t.Errorf("template reports %s at line %d: %s", f.Code, f.Line, f.Detail)
		}
	}

	if len(got) != 3 {
		t.Errorf("template reports %d findings, want 3 no-title warnings: %+v", len(got), got)
	}
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
