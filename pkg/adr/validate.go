package adr

import (
	"fmt"
	"regexp"
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
	// rejected document, and the two rejections — no frontmatter, CR line
	// endings — have no rule of their own, so they get this. The generic tier
	// reports the same document's underlying problem in its own family, and
	// this says only that the typed model is unavailable, which is why every
	// other check is silent for it.
	CodeParse = "adr.parse"

	CodeDecisionEmpty         = "adr.decision.empty"
	CodeConsequencesEmpty     = "adr.consequences.empty"
	CodeSupersededNoReference = "adr.superseded.no-reference"
)

// The two statuses the rules read, folded. A repo configures its own status
// list and may capitalise these however it likes, so nothing compares them
// literally.
const (
	statusAccepted   = "accepted"
	statusSuperseded = "superseded"
)

// adrIDPattern matches a reference to an ADR by id, in a bullet's text, in a
// link target, or in prose. Case-insensitive because a path spells the id
// lower-cased and a forward pointer in a path is still a forward pointer.
var adrIDPattern = regexp.MustCompile(`(?i)\bADR-\d+\b`)

// Validate reports the findings only the typed model can see.
//
// It is the type tier of DESIGN-0015 §4, not the whole of validation: the
// generic tier (validate.Document) checks markers, frontmatter, and region
// presence against the document's schema, and the two run side by side. So
// nothing here repeats a generic finding — an ADR with no decision region at
// all is region.missing there, and adr.decision.empty here only once the
// status claims the decision has been made.
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
			Detail:   strings.TrimPrefix(err.Error(), "adr: "),
		}}
	}

	// The same call Parse makes, so a finding's line and a parsed value can
	// never disagree about where a region is.
	regions, _ := kinds.ResolveRegions(doc, headings)

	// Three rules, each of which reports at most once, so the worst case never
	// grows the slice.
	out := make([]validate.Finding, 0, 3)

	out = append(out, emptyDecision(doc, &parsed, regions)...)
	out = append(out, emptyConsequences(doc, &parsed, regions)...)
	out = append(out, supersededWithoutReference(doc, &parsed, regions)...)

	// A correct document reports nil rather than an empty slice, the way every
	// reader in kinds does: a caller that marshals the result gets no findings
	// rather than an empty list of them.
	if len(out) == 0 {
		return nil
	}

	return out
}

// emptyDecision reports an accepted ADR whose decision section says nothing.
//
// The one error this package reports, and the only rule that reads the status:
// an ADR is the record of a decision, so one the repo has accepted with an
// empty Decision has recorded nothing a reader can act on. A proposed ADR with
// an empty decision is a draft, which is exactly what a draft looks like.
func emptyDecision(doc []byte, parsed *Doc, regions []docparse.Region) []validate.Finding {
	if !strings.EqualFold(string(parsed.Status), statusAccepted) || parsed.Decision != "" {
		return nil
	}

	return []validate.Finding{{
		Code:     CodeDecisionEmpty,
		Severity: validate.Error,
		Line:     regionLine(doc, regions, kindDecision),
		Kind:     kindDecision,
		Detail: fmt.Sprintf("status is %q but the decision section is empty",
			string(parsed.Status)),
	}}
}

// emptyConsequences reports an ADR that lists no consequences at all.
//
// One finding rather than three. The three lists are one section as a reader
// sees it, and a decision with nothing neutral to say about it is the normal
// case — what is worth reporting is a document that has not said what the
// decision costs anybody, in any of the three.
func emptyConsequences(doc []byte, parsed *Doc, regions []docparse.Region) []validate.Finding {
	c := &parsed.Consequences
	if len(c.Positive) > 0 || len(c.Negative) > 0 || len(c.Neutral) > 0 {
		return nil
	}

	return []validate.Finding{{
		Code:     CodeConsequencesEmpty,
		Severity: validate.Warning,
		Line:     regionLine(doc, regions, kindConsequences),
		Kind:     kindConsequences,
		Detail:   "no positive, negative, or neutral consequences are listed",
	}}
}

// supersededWithoutReference reports a superseded ADR that does not say what
// replaced it.
//
// A warning rather than an error: the decision it records still happened, and
// the history is the reason the document is kept. But a superseded ADR with no
// forward pointer is a dead end — the next reader has no way to reach the
// decision that replaced it short of reading every other ADR in the directory.
func supersededWithoutReference(
	doc []byte, parsed *Doc, regions []docparse.Region,
) []validate.Finding {
	if !strings.EqualFold(string(parsed.Status), statusSuperseded) ||
		namesAnotherADR(parsed) {
		return nil
	}

	return []validate.Finding{{
		Code:     CodeSupersededNoReference,
		Severity: validate.Warning,
		Line:     regionLine(doc, regions, kindReferences),
		Kind:     kindReferences,
		Detail:   "status is Superseded but nothing in the document names another ADR",
	}}
}

// namesAnotherADR reports whether the document points at an ADR other than
// itself.
//
// The reference list and the summary and context prose all count. The corpus
// records a supersession in prose at least as often as in the list — ADR-0002
// names ADR-0001 in its summary, in its context, and again in its references —
// so a rule that read only the list would fire on a document that says what
// replaced it in its first paragraph.
func namesAnotherADR(parsed *Doc) bool {
	if otherADRIn(parsed.Summary, parsed.ID) || otherADRIn(parsed.Context, parsed.ID) {
		return true
	}

	for _, ref := range parsed.References {
		if otherADRIn(ref.Text, parsed.ID) || otherADRIn(ref.URL, parsed.ID) {
			return true
		}
	}

	return false
}

// otherADRIn reports whether s carries an ADR id that is not own.
//
// A document that cites only itself has not pointed anywhere, so its own id is
// skipped: the whole point of the rule is the forward pointer.
func otherADRIn(s, own string) bool {
	for _, id := range adrIDPattern.FindAllString(s, -1) {
		if !strings.EqualFold(id, own) {
			return true
		}
	}

	return false
}

// regionLine is the document line a finding about a whole region points at:
// the region's first heading, or the region's own first line when it has none.
//
// The heading rather than the marker, because the heading is what a person
// scrolls to when they go to fix the section. A kind the document does not
// carry has no line at all and reports 0, which validate reads as a finding
// about the whole document — better than pointing at a line that means
// something else.
func regionLine(doc []byte, regions []docparse.Region, kind string) int {
	for _, r := range regions {
		if r.Kind != kind || !r.Closed {
			continue
		}

		heads := docparse.Headings(kinds.RegionBytes(doc, r))
		if len(heads) == 0 {
			return r.Start
		}

		return r.Start + heads[0].Line
	}

	return 0
}
