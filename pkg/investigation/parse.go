package investigation

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// The two bold fields the template asks for, given to kinds.Field without
// punctuation: it accepts both spellings the fleet writes, the colon inside
// the bold and outside it.
const (
	fieldTriggeredBy = "Triggered by"
	fieldAnswer      = "Answer"
)

// Parse interprets an INV document. It never touches the filesystem.
//
// It fails for exactly two things: no frontmatter and CR line endings. There
// is no content failure — unlike impl, whose phases are addresses, nothing in
// an investigation makes the document unreadable by being absent. A document
// missing a region leaves that field zero and is reported by validate.Document
// against the document's schema, which is what lets an investigation be parsed
// while it is still being written: a question with no findings yet is the
// normal state of one for as long as the work takes.
func Parse(doc []byte) (Doc, error) {
	if bytes.IndexByte(doc, '\r') >= 0 {
		return Doc{}, fmt.Errorf("investigation: %w", document.ErrUnsupportedLineEndings)
	}

	fm, err := document.ParseFrontmatter(doc)
	if err != nil {
		return Doc{}, fmt.Errorf("investigation: %w", err)
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

		readRegion(&out, kinds.RegionBytes(doc, r), r)
	}

	return out, nil
}

// readRegion fills the field one region belongs to.
//
// Split out of Parse rather than written as a switch inside it: eleven kinds
// plus the loop around them is past the complexity a reader — or the linter —
// will take, and "which field does this kind fill" is a table anyway.
//
// The switch is on the kind and nothing else. No case reads the document's
// type name (ADR-0002 R7), which is what lets a custom type carrying these
// regions parse with this package.
func readRegion(out *Doc, body []byte, at docparse.Region) {
	switch at.Kind {
	case kindQuestion:
		out.Question = kinds.Body(body)
	case kindHypothesis:
		out.Hypothesis = kinds.Body(body)
	case kindContext:
		out.Context = kinds.Body(body)
		out.TriggeredBy, _ = kinds.Field(body, fieldTriggeredBy)
	case kindApproach:
		out.Approach = kinds.ShiftItems(kinds.Items(body), at)
	case kindEnvironment:
		out.Environment = environment(body, at)
	case kindFindings:
		out.Findings = kinds.ShiftSections(kinds.Sections(body), at)
	case kindConclusion:
		out.Conclusion = kinds.Body(body)
		out.Answer, _ = kinds.Field(body, fieldAnswer)
		out.Verdict = verdictOf(out.Answer)
	case kindRecommendation:
		out.Recommendation = kinds.Body(body)
	case kindOpenQuestions:
		out.OpenQuestions = kinds.ShiftQuestions(kinds.OpenQuestions(body), at)
	case kindDecisions:
		out.Decisions = kinds.ShiftDecisions(kinds.Decisions(body), at)
	case kindReferences:
		out.References = kinds.ShiftReferences(kinds.References(body), at)
	}
}

// environment reads the first table in the environment region, mapping
// columns by position: Component, Value.
//
// By position rather than by name, per the shared field rules (DESIGN-0014
// §2.9). A row with one cell keeps the field it has; a wholly empty row is
// the template's placeholder and is dropped, so a document nobody has filled
// in reports no environment rather than one blank component.
func environment(region []byte, at docparse.Region) []Component {
	tables := docparse.Tables(region)
	if len(tables) == 0 {
		return nil
	}

	out := make([]Component, 0, len(tables[0].Rows))

	for i, row := range tables[0].Rows {
		component := Component{
			Component: cell(row, 0), Value: cell(row, 1),
			// The header, then the delimiter row, then the body.
			Line: at.Start + tables[0].Line + 2 + i,
		}

		if component.Component == "" && component.Value == "" {
			continue
		}

		out = append(out, component)
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

// verdictOf reads the verdict from an answer's first word.
//
// Only the first word, because that is what the template asks for ("Yes / No
// / Inconclusive") and what the corpus writes before the sentence explaining
// itself. An answer that opens with anything else is VerdictUnknown, which is
// a finding rather than a failure: the author has answered the question, just
// not in a word a consumer can branch on.
func verdictOf(answer string) Verdict {
	switch strings.ToLower(firstWord(answer)) {
	case wordYes:
		return VerdictYes
	case wordNo:
		return VerdictNo
	case wordInconclusive:
		return VerdictInconclusive
	default:
		return VerdictUnknown
	}
}

// firstWord returns the leading run of letters in s, skipping anything before
// it that is not one.
//
// The decoration around the word is not part of it: the corpus emphasises the
// verdict ("**Inconclusive** — the benchmark never finished"), punctuates it
// ("no."), and sometimes runs an em dash straight into it. Trimming to letters
// reads all three as the word the author wrote.
func firstWord(s string) string {
	start := strings.IndexFunc(s, unicode.IsLetter)
	if start < 0 {
		return ""
	}

	rest := s[start:]

	end := strings.IndexFunc(rest, func(r rune) bool { return !unicode.IsLetter(r) })
	if end < 0 {
		return rest
	}

	return rest[:end]
}
