package validate

import (
	"fmt"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// KindRule is what the catalogue knows about one region kind (DESIGN-0015
// §2): whether a document may hold more than one, and what its content has
// to look like.
//
// The catalogue is data, not an interface. Adding a kind is one map entry,
// which is the whole reason it grew from nine kinds to forty-one without a
// redesign when every built-in became a structured type.
type KindRule struct {
	// Singleton is true when a document may hold only one region of this
	// kind under a given parent. Scoping by parent is what lets an ADR have
	// exactly one positive list inside each consequences region while an
	// IMPL has one tasks region inside each of five phases.
	Singleton bool

	// Check reports the content findings for one region of this kind, or
	// nil when the kind has no content rule. A kind with no rule is still a
	// named span a package reads, which is why it is in the catalogue at
	// all.
	//
	// region is the region's bytes, heading included, as kinds.RegionBytes
	// cuts them. at is the span, for the line numbers a finding needs.
	Check func(region []byte, at docparse.Region) []Finding
}

// The kinds whose own content rules name them, so the catalogue key and the
// findings it produces cannot drift apart.
const (
	kindOpenQuestions = "open-questions"
	kindTasks         = "tasks"
)

// catalogue is the forty-one-kind catalogue of DESIGN-0015 §2.
//
// A kind absent from it is well-formedness only: validate checks that its
// markers pair and nothing more. That is deliberate — an unknown kind is
// allowed (DESIGN-0015 Open Question 8), so a repo can mark up something
// docz has no opinion about and still have the markers checked.
//
// Content findings use the content.* family. DESIGN-0015 §4 names codes for
// the families that are about a document as a whole, and §2 gives each kind
// a content rule without naming one; region.* stays what §4 defines it as,
// presence and structure, and the rule a kind failed is content.*, with the
// finding's Kind saying which kind it was.
var catalogue = map[string]KindRule{
	// Spans docz owns and splices. Their content is checked against a fresh
	// render by the document-level ToC and index rules, not per region.
	docparse.TocKind:   {Singleton: true},
	docparse.IndexKind: {Singleton: true},

	// Shared across every type.
	"references":      {Singleton: true, Check: checkReferences},
	kindOpenQuestions: {Singleton: true, Check: checkOpenQuestions},
	"decisions":       {Singleton: true},

	// rfc and adr.
	"summary":      {Singleton: true},
	"alternatives": {Singleton: true},

	// adr and investigation.
	"context": {Singleton: true},

	// rfc at the top level, impl inside a phase. One rule, both positions.
	"criteria": {Singleton: true, Check: checkCriteria},

	// design and impl.
	"testing": {Singleton: true},

	// rfc.
	"problem":  {Singleton: true},
	"proposal": {Singleton: true},
	"risks":    {Singleton: true, Check: checkRisksTable},

	// adr.
	"decision":     {Singleton: true},
	"consequences": {Singleton: true},
	"positive":     {Singleton: true, Check: checkItems},
	"negative":     {Singleton: true, Check: checkItems},
	"neutral":      {Singleton: true, Check: checkItems},

	// design.
	"overview":        {Singleton: true},
	"background":      {Singleton: true},
	"detailed-design": {Singleton: true},
	"api-changes":     {Singleton: true},
	"data-model":      {Singleton: true},
	"rollout":         {Singleton: true},
	"goals":           {Singleton: true, Check: checkItems},
	"non-goals":       {Singleton: true, Check: checkItems},

	// investigation.
	"question":       {Singleton: true},
	"hypothesis":     {Singleton: true},
	"recommendation": {Singleton: true},
	"approach":       {Singleton: true, Check: checkOrderedList},
	"environment":    {Singleton: true, Check: checkEnvironmentTable},
	"findings":       {Singleton: true},
	"conclusion":     {Singleton: true},

	// impl.
	"objective":    {Singleton: true},
	"scope":        {Singleton: true},
	"in-scope":     {Singleton: true, Check: checkItems},
	"out-of-scope": {Singleton: true, Check: checkItems},
	"phase":        {Singleton: false},
	kindTasks:      {Singleton: true, Check: checkTasks},
	"file-changes": {Singleton: true, Check: checkFileChangesTable},
	"dependencies": {Singleton: true},
}

// checkReferences reports a top-level bullet with no markdown link. The
// reader is kinds.References, which is why the finding names the same
// bullet the parsed value would hold.
func checkReferences(region []byte, at docparse.Region) []Finding {
	var out []Finding

	for _, ref := range kinds.References(region) {
		if ref.URL != "" {
			continue
		}

		out = append(out, Finding{
			Code:     "references.no-link",
			Severity: Warning,
			Line:     at.Start + ref.Line,
			Kind:     "references",
			Detail:   fmt.Sprintf("reference has no link: %s", truncate(ref.Text)),
		})
	}

	return out
}

// checkOpenQuestions reports questions numbered with a gap and questions
// with no options.
//
// Numbering is checked against position rather than against the previous
// number, so a document that numbers 1, 3, 4 reports one finding on the
// question that is wrong rather than one on every question after it.
func checkOpenQuestions(region []byte, at docparse.Region) []Finding {
	var out []Finding

	for i, q := range kinds.OpenQuestions(region) {
		if q.Number != i+1 {
			out = append(out, Finding{
				Code:     "open-questions.numbering",
				Severity: Warning,
				Line:     at.Start + q.Line,
				Kind:     kindOpenQuestions,
				Detail: fmt.Sprintf("question is numbered %d but is the %s",
					q.Number, ordinal(i+1)),
			})
		}

		if len(q.Options) == 0 {
			out = append(out, Finding{
				Code:     "open-questions.no-options",
				Severity: Warning,
				Line:     at.Start + q.Line,
				Kind:     kindOpenQuestions,
				Detail:   fmt.Sprintf("question %d has no lettered options", q.Number),
			})
		}
	}

	return out
}

// checkTasks reports a top-level bullet in a tasks region that is not a
// checkbox, and a nested checkbox.
//
// A nested checkbox is a warning rather than an error because the corpus
// has them and they read fine; what they are not is a task, so a consumer
// counting progress would be wrong to include them (INV-0010 Obs 3).
func checkTasks(region []byte, at docparse.Region) []Finding {
	var out []Finding

	checkboxes := make(map[int]bool)
	for _, task := range docparse.TaskItems(region) {
		checkboxes[task.Line] = true

		if task.Indent > 0 {
			out = append(out, Finding{
				Code:     "tasks.nested-checkbox",
				Severity: Warning,
				Line:     at.Start + task.Line,
				Kind:     kindTasks,
				Detail: fmt.Sprintf("nested checkbox is not a task: %s",
					truncate(task.Text)),
			})
		}
	}

	for _, item := range docparse.ListItems(region) {
		if item.Indent > 0 || checkboxes[item.Line] || item.Text == "" {
			continue
		}

		out = append(out, Finding{
			Code:     "tasks.not-checkbox",
			Severity: Error,
			Line:     at.Start + item.Line,
			Kind:     kindTasks,
			Detail:   fmt.Sprintf("task list item is not a checkbox: %s", truncate(item.Text)),
		})
	}

	return out
}

// The content rules below check that what a region holds is the right
// shape. They do not check that it holds anything.
//
// An empty section is incomplete, not malformed, and completeness belongs to
// the type tier: DESIGN-0015 §4 names rfc.alternatives.empty,
// adr.decision.empty, adr.consequences.empty, and design.goals.empty as
// findings the type packages emit. Reporting emptiness here too would make
// `docz validate` fail on the document `docz create` had just written, which
// is the behaviour the zero-findings golden exists to prevent.

// checkCriteria reports a criteria region written as something other than
// dash bullets. The kind is documented as dash bullets, so a section written
// as a numbered list is a procedure rather than a checklist anyone can tick
// off.
func checkCriteria(region []byte, at docparse.Region) []Finding {
	items := docparse.ListItems(region)
	if len(items) == 0 || len(kinds.Criteria(region)) > 0 {
		return nil
	}

	return []Finding{{
		Code:     "content.not-bullets",
		Severity: Warning,
		Line:     at.Start + items[0].Line,
		Kind:     "criteria",
		Detail:   "criteria are dash bullets, and this section has none",
	}}
}

// checkItems reports a region the catalogue says is a list of statements
// that holds prose instead.
func checkItems(region []byte, at docparse.Region) []Finding {
	if len(kinds.Items(region)) > 0 || len(docparse.ListItems(region)) > 0 {
		return nil
	}

	if kinds.Body(region) == "" {
		return nil
	}

	return []Finding{{
		Code:     "content.not-bullets",
		Severity: Warning,
		Line:     at.Start,
		Detail:   "section is prose where the kind is a list of bullets",
	}}
}

// checkOrderedList reports an investigation's approach written as a bullet
// list. The steps are a sequence someone else has to replicate, and a bullet
// list does not say what order to do them in.
func checkOrderedList(region []byte, at docparse.Region) []Finding {
	for _, item := range docparse.ListItems(region) {
		if item.Indent == 0 && !item.Ordered && item.Text != "" {
			return []Finding{{
				Code:     "content.not-ordered",
				Severity: Warning,
				Line:     at.Start + item.Line,
				Detail:   "steps are a sequence and should be a numbered list",
			}}
		}
	}

	return nil
}

func checkRisksTable(region []byte, at docparse.Region) []Finding {
	return checkTable(region, at, "risks", "risk", "mitigation")
}

func checkEnvironmentTable(region []byte, at docparse.Region) []Finding {
	return checkTable(region, at, "environment", "component")
}

func checkFileChangesTable(region []byte, at docparse.Region) []Finding {
	return checkTable(region, at, "file-changes", "file", "action", "description")
}

// checkTable reports a region the catalogue says holds a table that either
// has none or is missing a column the readers need.
//
// Only the first table is examined. A region with two is ambiguous about
// which one carries the data, and every reader in the module takes the
// first, so the check has to agree with them.
func checkTable(region []byte, at docparse.Region, kind string, columns ...string) []Finding {
	tables := docparse.Tables(region)
	if len(tables) == 0 {
		// Nothing there at all is incomplete, not malformed. Prose where a
		// table belongs is the shape finding.
		if kinds.Body(region) == "" {
			return nil
		}

		return []Finding{{
			Code:     "content.no-table",
			Severity: Warning,
			Line:     at.Start,
			Kind:     kind,
			Detail:   "section is prose where the kind is a table",
		}}
	}

	have := make(map[string]bool, len(tables[0].Header))
	for _, cell := range tables[0].Header {
		have[strings.ToLower(strings.TrimSpace(cell))] = true
	}

	var missing []string

	for _, want := range columns {
		if !have[want] {
			missing = append(missing, want)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	return []Finding{{
		Code:     "content.table-columns",
		Severity: Warning,
		Line:     at.Start + tables[0].Line,
		Kind:     kind,
		Detail: fmt.Sprintf("table is missing the %s column(s)",
			strings.Join(missing, ", ")),
	}}
}

// truncate keeps a quoted excerpt short enough to read in a terminal.
func truncate(s string) string {
	const limit = 60

	if len(s) <= limit {
		return s
	}

	return s[:limit] + "…"
}

// ordinal renders a small number as "first", "second", and so on, falling
// back to digits. A numbering finding reads better as "is the third" than
// "is at index 3".
func ordinal(n int) string {
	names := []string{
		"", "first", "second", "third", "fourth", "fifth",
		"sixth", "seventh", "eighth", "ninth", "tenth",
	}

	if n > 0 && n < len(names) {
		return names[n]
	}

	// Past ten, digits. Every number a numbered-question list reaches takes
	// "th" except 21, 31, and so on, and a document with thirty-one open
	// questions has a bigger problem than the suffix.
	return fmt.Sprintf("%dth item", n)
}
