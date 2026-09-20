package adr_test

import (
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/adr"
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

// TestValidate_CleanDocument is the guard that keeps the three rules from
// turning into noise. The grammar fixture is a correct ADR, so it earns
// nothing at all.
func TestValidate_CleanDocument(t *testing.T) {
	t.Parallel()

	if got := adr.Validate(grammar(t)); len(got) != 0 {
		t.Errorf("Validate() = %+v, want no findings", got)
	}
}

// TestValidate_Rules is one case per code: a document that trips it, and the
// codes it produces. The cases that trip nothing are as important as the ones
// that do — each rule reads the status, and a rule that fired on a proposed
// document would fire on every ADR in review.
func TestValidate_Rules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		edit func(string) string
		want []string
	}{
		{
			name: "accepted with an empty decision",
			edit: chain(accept, emptyDecision),
			want: []string{adr.CodeDecisionEmpty},
		},
		{
			name: "accepted with a decision",
			edit: accept,
			want: nil,
		},
		{
			name: "proposed with an empty decision",
			edit: emptyDecision,
			want: nil,
		},
		{
			name: "no consequences at all",
			edit: emptyConsequences,
			want: []string{adr.CodeConsequencesEmpty},
		},
		{
			// One finding, not three: the three lists are one section as a
			// reader sees it.
			name: "only one of the three consequence lists",
			edit: chain(
				replace("- Every document in the fleet eventually wants markers, which is a migration\n"+
					"  nobody has scheduled\n", ""),
				replace("- The heading table is written out in the type package rather than derived\n", ""),
			),
			want: nil,
		},
		{
			name: "superseded with no reference to another ADR",
			edit: chain(supersede, dropForwardPointer),
			want: []string{adr.CodeSupersededNoReference},
		},
		{
			name: "superseded with a reference to another ADR",
			edit: supersede,
			want: nil,
		},
		{
			// Its own id is not a forward pointer, which is the whole point of
			// the rule.
			name: "superseded citing only itself",
			edit: chain(supersede, replace("[ADR-0001]", "[ADR-0002]")),
			want: []string{adr.CodeSupersededNoReference},
		},
		{
			// The corpus records a supersession in prose at least as often as
			// in the reference list.
			name: "superseded with the forward pointer in the summary",
			edit: chain(supersede, dropForwardPointer,
				replace("Locate an ADR's sections", "Superseded by ADR-0004. Locate an ADR's sections")),
			want: nil,
		},
		{
			name: "superseded with the forward pointer in the context",
			edit: chain(supersede, dropForwardPointer,
				replace("Every field of the typed model", "ADR-0004 replaced this. Every field")),
			want: nil,
		},
		{
			// Folded: a path spells the id lower-cased and a forward pointer in
			// a path is still a forward pointer.
			name: "superseded with a lower-cased id",
			edit: chain(supersede, dropForwardPointer,
				replace("- [DESIGN-0014]", "- see adr-0004 [DESIGN-0014]")),
			want: nil,
		},
		{
			name: "every rule at once",
			edit: chain(supersede, dropForwardPointer, emptyConsequences),
			want: []string{adr.CodeConsequencesEmpty, adr.CodeSupersededNoReference},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := adr.Validate([]byte(tt.edit(string(grammar(t)))))

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

	tests := []struct {
		name   string
		edit   func(string) string
		code   string
		anchor string
	}{
		{
			name:   "decision points at its heading",
			edit:   chain(accept, emptyDecision),
			code:   adr.CodeDecisionEmpty,
			anchor: "## Decision",
		},
		{
			name:   "consequences points at its heading",
			edit:   emptyConsequences,
			code:   adr.CodeConsequencesEmpty,
			anchor: "## Consequences",
		},
		{
			name:   "superseded points at the references heading",
			edit:   chain(supersede, dropForwardPointer),
			code:   adr.CodeSupersededNoReference,
			anchor: "## References",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			edited := []byte(tt.edit(string(doc)))

			got := adr.Validate(edited)
			if len(got) != 1 || got[0].Code != tt.code {
				t.Fatalf("Validate() = %+v, want one %s", got, tt.code)
			}

			if want := lineOf(t, edited, tt.anchor); got[0].Line != want {
				t.Errorf("Line = %d, want the %q heading at %d", got[0].Line, tt.anchor, want)
			}
		})
	}
}

