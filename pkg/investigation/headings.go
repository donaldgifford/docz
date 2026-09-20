package investigation

import "github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"

// The region kinds this package names. Constants because the parser and the
// heading table have to agree on every one of them: a typo in either would
// leave a field silently zero for every document.
const (
	kindQuestion       = "question"
	kindHypothesis     = "hypothesis"
	kindContext        = "context"
	kindApproach       = "approach"
	kindEnvironment    = "environment"
	kindFindings       = "findings"
	kindConclusion     = "conclusion"
	kindRecommendation = "recommendation"
	kindReferences     = "references"
	kindOpenQuestions  = "open-questions"
	kindDecisions      = "decisions"
)

// headings is the heading table Parse falls back to for a document that
// carries no docz markers.
//
// It is the same data kinds.SpecFromTemplate derives from the embedded
// investigation template, written out here and pinned to it by a test.
// Written out because of rule R2 and the layer rules: deriving it at run time
// would mean this package importing internal/template, and a type package's
// production imports stop at the core. The test carries the coupling instead,
// so a template section that is renamed fails the build rather than silently
// making a field zero for every legacy document.
//
// The last two rules are kinds.sharedDefaults, which the template does not
// mark: an investigation that grew an Open Questions or Decisions section by
// hand still has it read. References is in the template, and its rule is the
// same as the shared default's.
var headings = kinds.HeadingSpec{
	{Kind: kindQuestion, Level: 2, Text: "question"},
	{Kind: kindHypothesis, Level: 2, Text: "hypothesis"},
	{Kind: kindContext, Level: 2, Text: "context"},
	{Kind: kindApproach, Level: 2, Text: "approach"},
	{Kind: kindEnvironment, Level: 2, Text: "environment"},
	{Kind: kindFindings, Level: 2, Text: "findings"},
	{Kind: kindConclusion, Level: 2, Text: "conclusion"},
	{Kind: kindRecommendation, Level: 2, Text: "recommendation"},
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
