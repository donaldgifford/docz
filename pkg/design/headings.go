package design

import "github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"

// The region kinds this package names. Constants because the parser, the
// heading table, and the findings have to agree on every one of them: a typo
// in any of the three would leave a field silently zero for every document.
const (
	kindOverview       = "overview"
	kindGoals          = "goals"
	kindNonGoals       = "non-goals"
	kindBackground     = "background"
	kindDetailedDesign = "detailed-design"
	kindAPIChanges     = "api-changes"
	kindDataModel      = "data-model"
	kindTesting        = "testing"
	kindRollout        = "rollout"
	kindOpenQuestions  = "open-questions"
	kindDecisions      = "decisions"
	kindReferences     = "references"
)

// headings is the heading table Parse falls back to for a document that
// carries no docz markers.
//
// It is the same data kinds.SpecFromTemplate derives from the embedded DESIGN
// template, written out here and pinned to it by a test. Written out because
// of rule R2 and the layer rules: deriving it at run time would mean this
// package importing internal/template, and a type package's production
// imports stop at the core. The test carries the coupling instead, so a
// template section that is renamed fails the build rather than silently
// making a field zero for every legacy document.
//
// Two entries are worth reading twice. Goals and Non-Goals are level-3
// headings under a "Goals and Non-Goals" heading the template does not mark,
// so neither has a parent — a rule with one only matches inside that parent's
// span, and a parent that never matches drops its children. And decisions is
// not in the template at all: it is one of the shared kinds any document may
// grow by hand, which kinds.SpecFromTemplate appends and this table spells out
// in the same place, last.
var headings = kinds.HeadingSpec{
	{Kind: kindOverview, Level: 2, Text: "overview"},
	{Kind: kindGoals, Level: 3, Text: "goals"},
	{Kind: kindNonGoals, Level: 3, Text: "non-goals"},
	{Kind: kindBackground, Level: 2, Text: "background"},
	{Kind: kindDetailedDesign, Level: 2, Text: "detailed design"},
	{Kind: kindAPIChanges, Level: 2, Text: "api / interface changes"},
	{Kind: kindDataModel, Level: 2, Text: "data model"},
	{Kind: kindTesting, Level: 2, Text: "testing strategy"},
	{Kind: kindRollout, Level: 2, Text: "migration / rollout plan"},
	{Kind: kindOpenQuestions, Level: 2, Text: "open questions"},
	{Kind: kindReferences, Level: 2, Text: "references"},
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