// TestValidate_MissingRegionHasNoLine pins the fallback: a rule about a
// section the document does not carry reports line 0, the whole document,
// rather than pointing at a line that means something else.
func TestValidate_MissingRegionHasNoLine(t *testing.T) {
	t.Parallel()

	edited := chain(supersede, dropForwardPointer, func(doc string) string {
		return dropRegion(doc, "references")
	})(string(grammar(t)))

	got := adr.Validate([]byte(edited))
	if len(got) != 1 || got[0].Code != adr.CodeSupersededNoReference {
		t.Fatalf("Validate() = %+v, want one %s", got, adr.CodeSupersededNoReference)
	}

	if got[0].Line != 0 {
		t.Errorf("Line = %d, want 0: the document has no references section", got[0].Line)
	}
}

// TestValidate_Severities pins which of the three gates a CI run. Only an
// accepted ADR with no decision is an error: the other two describe a document
// that is readable but has not said everything it should.
func TestValidate_Severities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		edit func(string) string
		code string
		want validate.Severity
	}{
		{chain(accept, emptyDecision), adr.CodeDecisionEmpty, validate.Error},
		{emptyConsequences, adr.CodeConsequencesEmpty, validate.Warning},
		{chain(supersede, dropForwardPointer), adr.CodeSupersededNoReference, validate.Warning},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			t.Parallel()

			got := adr.Validate([]byte(tt.edit(string(grammar(t)))))
			if len(got) != 1 {
				t.Fatalf("Validate() = %+v, want one finding", got)
			}

			if got[0].Severity != tt.want {
				t.Errorf("Severity = %v, want %v", got[0].Severity, tt.want)
			}

			if got[0].Kind == "" {
				t.Error("Kind is empty: every rule here concerns one region kind")
			}

			if got[0].Detail == "" {
				t.Error("Detail is empty")
			}
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
			doc:  []byte("# ADR-0001\n\n## Decision\n\nDo the thing.\n"),
		},
		{
			name: "CR line endings",
			doc:  []byte(strings.ReplaceAll(string(doc), "\n", "\r\n")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := adr.Validate(tt.doc)

			if len(got) != 1 {
				t.Fatalf("Validate() returned %d findings, want exactly 1: %+v", len(got), got)
			}

			if got[0].Code != adr.CodeParse {
				t.Errorf("Code = %q, want %q", got[0].Code, adr.CodeParse)
			}

			if got[0].Severity != validate.Error {
				t.Errorf("Severity = %v, want Error", got[0].Severity)
			}

			if got[0].Detail == "" {
				t.Error("Detail is empty: a rejected document must say why")
			}

			if strings.Contains(got[0].Detail, "adr: ") {
				t.Errorf("Detail = %q, want the package prefix trimmed", got[0].Detail)
			}
		})
	}
}

// The edits the cases above compose. Each is a single substitution that panics
// loudly if the fixture no longer contains what it names, so a fixture change
// fails here rather than quietly making a case assert nothing.

// accept flips the fixture to the status that makes an empty decision an error.
func accept(doc string) string {
	return replace("status: Proposed", "status: Accepted")(doc)
}

// supersede flips it to the status the forward-pointer rule reads.
func supersede(doc string) string {
	return replace("status: Proposed", "status: Superseded")(doc)
}

// emptyDecision leaves the decision region and its heading with nothing under
// them, which is how a section nobody has written yet reads.
func emptyDecision(doc string) string {
	return emptyRegion(doc, "decision")
}

// emptyConsequences empties all three lists, which is the one case the
// consequences rule fires for.
func emptyConsequences(doc string) string {
	for _, kind := range []string{"positive", "negative", "neutral"} {
		doc = emptyRegion(doc, kind)
	}

	return doc
}

// dropForwardPointer removes the only mention of another ADR in the fixture.
func dropForwardPointer(doc string) string {
	return replace("- [ADR-0001](0001-pkgdoczcore-as-the-single-public-core.md) — the layer rules\n"+
		"  this decision works inside\n", "")(doc)
}

// chain applies edits left to right.
func chain(edits ...func(string) string) func(string) string {
	return func(doc string) string {
		for _, edit := range edits {
			doc = edit(doc)
		}

		return doc
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
