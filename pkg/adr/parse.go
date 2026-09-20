package adr

import (
	"bytes"
	"fmt"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// Every reader in docparse and kinds numbers lines from the start of the bytes
// it was handed, which for a region is the region. A Doc's lines are the
// document's, because a Line is an address a consumer acts on — the line
// docwrite splices at, the line an editor jumps to (DESIGN-0014 §5).
//
// kinds owns that conversion for its own value types, which is why every
// reader below is wrapped in a Shift call and this package has no line
// arithmetic of its own. A nested region needs no special case: a region's
// Start is a document line at any depth.

// Parse interprets an ADR document. It never touches the filesystem.
//
// It fails for exactly two things: no frontmatter and CR line endings. A
// document missing a region — even its decision — leaves that field zero and
// is reported by validate.Document against its schema. The division is
// deliberate: an ADR is written to be argued over, so the half-written state
// is the normal one, and every rule that reads what is missing is a finding
// with a line on it rather than an error that hides the rest of the document.
func Parse(doc []byte) (Doc, error) {
	if bytes.IndexByte(doc, '\r') >= 0 {
		return Doc{}, fmt.Errorf("adr: %w", document.ErrUnsupportedLineEndings)
	}

	fm, err := document.ParseFrontmatter(doc)
	if err != nil {
		return Doc{}, fmt.Errorf("adr: %w", err)
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

	// The three consequence kinds arrive in this same flat slice at depth 1.
	// Selecting on kind alone is enough because a kind names one section of
	// the type: nothing an ADR carries means one thing at the top level and
	// another inside consequences, so there is nothing for the depth to
	// disambiguate.
	//
	// The consequences region itself has no case. It is a container for the
	// three lists and carries no field of its own, which is why an ADR whose
	// prose sits directly under "## Consequences" reports no consequences and
	// earns the adr.consequences.empty warning.
	for _, r := range regions {
		if !r.Closed {
			continue
		}

		body := kinds.RegionBytes(doc, r)

		switch r.Kind {
		case kindSummary:
			out.Summary = kinds.Body(body)
		case kindContext:
			out.Context = kinds.Body(body)
		case kindDecision:
			out.Decision = kinds.Body(body)
		case kindPositive:
			out.Consequences.Positive = kinds.ShiftItems(kinds.Items(body), r)
		case kindNegative:
			out.Consequences.Negative = kinds.ShiftItems(kinds.Items(body), r)
		case kindNeutral:
			out.Consequences.Neutral = kinds.ShiftItems(kinds.Items(body), r)
		case kindAlternatives:
			out.Alternatives = kinds.ShiftAlternatives(kinds.Alternatives(body), r)
		case kindOpenQuestions:
			out.OpenQuestions = kinds.ShiftQuestions(kinds.OpenQuestions(body), r)
		case kindReferences:
			out.References = kinds.ShiftReferences(kinds.References(body), r)
		}
	}

	return out, nil
}
