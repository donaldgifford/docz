package investigation

import (
	"fmt"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
)

// The codes this package reports (DESIGN-0015 §4). Constants because a
// consumer filters on them and a caller's allow-list and the emitter have to
// spell them the same.
//
// The prefix is "inv." and not "investigation.", even though the package is
// named for the type in full. DESIGN-0015 §4 names the codes with the type's
// alias, which is what the CLI's --type flag and a consumer's filter both
// spell, and the two are deliberately different: a code is a user-facing
// identifier, an import path is not. This is not an oversight to be tidied up.
const (
	// CodeParse reports a document Parse would not return a Doc for.
	//
	// It is not one of the rule families: the rules below all describe a
	// document that parsed. The design requires a single finding for a
	// rejected document, and a rejection has no rule of its own, so it gets
	// this. The generic tier reports the same document's underlying problem in
	// its own family — no frontmatter is frontmatter.missing there — and this
	// says only that the typed model is unavailable, which is why every other
	// check is silent for it.
	CodeParse = "inv.parse"

	CodeContextNoTrigger   = "inv.context.no-trigger"
	CodeConclusionNoAnswer = "inv.conclusion.no-answer"
	CodeConclusionVerdict  = "inv.conclusion.verdict"
)

// Validate reports the findings only the typed model can see.
//
// It is the type tier of DESIGN-0015 §4, not the whole of validation: the
// generic tier (validate.Document) checks markers, frontmatter, and region
// presence against the document's schema, and the two run side by side. So
// nothing here repeats a generic finding — a document with no conclusion
// region is region.missing there, and inv.conclusion.no-answer here only when
// its status claims the investigation is over.
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
			Detail:   strings.TrimPrefix(err.Error(), "investigation: "),
		}}
	}

	// The same resolution Parse made, so a finding and a field cannot disagree
	// about where a region is. Both rules below report something a region does
	// not say, and a line nobody wrote has no number — so the line comes from
	// the region that should have carried it.
	regions, _ := kinds.ResolveRegions(doc, headings)

	var out []validate.Finding

	if f, ok := triggerFinding(&parsed, regions); ok {
		out = append(out, f)
	}

	if f, ok := conclusionFinding(&parsed, regions); ok {
		out = append(out, f)
	}

	return out
}

// triggerFinding reports a context region that does not say what prompted the
// investigation.
//
// A warning rather than an error, and only when the region is there: an
// investigation with no context section at all is region.missing in the
// generic tier, and saying it twice in different words would train a reader to
// skim both.
func triggerFinding(parsed *Doc, regions []docparse.Region) (validate.Finding, bool) {
	at, ok := regionStart(regions, kindContext)
	if !ok || parsed.TriggeredBy != "" {
		return validate.Finding{}, false
	}

	return validate.Finding{
		Code:     CodeContextNoTrigger,
		Severity: validate.Warning,
		Line:     at,
		Kind:     kindContext,
		Detail: `context region has no "**Triggered by:**" field, ` +
			"so nothing records what prompted the investigation",
	}, true
}

// conclusionFinding reports the one thing wrong with a conclusion, of at most
// two: an answer that is missing when the status says the work is over, or one
// that does not open with a verdict. The two cannot both hold, because the
// second needs the answer the first says is absent.
func conclusionFinding(parsed *Doc, regions []docparse.Region) (validate.Finding, bool) {
	// 0 when the document has no conclusion region, which Finding documents as
	// "the whole document" — and for the no-answer rule that is the truth: the
	// missing answer is not on any line.
	at, _ := regionStart(regions, kindConclusion)

	if parsed.Answer == "" {
		if !concluded(parsed.Status) {
			return validate.Finding{}, false
		}

		// An error, not a warning. A status of Concluded is a claim other
		// documents link to, and a consumer that renders the verdict beside the
		// link has nothing to render.
		return validate.Finding{
			Code:     CodeConclusionNoAnswer,
			Severity: validate.Error,
			Line:     at,
			Kind:     kindConclusion,
			Detail: fmt.Sprintf(
				`status %q with no "**Answer:**" field: the question is not answered`,
				string(parsed.Status)),
		}, true
	}

	if parsed.Verdict != VerdictUnknown {
		return validate.Finding{}, false
	}

	return validate.Finding{
		Code:     CodeConclusionVerdict,
		Severity: validate.Warning,
		Line:     at,
		Kind:     kindConclusion,
		Detail: fmt.Sprintf(
			"answer opens with %q, which is not yes, no, or inconclusive: "+
				"a consumer cannot read a verdict from it",
			firstWord(parsed.Answer)),
	}, true
}

// concluded reports whether a status says the investigation is over.
//
// Folded, and against the two words rather than against the configured status
// list: this rule is about a document that claims to have finished, and a repo
// that renames its other statuses has not changed what "Concluded" means.
func concluded(status config.Status) bool {
	switch strings.ToLower(strings.TrimSpace(string(status))) {
	case wordConcluded, wordInconclusive:
		return true
	default:
		return false
	}
}

// regionStart returns the first line of the named region, and whether the
// document has one. A region left unclosed is not counted: Parse skipped it,
// so a rule that read it would report against a field Parse never filled.
func regionStart(regions []docparse.Region, kind string) (int, bool) {
	for _, r := range regions {
		if r.Kind == kind && r.Closed {
			return r.Start, true
		}
	}

	return 0, false
}
