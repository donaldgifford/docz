package runbook

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
)

// The codes this package reports (DESIGN-0019 §4). Constants because a
// consumer filters on them and a caller's allow-list and the emitter have to
// spell them the same.
const (
	// CodeParse reports a document Parse would not return a Doc for, other
	// than for a duplicate token, which has a code of its own. Every other
	// check reads a parsed Doc, so a rejected document gets this one finding
	// and nothing else.
	CodeParse = "runbook.parse"

	// A procedure token is a heading's "Procedure 2B:", not a secret; gosec
	// matches the word.
	CodeDuplicateToken = "runbook.procedure.duplicate-token" //nolint:gosec // not a credential
	CodeNoTitle        = "runbook.procedure.no-title"
	CodeNoSteps        = "runbook.procedure.no-steps"

	CodeStepEmpty     = "runbook.step.empty"
	CodeStepUnordered = "runbook.step.unordered"

	CodeNoSymptom         = "runbook.scenario.no-symptom"
	CodeScenarioNoSteps   = "runbook.scenario.no-steps"
	CodeNoOwner           = "runbook.overview.no-owner"
	CodeStatusNoProcedure = "runbook.status.no-procedure"

	CodeBadDate     = "runbook.last-verified.bad-date"
	CodeBadPR       = "runbook.last-verified.bad-pr"
	CodeBadCommit   = "runbook.last-verified.bad-commit"
	CodeNoVerifier  = "runbook.last-verified.no-verifier"
	CodeExtraRows   = "runbook.last-verified.extra-rows"
	CodeNotVerified = "runbook.last-verified.missing"
)

// statusActive is the status that says a runbook is in use, which is what
// the two status rules key on. Compared folded, as statuses are everywhere.
const statusActive = "Active"

var (
	// isoDate is the Date cell's shape. Only the shape: a date's age is for
	// a consumer to judge, and a rule that read the clock would make the same
	// document pass on Monday and fail on Tuesday.
	isoDate = regexp.MustCompile(`^\d{4}-(0[1-9]|1[012])-(0[1-9]|[12]\d|3[01])$`)

	// prRef is the PR cell's shape: "#137", a URL, or a markdown link to one.
	prRef = regexp.MustCompile(`^(#\d+|https?://\S+|\[[^\]]+\]\(https?://[^)\s]+\))$`)

	// commitSHA is the Commit cell's shape, an abbreviated or full SHA. Code
	// span backticks are allowed around it, since that is how most people
	// write one.
	commitSHA = regexp.MustCompile("^`?[0-9a-fA-F]{7,40}`?$")
)

// Validate reports the findings only the typed model can see.
//
// It is the type tier of DESIGN-0015 §4, run beside the generic tier rather
// than instead of it, so nothing here repeats a generic finding: a procedure
// with no steps region is region.missing there and runbook.procedure.no-steps
// here, and a steps region with no ordered item at all is steps.not-ordered
// there and silent here.
//
// No rule reads the clock.
func Validate(doc []byte) []validate.Finding {
	parsed, err := Parse(doc)
	if err != nil {
		return []validate.Finding{parseFinding(err)}
	}

	lines := strings.Split(strings.TrimSuffix(string(doc), "\n"), "\n")
	regions, _ := kinds.ResolveRegions(doc, headings)

	var out []validate.Finding

	out = append(out, overviewFindings(&parsed, regions)...)
	out = append(out, lastVerifiedFindings(doc, &parsed, regions)...)
	out = append(out, statusFindings(&parsed)...)

	for i := range parsed.Procedures {
		out = append(out, procedureFindings(&parsed.Procedures[i], lines)...)
	}

	for i := range parsed.Scenarios {
		out = append(out, scenarioFindings(&parsed.Scenarios[i], lines)...)
	}

	out = append(out, unorderedFindings(lines, regions)...)

	return out
}

