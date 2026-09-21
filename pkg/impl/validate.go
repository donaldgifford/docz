package impl

import (
	"errors"
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
	// rejected document, and a rejection that is not a duplicate token has no
	// rule of its own, so it gets this. The generic tier reports the same
	// document's underlying problem in its own family — no frontmatter is
	// frontmatter.missing there — and this says only that the typed model is
	// unavailable, which is why every other check is silent for it.
	CodeParse = "impl.parse"

	// A phase token is a heading's "Phase 2B:", not a secret; gosec matches
	// the word.
	CodeDuplicateToken  = "impl.phase.duplicate-token" //nolint:gosec // not a credential
	CodeNoHeading       = "impl.phase.no-heading"
	CodeNoTasks         = "impl.phase.no-tasks"
	CodeNoTitle         = "impl.phase.no-title"
	CodeTaskEmpty       = "impl.task.empty"
	CodeVerifyNoCommand = "impl.task.verify-no-command"
	CodeSkippedNoNote   = "impl.task.skipped-no-note"
)

// Validate reports the findings only the typed model can see.
//
// It is the type tier of DESIGN-0015 §4, not the whole of validation: the
// generic tier (validate.Document) checks markers, frontmatter, and region
// presence against the document's schema, and the two run side by side. So
// nothing here repeats a generic finding — a phase with no tasks region is
// region.missing there and impl.phase.no-tasks here, and the two say
// different things: one that the section is absent, one that the plan has a
// phase nobody can make progress on.
//
// A document Parse rejects yields exactly one finding. Every check below
// reads a parsed Doc, so reporting more would mean guessing at a document the
// parser could not read.
func Validate(doc []byte) []validate.Finding {
	parsed, err := Parse(doc)
	if err != nil {
		return []validate.Finding{parseFinding(err)}
	}

	lines := strings.Split(strings.TrimSuffix(string(doc), "\n"), "\n")

	var out []validate.Finding

	out = append(out, unreadablePhases(doc, &parsed)...)

	for i := range parsed.Phases {
		out = append(out, phaseFindings(&parsed.Phases[i], lines)...)
	}

	return out
}

// parseFinding turns a Parse error into the one finding a rejected document
// gets.
func parseFinding(err error) validate.Finding {
	var dup *DuplicatePhaseError
	if errors.As(err, &dup) {
		line := 0
		if len(dup.Lines) > 1 {
			// The second phase to claim the token, which is the one to rename:
			// the first is where the token legitimately belongs.
			line = dup.Lines[1]
		}

		return validate.Finding{
			Code:     CodeDuplicateToken,
			Severity: validate.Error,
			Line:     line,
			Kind:     kindPhase,
			Detail: fmt.Sprintf("phase token %q is claimed twice, so its task IDs are ambiguous",
				dup.Token),
		}
	}

	return validate.Finding{
		Code:     CodeParse,
		Severity: validate.Error,
		Detail:   strings.TrimPrefix(err.Error(), "impl: "),
	}
}

// unreadablePhases reports a phase region whose heading Parse could not read.
//
// Parse skips such a region rather than inventing a token for it, so the
// finding cannot come from the Doc — it is the difference between the phase
// regions the document has and the phases Parse returned. Resolving the
// regions a second time is the same call Parse makes, so the two agree about
// what a phase region is.
func unreadablePhases(doc []byte, parsed *Doc) []validate.Finding {
	regions, _ := kinds.ResolveRegions(doc, headings)

	var out []validate.Finding

	for i := range regions {
		r := regions[i]
		if r.Kind != kindPhase || r.Depth != 0 || !r.Closed {
			continue
		}

		if becameAPhase(doc, regions[i], parsed) {
			continue
		}

		out = append(out, validate.Finding{
			Code:     CodeNoHeading,
			Severity: validate.Error,
			Line:     r.Start,
			Kind:     kindPhase,
			Detail: "phase region has no heading matching " +
				`"Phase <token>:", so its tasks have no address`,
		})
	}

	return out
}

// becameAPhase reports whether a phase region turned into one of the Doc's
// phases, matched by the line its heading sits on.
func becameAPhase(doc []byte, r docparse.Region, parsed *Doc) bool {
	heading, ok := phaseHeadingIn(doc, r)
	if !ok {
		return false
	}

	at := r.Start + heading.Line

	for _, phase := range parsed.Phases {
		if phase.Line == at {
			return true
		}
	}

	return false
}

// phaseFindings reports the rules that read one parsed phase.
func phaseFindings(phase *Phase, lines []string) []validate.Finding {
	var out []validate.Finding

	if phase.Title == "" {
		// A warning, not an error: the template's own placeholder is a
		// comment, so a freshly created document has three of these and is
		// not broken. It is still worth saying, because a phase with no title
		// reads as "Phase 2" in every list a consumer renders.
		out = append(out, validate.Finding{
			Code:     CodeNoTitle,
			Severity: validate.Warning,
			Line:     phase.Line,
			Kind:     kindPhase,
			Detail:   fmt.Sprintf("phase %s has no title", phase.Token),
		})
	}

	if len(phase.Tasks) == 0 {
		out = append(out, validate.Finding{
			Code:     CodeNoTasks,
			Severity: validate.Warning,
			Line:     phase.Line,
			Kind:     kindPhase,
			Detail:   fmt.Sprintf("phase %s has no tasks", phase.Token),
		})
	}

	for i := range phase.Tasks {
		out = append(out, taskFindings(&phase.Tasks[i], lines)...)
	}

	return out
}

// taskFindings reports the rules that read one task.
func taskFindings(task *Task, lines []string) []validate.Finding {
	var out []validate.Finding

	if task.Text == "" {
		out = append(out, validate.Finding{
			Code:     CodeTaskEmpty,
			Severity: validate.Error,
			Line:     task.Line,
			Kind:     kindTasks,
			Detail:   fmt.Sprintf("task %s has no text", task.ID),
		})
	}

	// A verify line naming no command is worse than none: it reads as covered
	// while leaving nobody able to run it.
	if task.Verify == "" && hasVerifyLine(task, lines) {
		out = append(out, validate.Finding{
			Code:     CodeVerifyNoCommand,
			Severity: validate.Warning,
			Line:     task.Line,
			Kind:     kindTasks,
			Detail: fmt.Sprintf("task %s has a verify line with no command in backticks",
				task.ID),
		})
	}

	if task.Skipped != nil && task.Skipped.Note == "" {
		// Skipping work is a decision, and a decision with no reason cannot be
		// reviewed later. This is the one place the package asks for prose.
		out = append(out, validate.Finding{
			Code:     CodeSkippedNoNote,
			Severity: validate.Warning,
			Line:     task.Skipped.Line,
			Kind:     kindTasks,
			Detail:   fmt.Sprintf("task %s is skipped with no reason given", task.ID),
		})
	}

	return out
}

// hasVerifyLine reports whether any of a task's continuation lines is a
// verify line.
//
// Read off the document rather than carried on Task, because "a verify line
// with nothing runnable on it" is a finding about how the document is written
// and not a fact the typed model needs. Task stays what a consumer reads: a
// Verify that is empty means there is no command, which is all a consumer
// acts on.
func hasVerifyLine(task *Task, lines []string) bool {
	for n := task.Line + 1; n <= task.EndLine && n <= len(lines); n++ {
		if verifyLine.MatchString(strings.TrimSpace(lines[n-1])) {
			return true
		}
	}

	return false
}
