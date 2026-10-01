// Package runbook interprets a runbook document as a typed value.
//
// EXPERIMENTAL until v2.0.0: the surface may change between betas
// (ADR-0002 Decision 7). The five packages frozen at v1.0.0 are not
// affected; this one is not among them.
//
// Parse returns a Doc with one field per section of the runbook template,
// and Validate reports the rules only a typed model can check. Neither
// touches the filesystem, and neither reads the document's type name
// (ADR-0002 R7): spans are located by region kind, so a repo's custom type
// whose documents carry procedure and scenario regions parses with this
// package.
//
// A runbook is a list of things to do, in order. Its one grammar beyond the
// shared field rules is the step (DESIGN-0019 §4): an ordered list item in a
// steps or rollback region, with ordered sub-items as its children, a fenced
// block under it as a command, and an "**Expected:**" line as what running it
// should show. A bullet under a step is a note and stays in its text.
//
// Steps are addressed by ID. A procedure's steps are "<token>.<n>", built
// from the heading's token rather than its position, so reordering
// procedures does not renumber a step somebody has linked to; a scenario's
// are "S<index>.<n>", and a rollback's are "<token>.R<n>". Children extend
// their parent's ID one level at a time: "2.3.1".
//
// A document that carries no docz markers parses too. Its regions are
// inferred from its headings and Doc.Inferred is set. Markers, once present,
// are authoritative.
package runbook

import (
	"fmt"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// DuplicateProcedureError reports two procedures claiming the same token.
//
// It is an error rather than a finding because a token is an address: step
// IDs are "<token>.<n>", so two procedures numbered 2 make "2.1" ambiguous,
// and a consumer that links to it would reach whichever the parser happened
// to read first.
type DuplicateProcedureError struct {
	Token string
	Lines []int
}

func (e *DuplicateProcedureError) Error() string {
	numbers := make([]string, 0, len(e.Lines))
	for _, line := range e.Lines {
		numbers = append(numbers, fmt.Sprint(line))
	}

	return fmt.Sprintf("runbook: two procedures claim the token %q, at lines %s",
		e.Token, strings.Join(numbers, " and "))
}

// Doc is a parsed runbook: one field per section of the template.
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
	// library never logs.
	Inferred bool

	// Overview is the overview region's prose, without its Service and
	// Owner lines, which have fields of their own.
	Overview string
	Service  string
	Owner    string

	// LastVerified is the Last Verified table's first data row. It is nil
	// while that row is still the template's empty one: a runbook nobody
	// has run has no verification, not a verification with no date.
	LastVerified *Verification

	When          []kinds.Item
	Prerequisites []kinds.Item

	// Procedures and Scenarios are in document order.
	Procedures []Procedure
	Scenarios  []Scenario

	// Escalation holds the escalation table's rows; a wholly empty row is
	// the template's placeholder and is dropped.
	Escalation []Contact

	References []kinds.Reference
}

// Verification is the most recent end-to-end run of a runbook, read from
// the first data row of its Last Verified table.
//
// Every field is the cell as written. Nothing here reads the clock, and the
// date is a string rather than a time.Time: its shape is validated, and its
// age is for a consumer to judge (DESIGN-0019 Open Question 6).
type Verification struct {
	Date   string
	PR     string
	Commit string

	// VerifiedBy is the "Verified by" cell split on commas and trimmed.
	VerifiedBy []string

	// Notes is the "**Notes:**" line under the table.
	Notes string

	// Line is the 1-based line of the data row.
	Line int
}

// Procedure is one task the runbook performs.
type Procedure struct {
	// Index is the 1-based ordinal among procedures in document order.
	Index int

	// Token is the heading token: "1", "A", "2B". It is the procedure's
	// address and what a step ID is built from. A heading with no token,
	// "Procedure: Rotate", takes its Index.
	Token string

	// Title is the text after "Procedure <token>:", comments removed.
	// Empty while the template's placeholder is still there.
	Title string

	// Description is the prose between the heading and the first nested
	// region, comments removed and trimmed.
	Description string

	Steps        []Step
	Verification []kinds.Item
	Rollback     []Step

	// Line is the 1-based line of the procedure heading.
	Line int
}

// Scenario is one troubleshooting entry: a symptom and the steps that find
// and fix its cause.
type Scenario struct {
	// Index is the 1-based ordinal among scenarios in document order, and
	// what the scenario's step IDs are built from: "S2.1".
	Index int

	// Symptom is the text after "Scenario:", comments removed.
	Symptom string

	// Alert and LikelyCause are the "**Alert:**" and "**Likely cause:**"
	// fields.
	Alert       string
	LikelyCause string

	Steps []Step

	// Line is the 1-based line of the scenario heading.
	Line int
}

// Step is one ordered list item of a steps or rollback region.
type Step struct {
	// ID is the step's address; see the package documentation.
	ID string

	// Text is the step folded across its continuation lines, comments
	// removed. A bullet under the step is kept in it as written. The
	// "**Expected:**" line and the commands are not: they have fields.
	Text string

	// Commands are the fenced blocks under the step, in order.
	Commands []Command

	// Expected is the "**Expected:**" line, folded across its own
	// continuation lines.
	Expected string

	// Children are the ordered items indented under this one.
	Children []Step

	// Line is the 1-based line of the list item; EndLine is the last line
	// that belongs to the step, its children's included.
	Line    int
	EndLine int
}

// Command is a fenced code block under a step.
type Command struct {
	// Lang is the fence's info string up to the first space: "sh", "json",
	// or "" for a bare fence.
	Lang string

	// Body is the block's content with the fence's own indentation
	// removed, without a trailing newline.
	Body string

	// Line is the 1-based line of the opening fence.
	Line int
}

// Contact is one row of the escalation table.
type Contact struct {
	Who  string
	When string
	How  string

	// Line is the 1-based line of the row.
	Line int
}

// The lookups below take Doc by value, as impl.Doc's do: a Doc is a snapshot
// of bytes somebody already read, and a pointer receiver would invite a
// caller to believe a lookup can change it.

// Procedure returns the procedure with the given token. The lookup is
// folded, so a document that writes "Procedure a:" is reachable as "A".
//
//nolint:gocritic // value receiver is the published surface, as impl.Doc's
func (d Doc) Procedure(token string) (Procedure, bool) {
	for _, p := range d.Procedures {
		if strings.EqualFold(p.Token, token) {
			return p, true
		}
	}

	return Procedure{}, false
}

// Step returns the step with the given ID, at any depth, from a procedure's
// steps, its rollback, or a scenario.
//
//nolint:gocritic // value receiver is the published surface, as impl.Doc's
func (d Doc) Step(id string) (Step, bool) {
	for _, s := range d.Steps() {
		if s.ID == id {
			return s, true
		}
	}

	return Step{}, false
}

// Steps returns every step in document order, each followed by its
// children: each procedure's steps and then its rollback, then each
// scenario's steps.
//
//nolint:gocritic // value receiver is the published surface, as impl.Doc's
func (d Doc) Steps() []Step {
	var out []Step

	for _, p := range d.Procedures {
		out = flatten(out, p.Steps)
		out = flatten(out, p.Rollback)
	}

	for _, s := range d.Scenarios {
		out = flatten(out, s.Steps)
	}

	return out
}

// flatten appends steps depth-first, each before its children.
func flatten(out, steps []Step) []Step {
	for _, s := range steps {
		out = append(out, s)
		out = flatten(out, s.Children)
	}

	return out
}
