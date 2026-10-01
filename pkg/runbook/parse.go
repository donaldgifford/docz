package runbook

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

var (
	// procedureHeading reads the token and title from a procedure heading.
	// The token is optional, so "Procedure: Rotate" is a procedure that
	// takes its index; when present it stops at whitespace, a slash, or the
	// colon, so "Procedure 2B: Deploy" yields "2B".
	procedureHeading = regexp.MustCompile(`(?i)^Procedure(?:\s+([^\s/:]+))?\s*:\s*(.*)$`)

	// scenarioHeading reads the symptom from a scenario heading.
	scenarioHeading = regexp.MustCompile(`(?i)^Scenario\s*:\s*(.*)$`)
)

// The Last Verified table's columns, matched by folded header name rather
// than by position: the generic tier accepts them in any order, so the
// reader has to as well.
const (
	columnDate       = "date"
	columnPR         = "pr"
	columnCommit     = "commit"
	columnVerifiedBy = "verified by"
)

// Parse interprets a runbook. It never touches the filesystem.
//
// It fails for exactly two things, no frontmatter and CR line endings, plus
// two procedures claiming one token, which would make step IDs ambiguous.
// Everything else a document might be missing leaves its field zero for
// validate.Document to report against the schema: a runbook with only
// scenarios, or with nothing filled in yet, is still a runbook.
func Parse(doc []byte) (Doc, error) {
	if bytes.IndexByte(doc, '\r') >= 0 {
		return Doc{}, fmt.Errorf("runbook: %w", document.ErrUnsupportedLineEndings)
	}

	fm, err := document.ParseFrontmatter(doc)
	if err != nil {
		return Doc{}, fmt.Errorf("runbook: %w", err)
	}

	regions, inferred := kinds.ResolveRegions(doc, headings)
	lines := strings.Split(strings.TrimSuffix(string(doc), "\n"), "\n")

	out := Doc{
		ID:       fm.ID,
		Title:    fm.Title,
		Status:   fm.Status,
		Author:   fm.Author,
		Created:  fm.Created,
		Inferred: inferred,
	}

	for i := range regions {
		if regions[i].Closed && regions[i].Depth == 0 {
			out.read(doc, lines, regions, i)
		}
	}

	if err := duplicateToken(out.Procedures); err != nil {
		return Doc{}, err
	}

	return out, nil
}

// read fills the field one top-level region holds. It switches on the
// region's kind and never on the document's type name (R7).
func (d *Doc) read(doc []byte, lines []string, regions []docparse.Region, i int) {
	r := regions[i]
	body := kinds.RegionBytes(doc, r)

	switch r.Kind {
	case kindLastVerified:
		d.LastVerified = lastVerified(body, r)
	case kindOverview:
		d.Overview = overviewProse(body)
		d.Service = field(body, "Service")
		d.Owner = field(body, "Owner")
	case kindWhen:
		d.When = kinds.ShiftItems(kinds.Items(body), r)
	case kindPrerequisites:
		d.Prerequisites = kinds.ShiftItems(kinds.Items(body), r)
	case kindProcedure:
		if p, ok := parseProcedure(doc, lines, regions, i, len(d.Procedures)+1); ok {
			d.Procedures = append(d.Procedures, p)
		}
	case kindScenario:
		if s, ok := parseScenario(doc, lines, regions, i, len(d.Scenarios)+1); ok {
			d.Scenarios = append(d.Scenarios, s)
		}
	case kindEscalation:
		d.Escalation = contacts(body, r)
	case kindReferences:
		d.References = kinds.ShiftReferences(kinds.References(body), r)
	}
}

