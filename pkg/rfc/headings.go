package rfc

import "github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"

// The region kinds this package names. Constants because the parser and the
// heading table have to agree on every one of them: a typo in either would
// leave a field silently zero for every document.
const (
	kindSummary       = "summary"
	kindProblem       = "problem"
	kindProposal      = "proposal"
	kindAlternatives  = "alternatives"
	kindRisks         = "risks"
	kindCriteria      = "criteria"
	kindReferences    = "references"
	kindOpenQuestions = "open-questions"
	kindDecisions     = "decisions"
)

// headings is the heading table Parse falls back to for a document that
// carries no docz markers.
//
// It is the same data kinds.SpecFromTemplate derives from the embedded RFC
// template, written out here and pinned to it by a test. Written out because
// of rule R2 and the layer rules: deriving it at run time would mean this
// package importing internal/template, and a type package's production
// imports stop at the core. The test carries the coupling instead, so a
// template section that is renamed fails the build rather than silently
// making a field zero for every legacy document.
//
// Every rule matches by exact folded text: no RFC heading names a token the
// author is expected to replace, so none of them generalises to a prefix the
// way the IMPL template's phase heading does.
//
// The last two rules are kinds.sharedDefaults, in the order SpecFromTemplate
// appends them. The RFC template ships neither an Open Questions nor a
// Decisions section, but a document that grew one by hand still has it read —
// an RFC accumulates questions during review, and a reader that ignored them
// because the template predates them would report every such document as
// having none. The decisions rule has no field on Doc: the kind is read here
// only so a region for it is resolved and not mistaken for part of the
// section above it.
var headings = kinds.HeadingSpec{
	{Kind: kindSummary, Level: 2, Text: "summary"},
	{Kind: kindProblem, Level: 2, Text: "problem statement"},
	{Kind: kindProposal, Level: 2, Text: "proposed solution"},
	{Kind: kindAlternatives, Level: 2, Text: "alternatives considered"},
	{Kind: kindRisks, Level: 2, Text: "risks and mitigations"},
	{Kind: kindCriteria, Level: 2, Text: "success criteria"},
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
