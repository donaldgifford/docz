// Package adr interprets an ADR document as a typed value.
//
// Parse returns a Doc with one field per section of the ADR template, and
// Validate reports the rules only a typed model can check. Neither touches
// the filesystem, and neither reads the document's type name (ADR-0002 R7):
// spans are located by region kind, so a repo's custom type whose documents
// carry a decision region and consequence lists parses with this package.
//
// The package has no grammar of its own beyond the field rules the five type
// packages share (DESIGN-0014 §2.9): a string field is its region's body, a
// list field is the region's top-level items, and a shared kind is read by
// its kinds reader. That is also why there is no adr equivalent of
// impl.ErrNoPhases — a document missing a region parses with that field zero
// and is reported by validate.Document against its schema, because an ADR
// under review is normally half-written and a parser that refused one would
// be useless during the review it was written for.
//
// A document that carries no docz markers parses too. Its regions are
// inferred from its headings and Doc.Inferred is set, which is the
// backwards-compatibility path for every document created before markers
// existed. Markers, once present, are authoritative.
package adr

import (
	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// Doc is a parsed ADR document: one field per section of the template.
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

	// Summary is the summary region's body.
	Summary string

	// Context is the context region's body: what motivated the decision.
	Context string

	// Decision is the decision region's body, its Supporting Data
	// subsection included. The subsection is evidence for the decision
	// rather than a section beside it, so cutting it out would drop the
	// data from the one field a reader asks for the decision.
	Decision string

	// Consequences are the three consequence lists, each nested one level
	// inside the consequences region.
	Consequences Consequences

	// Alternatives is nil when the document names none.
	Alternatives []kinds.Alternative

	// OpenQuestions is nil when the document has none. The section is
	// optional for every type but design.
	OpenQuestions []kinds.Question

	References []kinds.Reference
}

// Consequences are an ADR's three consequence lists.
//
// Three fields rather than one list carrying a polarity, because the
// template gives each its own region and a consumer reads them apart: a
// release note shows what a decision bought, a review shows what it cost.
// Folding them together would make either of those a filter, and a filter
// over a list is a guess about which bullet meant what.
type Consequences struct {
	Positive []kinds.Item
	Negative []kinds.Item
	Neutral  []kinds.Item
}
