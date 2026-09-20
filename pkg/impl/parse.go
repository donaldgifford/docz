package impl

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// phaseHeading reads the token and title from a phase heading. The token
// stops at whitespace, a slash, or the colon, so "Phase 2B: Core" yields
// "2B" and "Phase 1 / 2: both" is not a phase.
var phaseHeading = regexp.MustCompile(`^Phase\s+([^\s/:]+):\s*(.*)$`)

// implementsID matches a document reference in the "**Implements:**" field:
// a prefix, a hyphen, and a number.
var implementsID = regexp.MustCompile(`\b([A-Za-z][A-Za-z0-9]*-\d+)\b`)

// Parse interprets an IMPL document. It never touches the filesystem.
//
// It fails for exactly three things: no frontmatter, CR line endings, and no
// phases. Everything else a document might be missing leaves its field zero
// and is reported by validate.Document against the document's schema — the
// division is deliberate, because a half-written plan is the normal state of
// a plan and a parser that refused one would be useless during the work it
// describes.
func Parse(doc []byte) (Doc, error) {
	if bytes.IndexByte(doc, '\r') >= 0 {
		return Doc{}, fmt.Errorf("impl: %w", document.ErrUnsupportedLineEndings)
	}

	fm, err := document.ParseFrontmatter(doc)
	if err != nil {
		return Doc{}, fmt.Errorf("impl: %w", err)
	}

	regions, inferred := kinds.ResolveRegions(doc, headings)

	out := Doc{
		ID:       fm.ID,
		Title:    fm.Title,
		Status:   fm.Status,
		Author:   fm.Author,
		Created:  fm.Created,
		Inferred: inferred,
	}

	for _, r := range regions {
		if !r.Closed {
			continue
		}

		body := kinds.RegionBytes(doc, r)

		switch r.Kind {
		case kindObjective:
			out.Objective = kinds.Body(body)
			out.Implements = implementsIn(body)
		case "in-scope":
			out.InScope = kinds.ShiftItems(kinds.Items(body), r)
		case "out-of-scope":
			out.OutOfScope = kinds.ShiftItems(kinds.Items(body), r)
		case "file-changes":
			out.FileChanges = fileChanges(body, r)
		case "testing":
			out.Testing = shiftTasks(docparse.TaskItems(body), r)
		case kindDependencies:
			out.Dependencies = kinds.Body(body)
		case "open-questions":
			out.OpenQuestions = kinds.ShiftQuestions(kinds.OpenQuestions(body), r)
		case kindDecisions:
			out.Decisions = kinds.ShiftDecisions(kinds.Decisions(body), r)
		case kindReferences:
			out.References = kinds.ShiftReferences(kinds.References(body), r)
		}
	}

	out.Phases, err = parsePhases(doc, regions)
	if err != nil {
		return Doc{}, err
	}

	return out, nil
}

// implementsIn reads the document IDs from the objective region's
// "**Implements:**" field.
//
// Several IDs on one line is normal — "IMPL-0016 / IMPL-0015" — so every ID
// in the field is reported rather than the first. The template's placeholder
// is an HTML comment, which kinds.Field strips, so an unfilled field yields
// nothing instead of "RFC-XXXX".
func implementsIn(region []byte) []string {
	value, ok := kinds.Field(region, "Implements")
	if !ok || value == "" {
		return nil
	}

	found := implementsID.FindAllStringSubmatch(value, -1)
	if len(found) == 0 {
		return nil
	}

	out := make([]string, 0, len(found))
	for _, m := range found {
		out = append(out, m[1])
	}

	return out
}