// parseFinding turns a Parse error into the one finding a rejected document
// gets.
func parseFinding(err error) validate.Finding {
	if dup, ok := errors.AsType[*DuplicateProcedureError](err); ok {
		line := 0
		if len(dup.Lines) > 1 {
			// The second procedure to claim the token is the one to rename.
			line = dup.Lines[1]
		}

		return validate.Finding{
			Code:     CodeDuplicateToken,
			Severity: validate.Error,
			Line:     line,
			Kind:     kindProcedure,
			Detail: fmt.Sprintf("procedure token %q is claimed twice, so its step IDs are ambiguous",
				dup.Token),
		}
	}

	return validate.Finding{
		Code:     CodeParse,
		Severity: validate.Error,
		Detail:   strings.TrimPrefix(err.Error(), "runbook: "),
	}
}

// overviewFindings reports an overview with no owner. A document with no
// overview region at all is the generic tier's region.missing, so this says
// nothing about it.
func overviewFindings(parsed *Doc, regions []docparse.Region) []validate.Finding {
	at, ok := firstRegion(regions, kindOverview)
	if !ok || parsed.Owner != "" {
		return nil
	}

	return []validate.Finding{{
		Code:     CodeNoOwner,
		Severity: validate.Warning,
		Line:     at.Start,
		Kind:     kindOverview,
		Detail:   "the overview names no owner, so nobody is answerable for this runbook",
	}}
}

// lastVerifiedFindings checks the shape of each filled cell of the Last
// Verified row, and that the table has one row.
func lastVerifiedFindings(doc []byte, parsed *Doc, regions []docparse.Region) []validate.Finding {
	var out []validate.Finding

	if at, ok := firstRegion(regions, kindLastVerified); ok {
		if rows := dataRows(kinds.RegionBytes(doc, at)); rows > 1 {
			out = append(out, validate.Finding{
				Code:     CodeExtraRows,
				Severity: validate.Warning,
				Line:     at.Start,
				Kind:     kindLastVerified,
				Detail: fmt.Sprintf("the table has %d rows; only the first is read, "+
					"and re-verifying replaces it rather than adding one", rows),
			})
		}
	}

	v := parsed.LastVerified
	if v == nil {
		return out
	}

	cellFinding := func(code string, sev validate.Severity, detail string) {
		out = append(out, validate.Finding{
			Code: code, Severity: sev, Line: v.Line, Kind: kindLastVerified, Detail: detail,
		})
	}

	if v.Date != "" && !isoDate.MatchString(v.Date) {
		cellFinding(CodeBadDate, validate.Error,
			fmt.Sprintf("date %q is not YYYY-MM-DD", v.Date))
	}

	if v.PR != "" && !prRef.MatchString(v.PR) {
		cellFinding(CodeBadPR, validate.Warning,
			fmt.Sprintf("PR %q is neither #N nor a URL", v.PR))
	}

	if v.Commit != "" && !commitSHA.MatchString(v.Commit) {
		cellFinding(CodeBadCommit, validate.Error,
			fmt.Sprintf("commit %q is not 7 to 40 hex characters", v.Commit))
	}

	if v.Date != "" && len(v.VerifiedBy) == 0 {
		cellFinding(CodeNoVerifier, validate.Error,
			"the row has a date but names nobody who ran it")
	}

	return out
}

// dataRows counts the first table's body rows.
func dataRows(region []byte) int {
	tables := docparse.Tables(region)
	if len(tables) == 0 {
		return 0
	}

	return len(tables[0].Rows)
}

// statusFindings reports an Active runbook that nobody has run, or that has
// nothing in it to run.
func statusFindings(parsed *Doc) []validate.Finding {
	if !strings.EqualFold(string(parsed.Status), statusActive) {
		return nil
	}

	var out []validate.Finding

	if parsed.LastVerified == nil {
		out = append(out, validate.Finding{
			Code:     CodeNotVerified,
			Severity: validate.Warning,
			Kind:     kindLastVerified,
			Detail:   "the runbook is Active but its Last Verified row is empty",
		})
	}

	if len(parsed.Procedures) == 0 && len(parsed.Scenarios) == 0 {
		out = append(out, validate.Finding{
			Code:     CodeStatusNoProcedure,
			Severity: validate.Error,
			Detail:   "the runbook is Active but has neither a procedure nor a scenario",
		})
	}

	return out
}

