package rfc

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// Parse interprets an RFC document. It never touches the filesystem.
//
// It fails for exactly two things: no frontmatter, and CR line endings.
// Everything else a document might be missing leaves its field zero and is
// reported by validate.Document against the document's schema — the division
// is deliberate, because a half-written proposal is the normal state of a
// proposal and a parser that refused one would be useless during the review
// it is written for. Unlike impl, this package has no content failure of its
// own: an RFC with no risks and no alternatives is an early draft, not a
// document with nothing of the type in it.
func Parse(doc []byte) (Doc, error) {
	if bytes.IndexByte(doc, '\r') >= 0 {
		return Doc{}, fmt.Errorf("rfc: %w", document.ErrUnsupportedLineEndings)
	}

	fm, err := document.ParseFrontmatter(doc)
	if err != nil {
		return Doc{}, fmt.Errorf("rfc: %w", err)
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

		// Every reader in docparse and kinds numbers lines from the start of
		// the bytes it was handed, which for a region is the region. A Doc's
		// lines are the document's, because a Line is an address a consumer
		// acts on — the line docwrite splices at, the line an editor jumps to
		// (DESIGN-0014 §5). kinds owns that conversion for its own value
		// types, so five type packages do not carry five chances to be off by
		// one; Risk is this package's own type and risks shifts it itself.
		switch r.Kind {
		case kindSummary:
			out.Summary = kinds.Body(body)
		case kindProblem:
			out.Problem = kinds.Body(body)
		case kindProposal:
			out.Proposal = kinds.Body(body)
		case kindAlternatives:
			out.Alternatives = kinds.ShiftAlternatives(kinds.Alternatives(body), r)
		case kindRisks:
			out.Risks = risks(body, r)
		case kindCriteria:
			out.Criteria = kinds.ShiftCriteria(kinds.Criteria(body), r)
		case kindOpenQuestions:
			out.OpenQuestions = kinds.ShiftQuestions(kinds.OpenQuestions(body), r)
		case kindReferences:
			out.References = kinds.ShiftReferences(kinds.References(body), r)
		}
	}

	return out, nil
}

// risks reads the first table in the risks region, mapping columns by
// position: Risk, Impact, Likelihood, Mitigation.
//
// By position rather than by name, per the shared field rules (DESIGN-0014
// §2.9). A row with fewer cells keeps the fields it has; a wholly empty row is
// the template's placeholder and is dropped, so a document nobody has filled
// in reports no risks rather than one blank one. A row that names a risk and
// nothing else is kept: that is rfc.risks.no-mitigation, which cannot be
// reported about a row the parser threw away.
func risks(region []byte, at docparse.Region) []Risk {
	tables := docparse.Tables(region)
	if len(tables) == 0 {
		return nil
	}

	out := make([]Risk, 0, len(tables[0].Rows))

	for i, row := range tables[0].Rows {
		risk := Risk{
			Risk: cell(row, 0), Impact: cell(row, 1),
			Likelihood: cell(row, 2), Mitigation: cell(row, 3),
			// The header, then the delimiter row, then the body.
			Line: at.Start + tables[0].Line + 2 + i,
		}

		if risk.Risk == "" && risk.Impact == "" &&
			risk.Likelihood == "" && risk.Mitigation == "" {
			continue
		}

		out = append(out, risk)
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