// fileChanges reads the first table in the file-changes region, mapping
// columns by position: File, Action, Description.
//
// By position rather than by name, per the shared field rules (DESIGN-0014
// §2.9). A row with fewer cells keeps the fields it has; a wholly empty row
// is the template's placeholder and is dropped, so a document nobody has
// filled in reports no changes rather than two blank ones.
func fileChanges(region []byte, at docparse.Region) []FileChange {
	tables := docparse.Tables(region)
	if len(tables) == 0 {
		return nil
	}

	out := make([]FileChange, 0, len(tables[0].Rows))

	for i, row := range tables[0].Rows {
		change := FileChange{
			File: cell(row, 0), Action: cell(row, 1), Description: cell(row, 2),
			// The header, then the delimiter row, then the body.
			Line: at.Start + tables[0].Line + 2 + i,
		}

		if change.File == "" && change.Description == "" {
			continue
		}

		out = append(out, change)
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

func cell(row []string, i int) string {
	if i >= len(row) {
		return ""
	}

	return strings.TrimSpace(row[i])
}

// Every reader in docparse and kinds numbers lines from the start of the bytes
// it was handed, which for a region is the region. A Doc's lines are the
// document's, because a Line is an address a consumer acts on — the line
// docwrite splices at, the line an editor jumps to (DESIGN-0014 §5).
//
// kinds owns that conversion for its own value types, so five type packages do
// not carry five chances to be off by one. shiftTasks is the one case it does
// not cover: docparse.TaskItem belongs to the facts layer, and only this
// package has a field of them.

// shiftTasks rebases task-item line numbers from a region onto the document.
func shiftTasks(items []docparse.TaskItem, at docparse.Region) []docparse.TaskItem {
	if len(items) == 0 {
		return nil
	}

	out := make([]docparse.TaskItem, 0, len(items))

	for _, item := range items {
		item.Line += at.Start
		out = append(out, item)
	}

	return out
}

// parsePhases reads the phase regions, each with its nested tasks and
// criteria.
func parsePhases(doc []byte, regions []docparse.Region) ([]Phase, error) {
	lines := strings.Split(strings.TrimSuffix(string(doc), "\n"), "\n")

	var out []Phase

	for i, r := range regions {
		if r.Kind != kindPhase || r.Depth != 0 || !r.Closed {
			continue
		}

		phase, ok := parsePhase(doc, lines, regions, i, len(out)+1)
		if !ok {
			continue
		}

		out = append(out, phase)
	}

	if len(out) == 0 {
		return nil, ErrNoPhases
	}

	if err := duplicateToken(out); err != nil {
		return nil, err
	}

	return out, nil
}

// duplicateToken reports the first token two phases both claim, comparing
// folded so "A" and "a" collide: they address the same tasks.
//
// In document order rather than by walking a map, so a document with two
// collisions reports the earlier one every run. A nondeterministic error
// message turns one broken document into an intermittent CI failure.
func duplicateToken(phases []Phase) error {
	for i, phase := range phases {
		at := []int{phase.Line}

		for _, other := range phases[i+1:] {
			if strings.EqualFold(other.Token, phase.Token) {
				at = append(at, other.Line)
			}
		}

		if len(at) > 1 {
			return &DuplicatePhaseError{Token: phase.Token, Lines: at}
		}
	}

	return nil
}

// parsePhase reads one phase region. A region whose first level-3 heading
// does not match "Phase <token>:" is not a phase and is skipped — validate
// reports it as impl.phase.no-heading, since a parser that invented a token
// for it would give its tasks an address nobody wrote.
func parsePhase(
	doc []byte, lines []string, regions []docparse.Region, i, index int,
) (Phase, bool) {
	r := regions[i]

	heading, ok := phaseHeadingIn(doc, r)
	if !ok {
		return Phase{}, false
	}

	m := phaseHeading.FindStringSubmatch(stripComments(heading.Text))
	if m == nil {
		return Phase{}, false
	}

	phase := Phase{
		Index: index,
		Token: strings.TrimSpace(m[1]),
		Title: strings.TrimSpace(m[2]),
		Line:  r.Start + heading.Line,
	}

	children := childrenOf(regions, i)

	phase.Description = descriptionBetween(lines, phase.Line, firstChildLine(children, r))

	for _, child := range children {
		body := kinds.RegionBytes(doc, child)

		switch child.Kind {
		case kindTasks:
			phase.Tasks = parseTasks(lines, child, phase.Token)
		case kindCriteria:
			phase.Criteria = kinds.ShiftCriteria(kinds.Criteria(body), child)
		}
	}

	return phase, true
}

// phaseHeadingIn returns the first level-3 heading inside a phase region.
func phaseHeadingIn(doc []byte, r docparse.Region) (docparse.Heading, bool) {
	for _, h := range docparse.Headings(kinds.RegionBytes(doc, r)) {
		if h.Level == 3 {
			return h, true
		}
	}

	return docparse.Heading{}, false
}

// childrenOf returns the regions nested one level inside the region at
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

// firstChildLine is where a phase's description stops: the first nested
// region, or the end of the phase when it has none.
func firstChildLine(children []docparse.Region, parent docparse.Region) int {
	if len(children) == 0 {
		return parent.End
	}

	return children[0].Start
}

// descriptionBetween returns the prose strictly between two lines, comments
// removed and trimmed. The template puts its guidance to the author in a
// comment, so an unfilled phase has an empty description rather than the
// instructions it shipped with.
func descriptionBetween(lines []string, after, before int) string {
	if after >= before || after < 1 {
		return ""
	}

	high := before - 1
	if high > len(lines) {
		high = len(lines)
	}

	if after >= high {
		return ""
	}

	return strings.TrimSpace(stripComments(strings.Join(lines[after:high], "\n")))
}

// stripComments removes HTML comments, including ones spanning lines, the
// way the kinds readers do. Duplicated rather than exported from kinds: it
// is three lines, and a helper shared across a layer boundary for its own
// sake is a dependency nobody asked for.
func stripComments(s string) string {
	var sb strings.Builder

	for {
		open := strings.Index(s, "<!--")
		if open < 0 {
			sb.WriteString(s)

			return sb.String()
		}

		sb.WriteString(s[:open])

		rest := s[open+4:]

		shut := strings.Index(rest, "-->")
		if shut < 0 {
			return sb.String()
		}

		s = rest[shut+3:]
	}
}