// procedureFindings reports the rules that read one procedure.
func procedureFindings(p *Procedure, lines []string) []validate.Finding {
	var out []validate.Finding

	if p.Title == "" {
		// A warning: the template's own placeholder is a comment, so a fresh
		// document has one and is not broken.
		out = append(out, validate.Finding{
			Code:     CodeNoTitle,
			Severity: validate.Warning,
			Line:     p.Line,
			Kind:     kindProcedure,
			Detail:   fmt.Sprintf("procedure %s has no title", p.Token),
		})
	}

	if len(p.Steps) == 0 {
		out = append(out, validate.Finding{
			Code:     CodeNoSteps,
			Severity: validate.Error,
			Line:     p.Line,
			Kind:     kindProcedure,
			Detail:   fmt.Sprintf("procedure %s has no steps", p.Token),
		})
	}

	out = append(out, emptySteps(p.Steps, lines)...)
	out = append(out, emptySteps(p.Rollback, lines)...)

	return out
}

// scenarioFindings reports the rules that read one scenario.
func scenarioFindings(s *Scenario, lines []string) []validate.Finding {
	var out []validate.Finding

	if s.Symptom == "" {
		out = append(out, validate.Finding{
			Code:     CodeNoSymptom,
			Severity: validate.Warning,
			Line:     s.Line,
			Kind:     kindScenario,
			Detail:   fmt.Sprintf("scenario %d names no symptom", s.Index),
		})
	}

	if len(s.Steps) == 0 {
		out = append(out, validate.Finding{
			Code:     CodeScenarioNoSteps,
			Severity: validate.Error,
			Line:     s.Line,
			Kind:     kindScenario,
			Detail:   fmt.Sprintf("scenario %d has no steps", s.Index),
		})
	}

	return append(out, emptySteps(s.Steps, lines)...)
}

// emptySteps reports a numbered item with nothing after the number, at any
// depth.
//
// Read off the item line as written, not off Text. Text has comments
// removed, so the template's "1. <!-- how to undo it -->" has empty Text,
// and that is a placeholder waiting to be filled rather than a step somebody
// forgot to write.
func emptySteps(steps []Step, lines []string) []validate.Finding {
	var out []validate.Finding

	for _, s := range steps {
		if s.Line >= 1 && s.Line <= len(lines) {
			items := docparse.ListItems([]byte(lines[s.Line-1]))
			if len(items) == 1 && items[0].Text == "" {
				out = append(out, validate.Finding{
					Code:     CodeStepEmpty,
					Severity: validate.Error,
					Line:     s.Line,
					Kind:     kindSteps,
					Detail:   fmt.Sprintf("step %s has no text", s.ID),
				})
			}
		}

		out = append(out, emptySteps(s.Children, lines)...)
	}

	return out
}

// unorderedFindings reports each bullet at step level in a steps region.
func unorderedFindings(lines []string, regions []docparse.Region) []validate.Finding {
	var out []validate.Finding

	for _, r := range regions {
		if r.Kind != kindSteps || !r.Closed {
			continue
		}

		for _, line := range stepLevelBullets(lines, r) {
			out = append(out, validate.Finding{
				Code:     CodeStepUnordered,
				Severity: validate.Warning,
				Line:     line,
				Kind:     kindSteps,
				Detail:   "a bullet at step level is not a step; number it, or indent it under one",
			})
		}
	}

	return out
}

// firstRegion returns the first closed region of a kind.
func firstRegion(regions []docparse.Region, kind string) (docparse.Region, bool) {
	for _, r := range regions {
		if r.Kind == kind && r.Closed {
			return r, true
		}
	}

	return docparse.Region{}, false
}