// field reads a bold-labelled field the way kinds.Field does, then folds
// in the lines that continue it, up to a blank line or the next bold label.
//
// A runbook's fields are sentences rather than the one-word values
// kinds.Field was written for ("**Answer:** Yes"), and a "**Likely cause:**"
// wrapped at eighty columns would otherwise be read as its first line.
func field(region []byte, label string) string {
	first, ok := kinds.Field(region, label)
	if !ok {
		return ""
	}

	forms := []string{"**" + label + ":**", "**" + label + "**:"}
	lines := strings.Split(string(region), "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, forms[0]) && !strings.HasPrefix(trimmed, forms[1]) {
			continue
		}

		parts := []string{first}

		for _, next := range lines[i+1:] {
			next = strings.TrimSpace(next)
			if next == "" || strings.HasPrefix(next, "**") || strings.HasPrefix(next, "|") ||
				strings.HasPrefix(next, "<!--docz:") {
				break
			}

			parts = append(parts, next)
		}

		return fold(parts)
	}

	return first
}

// overviewProse is the overview region's body without its field lines, which
// Doc carries separately.
func overviewProse(region []byte) string {
	body := kinds.Body(region)

	kept := make([]string, 0, strings.Count(body, "\n")+1)

	for line := range strings.SplitSeq(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "**Service") || strings.HasPrefix(trimmed, "**Owner") {
			continue
		}

		kept = append(kept, line)
	}

	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// lastVerified reads the first data row of the region's first table.
//
// Nil when there is no table, no data row, or a row whose every cell is
// empty, which is the template's own row: a verification is something
// somebody did, and an empty row records nobody doing anything.
func lastVerified(region []byte, at docparse.Region) *Verification {
	tables := docparse.Tables(region)
	if len(tables) == 0 || len(tables[0].Rows) == 0 {
		return nil
	}

	table := tables[0]
	row := table.Rows[0]

	if strings.TrimSpace(strings.Join(row, "")) == "" {
		return nil
	}

	cols := columns(table.Header)

	v := &Verification{
		Date:       cell(row, cols[columnDate]),
		PR:         cell(row, cols[columnPR]),
		Commit:     cell(row, cols[columnCommit]),
		VerifiedBy: splitNames(cell(row, cols[columnVerifiedBy])),
		// The header, then the delimiter row, then the body.
		Line: at.Start + table.Line + 2,
	}

	v.Notes = field(region, "Notes")

	return v
}

// columns maps each folded header name to its position. A header the table
// does not carry maps to -1, which cell reads as empty.
func columns(header []string) map[string]int {
	out := map[string]int{columnDate: -1, columnPR: -1, columnCommit: -1, columnVerifiedBy: -1}

	for i, name := range header {
		key := strings.ToLower(strings.TrimSpace(name))
		if _, ok := out[key]; ok {
			out[key] = i
		}
	}

	return out
}

// splitNames splits a "Verified by" cell on commas, dropping empty names.
func splitNames(s string) []string {
	var out []string

	for name := range strings.SplitSeq(s, ",") {
		if name = strings.TrimSpace(name); name != "" {
			out = append(out, name)
		}
	}

	return out
}

// contacts reads the escalation table, mapping columns by position: Who,
// When, How. A wholly empty row is the template's placeholder and is dropped.
func contacts(region []byte, at docparse.Region) []Contact {
	tables := docparse.Tables(region)
	if len(tables) == 0 {
		return nil
	}

	var out []Contact

	for i, row := range tables[0].Rows {
		c := Contact{
			Who: cell(row, 0), When: cell(row, 1), How: cell(row, 2),
			Line: at.Start + tables[0].Line + 2 + i,
		}

		if c.Who == "" && c.When == "" && c.How == "" {
			continue
		}

		out = append(out, c)
	}

	return out
}

func cell(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}

	return strings.TrimSpace(row[i])
}

// parseProcedure reads one procedure region. A region whose first level-3
// heading does not read "Procedure …:" is not a procedure and is skipped,
// since a parser that invented a token for it would give its steps an
// address nobody wrote.
func parseProcedure(
	doc []byte, lines []string, regions []docparse.Region, i, index int,
) (Procedure, bool) {
	r := regions[i]

	heading, ok := level3HeadingIn(doc, r)
	if !ok {
		return Procedure{}, false
	}

	m := procedureHeading.FindStringSubmatch(strings.TrimSpace(stripComments(heading.Text)))
	if m == nil {
		return Procedure{}, false
	}

	p := Procedure{
		Index: index,
		Token: strings.TrimSpace(m[1]),
		Title: strings.TrimSpace(m[2]),
		Line:  r.Start + heading.Line,
	}

	if p.Token == "" {
		p.Token = strconv.Itoa(index)
	}

	children := childrenOf(regions, i)
	p.Description = descriptionBetween(lines, p.Line, firstChildLine(children, r))

	for _, child := range children {
		switch child.Kind {
		case kindSteps:
			if p.Steps == nil {
				p.Steps = parseSteps(lines, child, p.Token+".")
			}
		case kindVerification:
			p.Verification = kinds.ShiftItems(kinds.Items(kinds.RegionBytes(doc, child)), child)
		case kindRollback:
			p.Rollback = parseSteps(lines, child, p.Token+".R")
		}
	}

	return p, true
}

