// Package validate reports what is wrong with a docz document.
//
// The generic checks live here: marker well-formedness, the regions a
// schema requires, frontmatter, the content rules of the kind catalogue,
// ToC freshness, and the file-level rules (DESIGN-0015 §4). Type-specific
// checks live in the type packages, return this package's Finding, and see
// a document through their own Parse — three tiers composed by the caller,
// never dispatched by a registry (ADR-0002 Decision 4).
//
// Document never fails. A document with no frontmatter is a document with a
// finding, not an error return: a validator that refused to look at a
// broken document would be useless on exactly the documents that need it.
//
// The content rules call the sibling kinds package's readers, so a finding
// and a parsed value can never disagree about what a region says.
package validate

import (
	"fmt"
	"sort"
)

// Severity separates what an author must fix from what they should look at.
type Severity int

// The two severities. A consumer gates on Error and reports Warning.
const (
	// Error means the document is wrong in a way that costs a reader or a
	// tool something concrete: a region that cannot be found, a status no
	// configuration allows, bytes docz will not write back.
	Error Severity = iota + 1

	// Warning means the document is readable but not what it claims to be:
	// a reference with no link, questions numbered with a gap, a stale ToC.
	Warning
)

// String renders a severity for a message.
func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	default:
		return fmt.Sprintf("Severity(%d)", int(s))
	}
}

// Finding is one thing wrong with a document.
type Finding struct {
	// Code is the stable identifier and the only part of a finding that is
	// the contract: "region.unclosed", "frontmatter.status" (DESIGN-0015
	// Open Question 3). A consumer switches on Code and may replace Detail
	// with its own wording.
	Code string

	// Severity is Error or Warning.
	Severity Severity

	// Line is 1-based, or 0 when the finding concerns the whole document.
	Line int

	// Kind is the region kind the finding concerns, or "" when none does.
	Kind string

	// Detail is the default human-readable text. It is a default, not the
	// contract: a consumer that wants its own phrasing switches on Code.
	Detail string
}

// String renders a finding the way a CLI would print one line of it.
func (f Finding) String() string {
	if f.Line > 0 {
		return fmt.Sprintf("%d: %s: %s: %s", f.Line, f.Severity, f.Code, f.Detail)
	}

	return fmt.Sprintf("%s: %s: %s", f.Severity, f.Code, f.Detail)
}

// sortFindings puts findings in the order a reader walks a document: by
// line, then by code so a line with several findings reads the same way
// every run.
//
// Document-level findings (line 0) come first: they are about the document
// as a whole, so they belong above the line-by-line list rather than
// buried where a reader would take them for a finding about line 1.
func sortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}

		return findings[i].Code < findings[j].Code
	})
}
