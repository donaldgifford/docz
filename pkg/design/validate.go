package design

import (
	"fmt"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
)

// The codes this package reports (DESIGN-0014 §2.9). Constants because a
// consumer filters on them and a caller's allow-list and the emitter have to
// spell them the same.
const (
	// CodeParse reports a document Parse would not return a Doc for.
	//
	// It is not one of the rule families: the three rules below all describe a
	// document that parsed. The design requires a single finding for a
	// rejected document, and a rejection has no rule of its own, so it gets
	// this. The generic tier reports the same document's underlying problem in
	// its own family — no frontmatter is frontmatter.missing there — and this
	// says only that the typed model is unavailable, which is why every other
	// check is silent for it.
	CodeParse = "design.parse"

	// CodeGoalsEmpty reports a goals region with no items in it.
	CodeGoalsEmpty = "design.goals.empty"

	// CodeStatusOpenQuestion reports a question still open at a status that
	// says the design is settled.
	CodeStatusOpenQuestion = "design.status.open-question"

	// CodeDecisionsMismatch reports a resolved question with no decisions row,
	// or a decisions row with no question.
	CodeDecisionsMismatch = "design.decisions.mismatch"
)

// The two statuses that mean the thinking is over. Compared folded against
// these names rather than looked up in config, because the rule is about what
// the words mean: a repo that renames its statuses has renamed the rule.
const (
	statusApproved    = "approved"
	statusImplemented = "implemented"
)

// Validate reports the findings only the typed model can see.
//
// It is the type tier of DESIGN-0015 §4, not the whole of validation: the
// generic tier (validate.Document) checks markers, frontmatter, and region
// presence against the document's schema, and the two run side by side. So
// nothing here repeats a generic finding — a document with no goals section at
// all is region.missing there and silent here, because "you have no goals
// section" and "your goals section is empty" are different sentences and only
// one of them is this tier's to say.
//
// A document Parse rejects yields exactly one finding. Every check below reads
// a parsed Doc, so reporting more would mean guessing at a document the parser
// could not read.
func Validate(doc []byte) []validate.Finding {
	parsed, err := Parse(doc)
	if err != nil {
		return []validate.Finding{{
			Code:     CodeParse,
			Severity: validate.Error,
			Detail:   strings.TrimPrefix(err.Error(), "design: "),
		}}
	}

	// Two of the three rules turn on whether a section is present rather than
	// on what is in it, and Doc does not record presence: an absent region and
	// an empty one both leave the field zero. Resolving the regions a second
	// time is the same call Parse makes, so the two can never disagree about
	// what a goals or a decisions region is.
	regions, _ := kinds.ResolveRegions(doc, headings)

	// Built by concatenation rather than into a preallocated slice so that a
	// document with nothing wrong with it returns nil rather than an empty
	// slice. A consumer that ranges cannot tell the two apart, but one that
	// compares against nil can, and "no findings" is the answer it is asking
	// for.
	out := emptyGoals(doc, regions, &parsed)
	out = append(out, unresolvedPastApproval(&parsed)...)
	out = append(out, decisionMismatches(regions, &parsed)...)

	return out
}

// emptyGoals reports a goals region that holds no items.
//
// A warning, not an error: the template ships the section with a bare "-" in
// it, so a design somebody started this morning trips this and is not broken.
// It is still worth saying, because a design with no goals has not said what
// it is for, and every later section is then unreviewable.
//
// Only when the region is present. An absent one is the generic tier's
// region.missing, and saying the same thing twice in two vocabularies teaches
// a reader to skim both.
func emptyGoals(doc []byte, regions []docparse.Region, parsed *Doc) []validate.Finding {
	r, ok := regionOf(regions, kindGoals)
	if !ok || len(parsed.Goals) > 0 {
		return nil
	}

	return []validate.Finding{{
		Code:     CodeGoalsEmpty,
		Severity: validate.Warning,
		Line:     headingLine(doc, r),
		Kind:     kindGoals,
		Detail:   "the goals section has no goals in it",
	}}
}

