// Package design interprets a DESIGN document as a typed value.
//
// Parse returns a Doc with one field per section of the DESIGN template, and
// Validate reports the rules only a typed model can check. Neither touches
// the filesystem, and neither reads the document's type name (ADR-0002 R7):
// spans are located by region kind, so a repo's custom type whose documents
// carry an overview, goals, and open questions parses with this package.
//
// The package has no grammar of its own beyond the shared field rules
// (DESIGN-0014 §2.9), and Detailed Design is why. It is the longest section a
// design has and the one authors shape most freely, so its numbered
// subsections are reported as part of the region's body rather than split
// into a structure. A reader that split on the numbers would be inventing a
// grammar out of one repo's habits, and a consumer that wants the subsections
// walks docparse.Headings over the body, where a heading is a fact.
//
// A document that carries no docz markers parses too. Its regions are
// inferred from its headings and Doc.Inferred is set, which is the
// backwards-compatibility path for every document created before markers
// existed. Markers, once present, are authoritative.
package design

import (
	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// Doc is a parsed DESIGN document: one field per section of the template.
//
// Every string is copied out of the input, so a caller may reuse or modify
// the bytes it passed to Parse.
//
// There is no sentinel error to go with it. Unlike an IMPL, which is nothing
// without its phases, a design with only a title is the normal first state of
// a design — so every section a document is missing leaves its field zero and
// is reported by validate.Document against the document's schema, and Parse
// fails only for bytes it cannot read at all.
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

	// Overview is the overview region's body: what is being designed and why.
	Overview string

	// Goals and NonGoals are the two lists under the template's "Goals and
	// Non-Goals" heading. That heading is not itself a region, so both are
	// top-level spans rather than children of it.
	Goals    []kinds.Item
	NonGoals []kinds.Item

	// Background is the background region's body: context and prior art.
	Background string

	// DetailedDesign is the whole detailed-design region's body. Its numbered
	// subsections are the author's structure, not a grammar — see the package
	// comment — so they stay in the body, headings and all.
	DetailedDesign string

	// APIChanges is the body of the "API / Interface Changes" region.
	APIChanges string

	// DataModel is the body of the data-model region.
	DataModel string

	// Testing is the body of the testing-strategy region. Prose, not
	// checkboxes: a design says how a thing will be tested, and the
	// checkboxes that track the testing live in the IMPL that implements it.
	Testing string

	// Rollout is the body of the "Migration / Rollout Plan" region.
	Rollout string

	// OpenQuestions is nil when the document has none. Design is the type the
	// section matters most to, because a question left open past approval is
	// what design.status.open-question reports.
	OpenQuestions []kinds.Question

	// Decisions is the rows of the decisions table, nil when the document has
	// no decisions region. The DESIGN template does not ship one — it is a
	// shared kind a document grows by hand once its questions start being
	// answered — which is why design.decisions.mismatch is silent for a
	// document that has none.
	Decisions []kinds.Decision

	// References is the references region's bullets.
	References []kinds.Reference
}
