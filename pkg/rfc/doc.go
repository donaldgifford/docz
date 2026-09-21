// Package rfc interprets an RFC document as a typed value.
//
// EXPERIMENTAL until v2.0.0: the surface may change between betas
// (ADR-0002 Decision 7). The five packages frozen at v1.0.0 are not
// affected; this one is not among them.
//
// Parse returns a Doc with one field per section of the RFC template, and
// Validate reports the rules only a typed model can check. Neither touches
// the filesystem, and neither reads the document's type name (ADR-0002 R7):
// spans are located by region kind, so a repo's custom type whose documents
// carry a risks table and a set of alternatives parses with this package.
//
// The package has no grammar of its own. Every field follows the rules the
// five type packages share (DESIGN-0014 §2.9) — a string field is its
// region's body, a list field is its region's reader, the risks table is
// read by column position — and the shared kinds come from the kinds
// package, so an open question means the same thing here as in an IMPL.
//
// A document that carries no docz markers parses too. Its regions are
// inferred from its headings and Doc.Inferred is set, which is the
// backwards-compatibility path for every document created before markers
// existed. Markers, once present, are authoritative.
package rfc

import (
	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// Doc is a parsed RFC document: one field per section of the template.
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

	// Problem is the problem region's body, its Supporting Data subsection
	// included: the heading and the evidence under it are part of the
	// statement, not a section of their own. The template does not mark the
	// subsection, so there is nothing to read it separately with, and an
	// author who moves a number from the prose into the table has not
	// changed what the problem is.
	Problem string

	// Proposal is the proposal region's body.
	Proposal string

	// Alternatives are the options the RFC considered and did not take, read
	// from whichever shape the document uses (kinds.Alternatives).
	Alternatives []kinds.Alternative

	// Risks are the rows of the risks table.
	Risks []Risk

	// Criteria are the success criteria at the document level. An RFC has
	// one set for the whole proposal, unlike an IMPL, which has one per
	// phase.
	Criteria []kinds.Criterion

	// OpenQuestions is nil when the document has none. The section is
	// optional for every type but design.
	OpenQuestions []kinds.Question

	References []kinds.Reference
}

// Risk is one row of the risks table.
type Risk struct {
	Risk       string
	Impact     string
	Likelihood string
	Mitigation string

	// Line is the 1-based line of the row in the document.
	Line int
}
