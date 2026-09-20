// Package investigation interprets an INV document as a typed value.
//
// EXPERIMENTAL until v2.0.0: the surface may change between betas
// (ADR-0002 Decision 7). The five packages frozen at v1.0.0 are not
// affected; this one is not among them.
//
// Parse returns a Doc with one field per section of the investigation
// template, and Validate reports the rules only a typed model can check.
// Neither touches the filesystem, and neither reads the document's type name
// (ADR-0002 R7): spans are located by region kind, so a repo's custom type
// whose documents carry a question, findings, and a conclusion parses with
// this package.
//
// The grammar is the shared one (DESIGN-0014 §2.9): a string field is its
// region's body, a list field is the region's top-level items, the
// environment table is mapped by column position, and the "**Triggered by:**"
// and "**Answer:**" fields are read with kinds.Field. The one interpretation
// this package adds is Verdict, which is the answer's first word read as a
// value a consumer can switch on.
//
// A document that carries no docz markers parses too. Its regions are
// inferred from its headings and Doc.Inferred is set, which is the
// backwards-compatibility path for every document created before markers
// existed. Markers, once present, are authoritative.
package investigation

import (
	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// The words this package compares against and renders, lower-cased.
//
// One definition each because three places have to agree on them: Verdict's
// String method renders them, Parse folds an answer's first word to match
// them, and "inconclusive" is also a status a finished-but-undecided
// investigation carries, which the conclusion rule folds a status against.
const (
	wordYes          = "yes"
	wordNo           = "no"
	wordInconclusive = "inconclusive"
	wordConcluded    = "concluded"
	wordUnknown      = "unknown"
)

// Doc is a parsed INV document: one field per section of the template.
//
// Every string is copied out of the input, so a caller may reuse or modify
// the bytes it passed to Parse.
type Doc struct {
	// Frontmatter, as document.ParseFrontmatter read it.
	ID      string
	Title   string
	Status  config.Status
	Author  string
	Created string

	// Inferred is true when the document carried no docz markers and its
	// spans came from its headings instead. A consumer surfaces it; the
	// library never logs. validate.Document reports the matching
	// region.inferred warning, so a run that does both says it once.
	Inferred bool

	// Question is the question region's body: what the investigation set out
	// to answer.
	Question string

	// Hypothesis is the hypothesis region's body.
	Hypothesis string

	// Context is the context region's body, the "**Triggered by:**" line
	// included. The field is the same bytes read a second way, not a line cut
	// out of the prose.
	Context string

	// TriggeredBy is the text after "**Triggered by:**", "" when the field is
	// absent or still holds the template's placeholder comment.
	TriggeredBy string

	// Approach are the steps of the approach region's list, numbered or
	// bulleted alike: the fleet writes it ordered, and a document that
	// bullets it is describing the same thing.
	Approach []kinds.Item

	// Environment are the rows of the environment table, mapped by column
	// position. Nil for a document with no table, or one whose only row is
	// the template's empty placeholder.
	Environment []Component

	// Findings are the level-3 headings inside the findings region with the
	// evidence under each. The titles are the author's — "Observation 1" from
	// the template, or whatever the finding actually was — so nothing here
	// depends on how they are worded.
	Findings []kinds.Section

	// Conclusion is the conclusion region's body, the "**Answer:**" line
	// included.
	Conclusion string

	// Answer is the text after "**Answer:**", "" when the field is absent or
	// unfilled. Either bold spelling the corpus writes is read (kinds.Field).
	Answer string

	// Verdict is Answer's first word as a value: VerdictUnknown when the
	// answer is empty, or when its first word is none of yes, no, or
	// inconclusive. Answer stays the text, because the sentence after the
	// verdict is usually the useful half of it.
	Verdict Verdict

	// Recommendation is the recommendation region's body: what should happen
	// next.
	Recommendation string

	// OpenQuestions is nil when the document has none. The section is
	// optional for every type but design.
	OpenQuestions []kinds.Question

	Decisions  []kinds.Decision
	References []kinds.Reference
}

// Component is one row of the environment table: a thing and the version or
// value of it the investigation ran against.
type Component struct {
	// Component is the first column, inline markdown kept verbatim.
	Component string

	// Value is the second column: the template heads it "Version / Value",
	// and the corpus writes both.
	Value string

	// Line is the 1-based line of the row in the document.
	Line int
}

// Verdict is an answer's first word: the part of a conclusion a consumer can
// branch on without reading prose.
//
// It is derived rather than declared, because the document does not carry it
// separately — an author writes one Answer line, and this is that line's first
// word. Anything outside the three is VerdictUnknown rather than an error: an
// answer that does not open with a verdict is still an answer, and saying so
// is validate's job (inv.conclusion.verdict).
type Verdict int

// The verdicts. VerdictUnknown is the zero value, so a Doc whose conclusion
// region is absent reports it without Parse having to decide anything.
const (
	VerdictUnknown Verdict = iota
	VerdictYes
	VerdictNo
	VerdictInconclusive
)

// String renders a verdict in the spelling the answer line uses, lower-cased.
// Anything outside the three renders as "unknown".
func (v Verdict) String() string {
	switch v {
	case VerdictYes:
		return wordYes
	case VerdictNo:
		return wordNo
	case VerdictInconclusive:
		return wordInconclusive
	default:
		return wordUnknown
	}
}
