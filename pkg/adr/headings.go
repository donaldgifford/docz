package adr

import "github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"

// The region kinds this package names. Constants because the parser and the
// heading table have to agree on every one of them: a typo in either would
// leave a field silently zero for every document.
const (
	kindSummary       = "summary"
	kindContext       = "context"
	kindDecision      = "decision"
	kindConsequences  = "consequences"
	kindPositive      = "positive"
	kindNegative      = "negative"
	kindNeutral       = "neutral"
	kindAlternatives  = "alternatives"
	kindReferences    = "references"
	kindOpenQuestions = "open-questions"
	kindDecisions     = "decisions"
)

// headings is the heading table Parse falls back to for a document that
// carries no docz markers.
//
// It is the same data kinds.SpecFromTemplate derives from the embedded ADR
// template, written out here and pinned to it by a test. Written out because
// of rule R2 and the layer rules: deriving it at run time would mean this
// package importing internal/template, and a type package's production
// imports stop at the core. The test carries the coupling instead, so a
// template section that is renamed fails the build rather than silently
// making a field zero for every legacy document.
//
// The three consequence kinds are the one place an ADR nests: each holds only
// inside a consequences region, so a "### Positive" heading anywhere else is
// a heading and not a consequence list.
//
// The last two are the shared defaults SpecFromTemplate appends for a kind
// the template does not mark. The ADR template ships References and neither
// Open Questions nor Decisions, and a document that grew one of those by hand
// still has it read.
var headings = kinds.HeadingSpec{
	{Kind: kindSummary, Level: 2, Text: "summary"},
	{Kind: kindContext, Level: 2, Text: "context"},
	{Kind: kindDecision, Level: 2, Text: "decision"},
	{Kind: kindConsequences, Level: 2, Text: "consequences"},
	{Kind: kindPositive, Level: 3, Text: "positive", Parent: kindConsequences},
	{Kind: kindNegative, Level: 3, Text: "negative", Parent: kindConsequences},
	{Kind: kindNeutral, Level: 3, Text: "neutral", Parent: kindConsequences},
	{Kind: kindAlternatives, Level: 2, Text: "alternatives considered"},
	{Kind: kindReferences, Level: 2, Text: "references"},
	{Kind: kindOpenQuestions, Level: 2, Text: "open questions"},
	{Kind: kindDecisions, Level: 2, Text: "decisions"},
}

// Headings returns the heading table this package infers regions from, for a
// consumer that wants to run validate.Document with the same fallback Parse
// uses. Returned as a copy, so a caller cannot reshape what Parse does.
func Headings() kinds.HeadingSpec {
	out := make(kinds.HeadingSpec, len(headings))
	copy(out, headings)

	return out
}