// unresolvedPastApproval reports an open question left open past approval.
//
// An error: a design that is Approved or Implemented has had its questions
// answered by definition, so an unresolved one means either the status or the
// question is wrong. Both are things a person decides, and shipping the
// document as settled hides the decision from the next reader.
//
// One finding per question rather than one for the document. A reader fixing
// this has to go to each question, and a single finding at the status line
// would name none of them.
func unresolvedPastApproval(parsed *Doc) []validate.Finding {
	if !settled(parsed.Status) {
		return nil
	}

	var out []validate.Finding

	for _, q := range parsed.OpenQuestions {
		if q.Resolved != nil {
			continue
		}

		out = append(out, validate.Finding{
			Code:     CodeStatusOpenQuestion,
			Severity: validate.Error,
			Line:     q.Line,
			Kind:     kindOpenQuestions,
			Detail: fmt.Sprintf("open question %d is unresolved but the status is %q",
				q.Number, parsed.Status),
		})
	}

	return out
}

// settled reports whether a status means the design's questions are answered.
//
// Only these two of the five. Draft and In Review are exactly when a question
// should still be open, and an Abandoned design is one nobody will act on, so
// reporting its leftovers would be asking for work on a document that has been
// put down.
func settled(status config.Status) bool {
	s := string(status)

	return strings.EqualFold(s, statusApproved) || strings.EqualFold(s, statusImplemented)
}

// decisionMismatches reports a resolved question with no row in the decisions
// table, and a row with no question.
//
// Only when the region is present. The DESIGN template ships no decisions
// section, so a design that keeps its resolutions in the questions'
// blockquotes and nowhere else is written correctly and must report nothing. A
// document that does keep a table has made the table its summary, and a
// summary that disagrees with the questions is worse than none, because a
// reader trusts it and stops scrolling.
//
// Matched by number in both directions, and a warning in both: the table is a
// convenience, and docz cannot tell whether the row or the question is the one
// that is out of date. A row whose number column is absent or not a number
// comes back as 0, which no question can be, so such a row is reported as
// matching nothing rather than quietly attached to the first question.
func decisionMismatches(regions []docparse.Region, parsed *Doc) []validate.Finding {
	if _, ok := regionOf(regions, kindDecisions); !ok {
		return nil
	}

	decided := make(map[int]bool, len(parsed.Decisions))

	for _, d := range parsed.Decisions {
		if d.Number != 0 {
			decided[d.Number] = true
		}
	}

	asked := make(map[int]bool, len(parsed.OpenQuestions))
	for _, q := range parsed.OpenQuestions {
		asked[q.Number] = true
	}

	var out []validate.Finding

	// A question that is still open is not expected in the table yet, so only
	// the resolved ones are checked this way.
	for _, q := range parsed.OpenQuestions {
		if q.Resolved == nil || decided[q.Number] {
			continue
		}

		out = append(out, validate.Finding{
			Code:     CodeDecisionsMismatch,
			Severity: validate.Warning,
			Line:     q.Line,
			Kind:     kindOpenQuestions,
			Detail: fmt.Sprintf(
				"open question %d is resolved but the decisions table has no row for it", q.Number),
		})
	}

	for _, d := range parsed.Decisions {
		if d.Number != 0 && asked[d.Number] {
			continue
		}

		out = append(out, validate.Finding{
			Code:     CodeDecisionsMismatch,
			Severity: validate.Warning,
			Line:     d.Line,
			Kind:     kindDecisions,
			Detail:   unmatchedRow(d.Number),
		})
	}

	return out
}

// unmatchedRow names why a decisions row matches no open question.
func unmatchedRow(number int) string {
	if number == 0 {
		return "this decisions row carries no question number, so it matches no open question"
	}

	return fmt.Sprintf(
		"the decisions table has a row for question %d, which the document does not ask", number)
}

// regionOf returns the first closed region of the given kind.
func regionOf(regions []docparse.Region, kind string) (docparse.Region, bool) {
	for _, r := range regions {
		if r.Kind == kind && r.Closed {
			return r, true
		}
	}

	return docparse.Region{}, false
}

// headingLine returns the document line of a region's first heading, or the
// region's own start line when it has none.
//
// A finding about a whole section points at its heading rather than at the
// marker above it: the heading is what a reader recognises, and an inferred
// region has no marker to point at at all.
func headingLine(doc []byte, r docparse.Region) int {
	heads := docparse.Headings(kinds.RegionBytes(doc, r))
	if len(heads) == 0 {
		return r.Start
	}

	return r.Start + heads[0].Line
}
