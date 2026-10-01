package runbook

import "github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"

// The region kinds this package names. Constants because the parser and the
// heading table have to agree on every one of them: a typo in either would
// leave a field silently zero for every document.
const (
	kindLastVerified  = "last-verified"
	kindOverview      = "overview"
	kindWhen          = "when"
	kindPrerequisites = "prerequisites"
	kindProcedure     = "procedure"
	kindSteps         = "steps"
	kindVerification  = "verification"
	kindRollback      = "rollback"
	kindScenario      = "scenario"
	kindEscalation    = "escalation"
	kindReferences    = "references"
)

// headings is the heading table Parse falls back to for a document that
// carries no docz markers.
//
// It is the same data kinds.SpecFromTemplate derives from the embedded
// runbook template, written out here and pinned to it by a test, for the
// reason impl's is: deriving it at run time would mean importing doctemplate,
// and a type package's production imports stop at the core (R2).
//
// The steps kind has two rules because it has two parents. A procedure's
// prefix is the bare word, so "### Procedure 3: Rotate" and the template's
// "### Procedure 1: <!-- … -->" both match; a scenario's keeps its colon,
// because "Scenario:" is the whole of the template's heading.
var headings = kinds.HeadingSpec{
	{Kind: kindLastVerified, Level: 2, Text: "last verified"},
	{Kind: kindOverview, Level: 2, Text: "overview"},
	{Kind: kindWhen, Level: 2, Text: "when to use"},
	{Kind: kindPrerequisites, Level: 2, Text: "prerequisites"},
	{Kind: kindProcedure, Level: 3, Prefix: "procedure"},
	{Kind: kindSteps, Level: 4, Text: "steps", Parent: kindProcedure},
	{Kind: kindVerification, Level: 4, Text: "verification", Parent: kindProcedure},
	{Kind: kindRollback, Level: 4, Text: "rollback", Parent: kindProcedure},
	{Kind: kindScenario, Level: 3, Prefix: "scenario:"},
	{Kind: kindSteps, Level: 4, Text: "steps", Parent: kindScenario},
	{Kind: kindEscalation, Level: 2, Text: "escalation"},
	{Kind: kindReferences, Level: 2, Text: "references"},
	{Kind: "open-questions", Level: 2, Text: "open questions"},
	{Kind: "decisions", Level: 2, Text: "decisions"},
}

// Headings returns the heading table this package infers regions from, for a
// consumer that wants to run validate.Document with the same fallback Parse
// uses. Returned as a copy, so a caller cannot reshape what Parse does.
func Headings() kinds.HeadingSpec {
	out := make(kinds.HeadingSpec, len(headings))
	copy(out, headings)

	return out
}
