package rfc

import (
	"fmt"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
)

// The codes this package reports (DESIGN-0015 §4). Constants because a
// consumer filters on them and a caller's allow-list and the emitter have to
// spell them the same.
const (
	// CodeParse reports a document Parse would not return a Doc for.
	//
	// It is not one of the rule families: the rules below all describe a
	// document that parsed. The design requires a single finding for a
	// rejected document, and this package rejects only bytes it cannot read
	// at all, so there is one code for both reasons. The generic tier reports
	// the same document's underlying problem in its own family — no
	// frontmatter is frontmatter.missing there — and this says only that the
	// typed model is unavailable, which is why every other check is silent
	// for it.
	CodeParse = "rfc.parse"

	CodeAlternativesEmpty  = "rfc.alternatives.empty"
	CodeRisksNoMitigation  = "rfc.risks.no-mitigation"
	CodeStatusOpenQuestion = "rfc.status.open-question"
)

// statusAccepted is the status that makes an unresolved open question a
// contradiction. Compared folded, because a repo configures its own spelling
// of the word and "accepted" and "Accepted" mean the same thing.
const statusAccepted = "accepted"

// Validate reports the findings only the typed model can see.
//
// It is the type tier of DESIGN-0015 §4, not the whole of validation: the
// generic tier (validate.Document) checks markers, frontmatter, and region
// presence against the document's schema, and the two run side by side. So
// nothing here repeats a generic finding — an absent alternatives section is
// region.missing there and nothing here, while a section that is present and
// says nothing is the reverse.
//
// A document Parse rejects yields exactly one finding. Every check below
// reads a parsed Doc, so reporting more would mean guessing at a document the
// parser could not read.
func Validate(doc []byte) []validate.Finding {
	parsed, err := Parse(doc)
	if err != nil {
		return []validate.Finding{{
			Code:     CodeParse,
			Severity: validate.Error,
			Detail:   strings.TrimPrefix(err.Error(), "rfc: "),
		}}
	}

	// In document order, which is the order the template puts the sections in
	// and the order a reader walks them. Built from the first rule's return
	// rather than from an empty slice, so a clean document reports nil rather
	// than an allocated nothing — the same shape pkg/impl returns.
	out := emptyAlternatives(doc, &parsed)
	out = append(out, unmitigatedRisks(&parsed)...)
	out = append(out, openQuestionsPastAccepted(&parsed)...)

	return out
}

// emptyAlternatives reports an alternatives section that holds none.
//
// The check needs the regions and not only the Doc: an RFC with no
// alternatives section at all is region.missing in the generic tier, and
// saying it twice in different words would make a consumer that prints both
// tiers read as though there were two problems. So the finding is the
// difference between a section that exists and one that says something.
//
// A warning rather than an error, because the section is empty in every
// freshly created document and filling it in is review's job, not the
// author's first draft.
func emptyAlternatives(doc []byte, parsed *Doc) []validate.Finding {
	if len(parsed.Alternatives) > 0 {
		return nil
	}

	regions, _ := kinds.ResolveRegions(doc, headings)

	for i := range regions {
		r := regions[i]
		if r.Kind != kindAlternatives || !r.Closed {
			continue
		}

		return []validate.Finding{{
			Code:     CodeAlternativesEmpty,
			Severity: validate.Warning,
			Line:     headingLine(doc, r),
			Kind:     kindAlternatives,
			Detail: "the alternatives section holds no alternatives, " +
				"so the proposal reads as though nothing else was considered",
		}}
	}

	return nil
}

// headingLine is the document line of a region's first heading, or the
// region's own start line when it has none.
//
// The heading rather than the marker, because a finding is an address a
// person edits at and the heading is where they would look. An inferred
// region's start is the blank line above its heading, which would be a
// worse answer still.
func headingLine(doc []byte, r docparse.Region) int {
	heads := docparse.Headings(kinds.RegionBytes(doc, r))
	if len(heads) == 0 {
		return r.Start
	}

	return r.Start + heads[0].Line
}

// unmitigatedRisks reports a risks row that names a risk and no mitigation,
// one finding per row.
//
// Per row rather than per table: each row is a separate decision the author
// has to make, and a single finding would name one risk and hide the rest.
//
// A warning, because listing a risk before knowing what to do about it is how
// the section gets written. The row is still worth flagging: a risks table
// read by a consumer is a list of things somebody is handling, and a blank
// mitigation claims a handler nobody assigned.
func unmitigatedRisks(parsed *Doc) []validate.Finding {
	var out []validate.Finding

	for _, risk := range parsed.Risks {
		if risk.Mitigation != "" {
			continue
		}

		out = append(out, validate.Finding{
			Code:     CodeRisksNoMitigation,
			Severity: validate.Warning,
			Line:     risk.Line,
			Kind:     kindRisks,
			Detail:   fmt.Sprintf("risk %q has no mitigation", risk.Risk),
		})
	}

	return out
}

// openQuestionsPastAccepted reports an accepted RFC that still has unresolved
// questions.
//
// An error, and the only one this package reports: the status is the
// document's claim that the proposal was decided, and an open question is the
// document saying part of it was not. One of the two is wrong, and a reader
// who trusts the status acts on a decision nobody made.
//
// One finding per unresolved question, matching design.status.open-question.
// The two codes are siblings and a consumer filters on both, so they behave
// the same — and each unresolved question is its own piece of work, so a
// finding per question points at every line somebody has to visit rather than
// only the first.
func openQuestionsPastAccepted(parsed *Doc) []validate.Finding {
	if !strings.EqualFold(string(parsed.Status), statusAccepted) {
		return nil
	}

	var out []validate.Finding

	for _, question := range parsed.OpenQuestions {
		if question.Resolved != nil {
			continue
		}

		out = append(out, validate.Finding{
			Code:     CodeStatusOpenQuestion,
			Severity: validate.Error,
			Line:     question.Line,
			Kind:     kindOpenQuestions,
			Detail: fmt.Sprintf(
				"status is %s while open question %d is unresolved",
				parsed.Status, question.Number),
		})
	}

	return out
}
