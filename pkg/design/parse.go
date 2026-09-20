package design

import (
	"bytes"
	"fmt"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// Parse interprets a DESIGN document. It never touches the filesystem.
//
// It fails for exactly two things: no frontmatter, and a carriage return
// anywhere in the input. Nothing about the content can make it fail — a
// document missing a section leaves that field zero and is reported by
// validate.Document against the document's schema. The division is
// deliberate, because a half-written design is the normal state of a design
// and a parser that refused one would be useless during the thinking it
// records.
func Parse(doc []byte) (Doc, error) {
	if bytes.IndexByte(doc, '\r') >= 0 {
		return Doc{}, fmt.Errorf("design: %w", document.ErrUnsupportedLineEndings)
	}

	fm, err := document.ParseFrontmatter(doc)
	if err != nil {
		return Doc{}, fmt.Errorf("design: %w", err)
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

		readRegion(&out, r, kinds.RegionBytes(doc, r))
	}

	return out, nil
}

// readRegion assigns one region to the field that holds it.
//
// Selected on the region's kind and on nothing else, which is rule R7: a
// custom type whose documents carry a goals region is read here exactly as a
// DESIGN is, because the type name never enters the decision.
//
// Split out of Parse only so that twelve cases and a loop are not one
// function's worth of branching; the two read as one thing.
//
// Every Line the kinds readers report is counted from the start of the region
// they were handed, and every Line a Doc carries is the document's, because a
// Line is an address a consumer acts on — the line an editor jumps to
// (DESIGN-0014 §5). The kinds.Shift helpers are that conversion, which is why
// there is one on every slice field and none on a string.
func readRegion(out *Doc, r docparse.Region, body []byte) {
	switch r.Kind {
	case kindOverview:
		out.Overview = kinds.Body(body)
	case kindGoals:
		out.Goals = kinds.ShiftItems(kinds.Items(body), r)
	case kindNonGoals:
		out.NonGoals = kinds.ShiftItems(kinds.Items(body), r)
	case kindBackground:
		out.Background = kinds.Body(body)
	case kindDetailedDesign:
		out.DetailedDesign = kinds.Body(body)
	case kindAPIChanges:
		out.APIChanges = kinds.Body(body)
	case kindDataModel:
		out.DataModel = kinds.Body(body)
	case kindTesting:
		out.Testing = kinds.Body(body)
	case kindRollout:
		out.Rollout = kinds.Body(body)
	case kindOpenQuestions:
		out.OpenQuestions = kinds.ShiftQuestions(kinds.OpenQuestions(body), r)
	case kindDecisions:
		out.Decisions = kinds.ShiftDecisions(kinds.Decisions(body), r)
	case kindReferences:
		out.References = kinds.ShiftReferences(kinds.References(body), r)
	}
}