// parseScenario reads one scenario region. Its heading is read the same way
// a procedure's is, and a region without one is skipped.
func parseScenario(
	doc []byte, lines []string, regions []docparse.Region, i, index int,
) (Scenario, bool) {
	r := regions[i]

	heading, ok := level3HeadingIn(doc, r)
	if !ok {
		return Scenario{}, false
	}

	m := scenarioHeading.FindStringSubmatch(strings.TrimSpace(stripComments(heading.Text)))
	if m == nil {
		return Scenario{}, false
	}

	s := Scenario{
		Index:   index,
		Symptom: strings.TrimSpace(m[1]),
		Line:    r.Start + heading.Line,
	}

	children := childrenOf(regions, i)

	// The fields are read from the prose above the steps only, so a step
	// that happens to say "**Alert:**" does not become the scenario's alert.
	fields := []byte(strings.Join(
		linesBetween(lines, s.Line, firstChildLine(children, r)), "\n"))
	s.Alert = field(fields, "Alert")
	s.LikelyCause = field(fields, "Likely cause")

	for _, child := range children {
		if child.Kind == kindSteps && s.Steps == nil {
			s.Steps = parseSteps(lines, child, "S"+strconv.Itoa(index)+".")
		}
	}

	return s, true
}

// duplicateToken reports the first token two procedures both claim, folded
// so "A" and "a" collide, in document order so the error is the same every
// run.
func duplicateToken(procedures []Procedure) error {
	for i := range procedures {
		token := procedures[i].Token
		at := []int{procedures[i].Line}

		for j := i + 1; j < len(procedures); j++ {
			if strings.EqualFold(procedures[j].Token, token) {
				at = append(at, procedures[j].Line)
			}
		}

		if len(at) > 1 {
			return &DuplicateProcedureError{Token: token, Lines: at}
		}
	}

	return nil
}

// level3HeadingIn returns the first level-3 heading inside a region.
func level3HeadingIn(doc []byte, r docparse.Region) (docparse.Heading, bool) {
	for _, h := range docparse.Headings(kinds.RegionBytes(doc, r)) {
		if h.Level == 3 {
			return h, true
		}
	}

	return docparse.Heading{}, false
}

// childrenOf returns the closed regions nested one level inside the region at
// index i.
func childrenOf(regions []docparse.Region, i int) []docparse.Region {
	parent := regions[i]

	var out []docparse.Region

	for _, r := range regions[i+1:] {
		if r.Start > parent.End {
			break
		}

		if r.Depth == parent.Depth+1 && r.Closed {
			out = append(out, r)
		}
	}

	return out
}

// firstChildLine is where a region's own prose stops: its first nested
// region, or its end when it has none.
func firstChildLine(children []docparse.Region, parent docparse.Region) int {
	if len(children) == 0 {
		return parent.End
	}

	return children[0].Start
}

// linesBetween returns the document lines strictly between two 1-based
// lines.
func linesBetween(lines []string, after, before int) []string {
	high := min(before-1, len(lines))
	if after < 1 || after >= high {
		return nil
	}

	return lines[after:high]
}

// descriptionBetween returns the prose strictly between two lines, comments
// removed and trimmed, so an unfilled procedure has an empty description
// rather than the template's guidance.
func descriptionBetween(lines []string, after, before int) string {
	return strings.TrimSpace(stripComments(strings.Join(linesBetween(lines, after, before), "\n")))
}
