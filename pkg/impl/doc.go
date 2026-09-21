// Package impl interprets an IMPL document as a typed value.
//
// EXPERIMENTAL until v2.0.0: the surface may change between betas
// (ADR-0002 Decision 7). The five packages frozen at v1.0.0 are not
// affected; this one is not among them.
//
// Parse returns a Doc with one field per section of the IMPL template, and
// Validate reports the rules only a typed model can check. Neither touches
// the filesystem, and neither reads the document's type name (ADR-0002 R7):
// spans are located by region kind, so a repo's custom type whose documents
// carry a phase region with tasks and criteria parses with this package.
//
// This is the one type package with a grammar of its own beyond the shared
// field rules (DESIGN-0014 §3). The tolerances in it are not guesses — each
// exists because a real document in the fleet needed it (INV-0010): tasks
// wrap across lines, a verify line names a command to run, a deferred task
// says why, and a skipped one is struck through with a note.
//
// A document that carries no docz markers parses too. Its regions are
// inferred from its headings and Doc.Inferred is set, which is the
// backwards-compatibility path for every document created before markers
// existed. Markers, once present, are authoritative.
package impl

import (
	"errors"
	"fmt"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// ErrNoPhases reports a document with no phases at all.
//
// This is the one content failure Parse returns rather than reports. A
// document missing any other region parses with that field zero and is
// reported by validate.Document against its schema; an IMPL with no phases
// has nothing of the type in it, so there is no Doc to return.
var ErrNoPhases = errors.New("impl: no phases found")

// DuplicatePhaseError reports two phases claiming the same token.
//
// It is an error rather than a finding because a token is an address: Task
// IDs are "<token>.<index>", so two phases numbered 3 make "3.1" ambiguous
// and a consumer that checks off 3.1 would write to whichever the parser
// happened to reach first.
type DuplicatePhaseError struct {
	Token string
	Lines []int
}

func (e *DuplicatePhaseError) Error() string {
	numbers := make([]string, 0, len(e.Lines))
	for _, line := range e.Lines {
		numbers = append(numbers, fmt.Sprint(line))
	}

	return fmt.Sprintf("impl: two phases claim the token %q, at lines %s",
		e.Token, strings.Join(numbers, " and "))
}

// Doc is a parsed IMPL document: one field per section of the template.
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

	// Objective is the objective region's body.
	Objective string

	// Implements holds the document IDs from the "**Implements:**" field.
	Implements []string

	InScope    []kinds.Item
	OutOfScope []kinds.Item

	// Phases are the phase regions in document order.
	Phases []Phase

	// FileChanges are the rows of the file-changes table.
	FileChanges []FileChange

	// Testing are the checkboxes in the testing region. They are never
	// tasks: a consumer counting progress must not include them, which is
	// why they have their own field and their own type (DESIGN-0014 §3).
	Testing []docparse.TaskItem

	Dependencies string

	// OpenQuestions is nil when the document has none. The section is
	// optional for every type but design.
	OpenQuestions []kinds.Question

	Decisions  []kinds.Decision
	References []kinds.Reference
}

// FileChange is one row of the file-changes table.
type FileChange struct {
	File        string
	Action      string
	Description string

	// Line is the 1-based line of the row in the document.
	Line int
}

// Phase is one phase of the plan.
type Phase struct {
	// Index is the 1-based ordinal among phases in document order. For a
	// document that numbers its phases from 1, it equals Token.
	Index int

	// Token is the heading token: "1", "A", "2B". It is the phase's address,
	// and it is what a Task ID is built from, so a consumer addresses a
	// phase by what the document calls it rather than by where it sits.
	Token string

	// Title is the text after "Phase <token>:", inline markdown stripped.
	// Empty when the template's placeholder comment is still there.
	Title string

	// Description is the prose between the phase heading and its first
	// nested region, comments removed and trimmed.
	Description string

	Tasks []Task

	// Criteria is nil when the phase has no criteria region.
	Criteria []kinds.Criterion

	// Line is the 1-based line of the phase heading.
	Line int
}

// Task is one checkbox item of a phase's task list.
type Task struct {
	// ID is "<phase token>.<index>", with a 1-based index in document
	// order: "2.3" is the third task of phase 2.
	ID string

	// Text is the task, folded across its continuation lines. The verify
	// line and any deferred or skipped marker text are removed, so Text is
	// what the task asks for and nothing about its state. Other inline
	// markdown is kept verbatim.
	Text string

	Checked bool

	// Verify is the command from the task's verify line, "" when it has
	// none.
	Verify string

	// Deferred is non-nil when the task carries a deferred marker.
	Deferred *Marker

	// Skipped is non-nil when the task is struck through with a skipped
	// note. A skipped task keeps its ID, so the numbering of the tasks
	// after it does not shift when one is skipped.
	Skipped *Marker

	// Line is the 1-based line of the checkbox, byte-accurate against the
	// input: it is the line docwrite.CheckTask splices at.
	Line int

	// EndLine is the last continuation line, equal to Line for a task that
	// fits on one.
	EndLine int
}

// Marker is a deferred or skipped annotation on a task.
type Marker struct {
	// Note is the reason text, "" when the marker carries none.
	Note string

	// Line is the line the marker sits on, which may be a continuation
	// line rather than the checkbox.
	Line int
}

// The four lookups below take Doc by value, which DESIGN-0014 §2.9 pins as
// the published surface. Doc is past the linter's size threshold, and a
// pointer receiver would be the cheaper call — but it would also make
// `impl.Parse` results awkward to use from a slice or a map, and it would
// invite a caller to believe a lookup can mutate the document. A Doc is a
// snapshot of bytes somebody already read; copying it is the honest cost of
// saying so.

// Task returns the task with the given ID.
//
//nolint:gocritic // value receiver is the published surface (DESIGN-0014 §2.9)
func (d Doc) Task(id string) (Task, bool) {
	for _, phase := range d.Phases {
		for _, task := range phase.Tasks {
			if task.ID == id {
				return task, true
			}
		}
	}

	return Task{}, false
}

// Phase returns the phase with the given token. The lookup is folded, so a
// document that writes "Phase a:" is reachable as "A".
//
//nolint:gocritic // value receiver is the published surface (DESIGN-0014 §2.9)
func (d Doc) Phase(token string) (Phase, bool) {
	for _, phase := range d.Phases {
		if strings.EqualFold(phase.Token, token) {
			return phase, true
		}
	}

	return Phase{}, false
}

// Tasks returns every task across every phase in document order.
//
//nolint:gocritic // value receiver is the published surface (DESIGN-0014 §2.9)
func (d Doc) Tasks() []Task {
	total := 0
	for _, phase := range d.Phases {
		total += len(phase.Tasks)
	}

	out := make([]Task, 0, total)
	for _, phase := range d.Phases {
		out = append(out, phase.Tasks...)
	}

	return out
}

// Progress counts checked tasks over the tasks that could be checked.
//
// A skipped task is in neither count. It is work that will not happen, so
// including it in the total would make a finished plan read as unfinished
// forever, and counting it as done would claim work nobody did.
//
//nolint:gocritic // value receiver is the published surface (DESIGN-0014 §2.9)
func (d Doc) Progress() (done, total int) {
	for _, task := range d.Tasks() {
		if task.Skipped != nil {
			continue
		}

		total++

		if task.Checked {
			done++
		}
	}

	return done, total
}
