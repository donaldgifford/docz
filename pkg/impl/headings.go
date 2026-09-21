package impl

import "github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"

// headings is the heading table Parse falls back to for a document that
// carries no docz markers.
//
// It is the same data kinds.SpecFromTemplate derives from the embedded IMPL
// template, written out here and pinned to it by a test. Written out because
// of rule R2 and the layer rules: deriving it at run time would mean this
// package importing internal/template, and a type package's production
// imports stop at the core. The test carries the coupling instead, so a
// template section that is renamed fails the build rather than silently
// making a field zero for every legacy document.
//
// The phase rule matches by prefix, so the template's placeholder heading
// "### Phase 1: <!-- Foundation -->" and a real document's "### Phase 3: CI
// Readiness" are both phases.
// The region kinds this package names. Constants because the parser and the
// heading table have to agree on every one of them: a typo in either would
// leave a field silently zero for every document.
const (
	kindObjective    = "objective"
	kindScope        = "scope"
	kindPhase        = "phase"
	kindTasks        = "tasks"
	kindCriteria     = "criteria"
	kindDependencies = "dependencies"
	kindReferences   = "references"
	kindDecisions    = "decisions"
)

var headings = kinds.HeadingSpec{
	{Kind: kindObjective, Level: 2, Text: "objective"},
	{Kind: kindScope, Level: 2, Text: "scope"},
	{Kind: "in-scope", Level: 3, Text: "in scope", Parent: kindScope},
	{Kind: "out-of-scope", Level: 3, Text: "out of scope", Parent: kindScope},
	{Kind: kindPhase, Level: 3, Prefix: "phase"},
	{Kind: kindTasks, Level: 4, Text: "tasks", Parent: kindPhase},
	{Kind: kindCriteria, Level: 4, Text: "success criteria", Parent: kindPhase},
	{Kind: "file-changes", Level: 2, Text: "file changes"},
	{Kind: "testing", Level: 2, Text: "testing plan"},
	{Kind: kindDependencies, Level: 2, Text: "dependencies"},
	{Kind: kindReferences, Level: 2, Text: "references"},
	{Kind: "open-questions", Level: 2, Text: "open questions"},
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
