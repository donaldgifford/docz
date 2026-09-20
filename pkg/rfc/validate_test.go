package rfc_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
	"github.com/donaldgifford/docz/v2/pkg/rfc"
)

// alternativeBullets is the fixture's whole alternatives list, so a case can
// empty the section without removing its heading.
const alternativeBullets = `- **A. Leave the parsing to each consumer.** Rejected: three readers already
  disagree about the risks table, which is the problem rather than the fix.
- **Ship a generic markdown model.** Rejected: a consumer would still have to
  know which heading held the risks.
- Wait for the schema work to land first.`

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
// into noise. The grammar fixture is a correct RFC, so the only finding it may
// produce is the one its own unmitigated risk earns.
func TestValidate_CleanDocument(t *testing.T) {
	t.Parallel()

	got := impliedCodes(t, grammar(t))

	want := []string{rfc.CodeRisksNoMitigation}
	if !slices.Equal(got, want) {
		t.Errorf("Validate() = %v, want %v", got, want)
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
			// The section is still there, which is the point: an absent one is
			// region.missing in the generic tier, not a finding here.
			name: "an alternatives section that holds none",
			edit: replace(alternativeBullets, "<!-- nothing considered yet -->"),
			want: []string{rfc.CodeAlternativesEmpty, rfc.CodeRisksNoMitigation},
		},
		{
			// The row already in the fixture, plus a second.
			name: "a second risk with no mitigation",
			edit: replace("A test pins the heading table to it", ""),
			want: []string{rfc.CodeRisksNoMitigation, rfc.CodeRisksNoMitigation},
		},
		{
			name: "accepted with an unresolved question",
			edit: replace("status: Draft", "status: Accepted"),
			want: []string{rfc.CodeRisksNoMitigation, rfc.CodeStatusOpenQuestion},
		},
		{
			// Folded, so a repo that spells its status in lower case is held to
			// the same rule.
			name: "accepted in lower case is still accepted",
			edit: replace("status: Draft", "status: accepted"),
			want: []string{rfc.CodeRisksNoMitigation, rfc.CodeStatusOpenQuestion},
		},
		{
			name: "accepted with every question resolved",
			edit: chain(
				replace("status: Draft", "status: Accepted"),
				replace("### 2. Should an empty mitigation be an error rather than a warning?",
					"### 2. Should an empty mitigation be an error rather than a warning?\n\n"+
						"> **Resolved 2026-09-20: (a)** drafting a risk before its mitigation is normal."),
			),
			want: []string{rfc.CodeRisksNoMitigation},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := impliedCodes(t, []byte(tt.edit(string(grammar(t)))))

			if !slices.Equal(got, tt.want) {
				t.Errorf("Validate() = %v, want %v", got, tt.want)
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

	risks := rfc.Validate(doc)
	if len(risks) != 1 {
		t.Fatalf("Validate() = %+v, want one finding", risks)
	}

	if want := lineOf(t, doc, "| The risks table grows a fifth column |"); risks[0].Line != want {
		t.Errorf("Line = %d, want the row at %d", risks[0].Line, want)
	}

	// The heading rather than the region's marker: the heading is where a
	// reader would look for the section that says nothing.
	empty := []byte(replace(alternativeBullets, "<!-- nothing considered yet -->")(string(doc)))

	emptyFindings := rfc.Validate(empty)
	if len(emptyFindings) == 0 || emptyFindings[0].Code != rfc.CodeAlternativesEmpty {
		t.Fatalf("Validate() = %+v, want the empty-alternatives finding first", emptyFindings)
	}

	if want := lineOf(t, empty, "## Alternatives Considered"); emptyFindings[0].Line != want {
		t.Errorf("Line = %d, want the heading at %d", emptyFindings[0].Line, want)
	}

	// The unresolved question, not the frontmatter: the status is the claim,
	// but the question is the thing left to settle.
	accepted := []byte(replace("status: Draft", "status: Accepted")(string(doc)))

	acceptedFindings := rfc.Validate(accepted)
	if len(acceptedFindings) != 2 {
		t.Fatalf("Validate() = %+v, want two findings", acceptedFindings)
	}

	if want := lineOf(t, accepted, "### 2. Should an empty mitigation"); acceptedFindings[1].Line != want {
		t.Errorf("Line = %d, want the open question at %d", acceptedFindings[1].Line, want)
	}
}

// TestValidate_AlternativesEmptyIsAWarning pins the severities, which are the
// half of a finding a consumer gates on.
func TestValidate_AlternativesEmptyIsAWarning(t *testing.T) {
	t.Parallel()

	doc := string(grammar(t))

	tests := []struct {
		name string
		doc  []byte
		code string
		want validate.Severity
	}{
		{
			name: "empty alternatives",
			doc:  []byte(replace(alternativeBullets, "<!-- nothing considered yet -->")(doc)),
			code: rfc.CodeAlternativesEmpty,
			want: validate.Warning,
		},
		{
			name: "no mitigation",
			doc:  []byte(doc),
			code: rfc.CodeRisksNoMitigation,
			want: validate.Warning,
		},
		{
			name: "accepted with an open question",
			doc:  []byte(replace("status: Draft", "status: Accepted")(doc)),
			code: rfc.CodeStatusOpenQuestion,
			want: validate.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for _, f := range rfc.Validate(tt.doc) {
				if f.Code != tt.code {
					continue
				}

				if f.Severity != tt.want {
					t.Errorf("%s severity = %v, want %v", f.Code, f.Severity, tt.want)
				}

				if f.Detail == "" {
					t.Errorf("%s has no detail", f.Code)
				}

				return
			}

			t.Errorf("Validate() reported no %s: %+v", tt.code, rfc.Validate(tt.doc))
		})
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
			doc:  []byte("## Summary\n\nA proposal with no frontmatter.\n"),
		},
		{
			name: "CR line endings",
			doc:  []byte(strings.ReplaceAll(string(doc), "\n", "\r\n")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := rfc.Validate(tt.doc)

			if len(got) != 1 {
				t.Fatalf("Validate() returned %d findings, want exactly 1: %+v", len(got), got)
			}

			if got[0].Code != rfc.CodeParse {
				t.Errorf("Code = %q, want %q", got[0].Code, rfc.CodeParse)
			}

			if got[0].Severity != validate.Error {
				t.Errorf("Severity = %v, want Error", got[0].Severity)
			}

			// The package prefix belongs to the error, not to the finding: a
			// consumer already knows which code it is reading.
			if got[0].Detail == "" || strings.HasPrefix(got[0].Detail, "rfc: ") {
				t.Errorf("Detail = %q, want the reason without the package prefix", got[0].Detail)
			}
		})
	}
}

// TestValidate_InferredDocumentIsValidated pins that the rules read a bare
// document too: inference is the compatibility path, not a reduced one.
func TestValidate_InferredDocumentIsValidated(t *testing.T) {
	t.Parallel()

	doc := unmark(grammar(t))

	got := impliedCodes(t, doc)

	want := []string{rfc.CodeRisksNoMitigation}
	if !slices.Equal(got, want) {
		t.Errorf("Validate() = %v, want %v", got, want)
	}
}

// impliedCodes is Validate's codes for a document, named so a case reads as an
// assertion about what the document earns.
func impliedCodes(t *testing.T, doc []byte) []string {
	t.Helper()

	return codes(rfc.Validate(doc))
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

// chain applies edits left to right, for a case that needs two.
func chain(edits ...func(string) string) func(string) string {
	return func(doc string) string {
		for _, edit := range edits {
			doc = edit(doc)
		}

		return doc
	}
}
