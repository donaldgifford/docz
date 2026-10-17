package mcp

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/donaldgifford/docz/v2/pkg/adr"
	"github.com/donaldgifford/docz/v2/pkg/design"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/impl"
	"github.com/donaldgifford/docz/v2/pkg/investigation"
	"github.com/donaldgifford/docz/v2/pkg/rfc"
	"github.com/donaldgifford/docz/v2/pkg/runbook"
)

// The fields.kind values (DESIGN-0023 §4): a built-in's canonical name, or
// generic for a custom type and for a document its package could not
// parse.
const (
	KindRFC           = "rfc"
	KindADR           = "adr"
	KindDesign        = "design"
	KindImpl          = "impl"
	KindInvestigation = "investigation"
	KindRunbook       = "runbook"
	KindGeneric       = "generic"
)

// The typed shapes: the type package's Doc with kind beside its fields.
// Each is its own struct so the Doc's fields are promoted as the package
// defines them and nothing here restates a type package's shape.
type (
	rfcFields struct {
		Kind string `json:"kind"`
		rfc.Doc
	}
	adrFields struct {
		Kind string `json:"kind"`
		adr.Doc
	}
	designFields struct {
		Kind string `json:"kind"`
		design.Doc
	}
	implFields struct {
		Kind string `json:"kind"`
		impl.Doc
	}
	investigationFields struct {
		Kind string `json:"kind"`
		investigation.Doc
	}
	runbookFields struct {
		Kind string `json:"kind"`
		runbook.Doc
	}
)

// GenericFields is what a custom type's document, or one a built-in's
// package rejected, parses to: every heading and every region it carries,
// with the parse error when there was one.
type GenericFields struct {
	Kind       string             `json:"kind"`
	ParseError string             `json:"parse_error,omitempty"`
	Headings   []docparse.Heading `json:"headings"`
	Regions    []docparse.Region  `json:"regions"`
	// Inferred is true when the document carries no markers and the
	// regions were read from its headings.
	Inferred bool `json:"inferred"`
}

// Fields is the typed-field switch (DESIGN-0023 §4): the six canonical
// type names to their packages' Parse, each marshalled under kind; any
// other name, or a parse failure, to GenericFields. It is the one place in
// the server that switches on a document's type name, for the reason
// cmd/validate.go carries the same switch: the core may not import a type
// package (DESIGN-0014 R2).
func Fields(typeName string, content []byte) any {
	switch typeName {
	case KindRFC:
		doc, err := rfc.Parse(content)
		if err != nil {
			return generic(content, rfc.Headings(), err)
		}

		return rfcFields{Kind: KindRFC, Doc: doc}
	case KindADR:
		doc, err := adr.Parse(content)
		if err != nil {
			return generic(content, adr.Headings(), err)
		}

		return adrFields{Kind: KindADR, Doc: doc}
	case KindDesign:
		doc, err := design.Parse(content)
		if err != nil {
			return generic(content, design.Headings(), err)
		}

		return designFields{Kind: KindDesign, Doc: doc}
	case KindImpl:
		doc, err := impl.Parse(content)
		if err != nil {
			return generic(content, impl.Headings(), err)
		}

		return implFields{Kind: KindImpl, Doc: doc}
	case KindInvestigation:
		doc, err := investigation.Parse(content)
		if err != nil {
			return generic(content, investigation.Headings(), err)
		}

		return investigationFields{Kind: KindInvestigation, Doc: doc}
	case KindRunbook:
		doc, err := runbook.Parse(content)
		if err != nil {
			return generic(content, runbook.Headings(), err)
		}

		return runbookFields{Kind: KindRunbook, Doc: doc}
	default:
		return generic(content, nil, nil)
	}
}

// generic reads the headings and the regions a document carries. With a
// spec, markers win and headings are inferred from when there are none;
// with none, only real markers are regions.
func generic(content []byte, spec kinds.HeadingSpec, parseErr error) GenericFields {
	regions, inferred := kinds.ResolveRegions(content, spec)
	out := GenericFields{
		Kind:     KindGeneric,
		Headings: docparse.Headings(content),
		Regions:  regions,
		Inferred: inferred,
	}
	if out.Headings == nil {
		out.Headings = []docparse.Heading{}
	}
	if out.Regions == nil {
		out.Regions = []docparse.Region{}
	}
	if parseErr != nil {
		out.ParseError = parseErr.Error()
	}

	return out
}

// progressOut is get_impl_progress's output (DESIGN-0023 §3, question 8):
// get_doc for an IMPL with the counts folded in and the markdown left out.
type progressOut struct {
	Repo          string          `json:"repo"`
	DocID         string          `json:"doc_id"`
	Title         string          `json:"title"`
	Status        string          `json:"status"`
	Objective     string          `json:"objective"`
	Done          int             `json:"done"`
	Total         int             `json:"total"`
	Phases        []progressPhase `json:"phases"`
	OpenQuestions []progressQ     `json:"open_questions"`
}

// progressPhase is one phase with its counts. Done and Total count as
// impl.Doc.Progress does, so a skipped task is in neither; Deferred counts
// the tasks marked deferred, which are in Total and not in Done.
type progressPhase struct {
	Index    int            `json:"index"`
	Token    string         `json:"token"`
	Title    string         `json:"title"`
	Done     int            `json:"done"`
	Total    int            `json:"total"`
	Deferred int            `json:"deferred"`
	Tasks    []progressTask `json:"tasks"`
}

// progressTask is one task as an agent asks about it.
type progressTask struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Checked  bool   `json:"checked"`
	Deferred bool   `json:"deferred"`
	Skipped  bool   `json:"skipped"`
	Verify   string `json:"verify,omitempty"`
	// Note is the deferred or skipped marker's note, when there is one.
	Note string `json:"note,omitempty"`
}

// progressQ is one open question and whether it has been resolved.
type progressQ struct {
	Number   int    `json:"number"`
	Text     string `json:"text"`
	Resolved bool   `json:"resolved"`
	// Choice is the letter the resolution chose, when resolved.
	Choice string `json:"choice,omitempty"`
}

// getImplProgress derives the counts an agent would otherwise compute
// from impl.Doc.
func (t *tools) getImplProgress(ctx context.Context, _ *sdk.CallToolRequest, in docIn) (*sdk.CallToolResult, progressOut, error) {
	repo, row, err := t.document(ctx, in.Repo, in.ID)
	if err != nil {
		return nil, progressOut{}, err
	}
	doc, err := impl.Parse([]byte(row.Document.RawMd))
	if err != nil {
		// An IMPL that does not parse as one has no phases to count; the
		// document itself is still readable through get_doc.
		return nil, progressOut{}, fmt.Errorf("%w: %w", errDocNotFound, err)
	}

	out := progressOut{
		Repo:          repo.Owner + "/" + repo.Name,
		DocID:         doc.ID,
		Title:         doc.Title,
		Status:        string(doc.Status),
		Objective:     doc.Objective,
		Phases:        make([]progressPhase, 0, len(doc.Phases)),
		OpenQuestions: make([]progressQ, 0, len(doc.OpenQuestions)),
	}
	out.Done, out.Total = doc.Progress()

	var text strings.Builder
	fmt.Fprintf(&text, "%s %s: %d/%d tasks done\n", doc.ID, doc.Title, out.Done, out.Total)
	for i := range doc.Phases {
		phase := progressOf(&doc.Phases[i])
		out.Phases = append(out.Phases, phase)
		fmt.Fprintf(&text, "Phase %s: %s %d/%d", phase.Token, phase.Title, phase.Done, phase.Total)
		if phase.Deferred > 0 {
			fmt.Fprintf(&text, " (%d deferred)", phase.Deferred)
		}
		text.WriteString("\n")
	}
	for i := range doc.OpenQuestions {
		q := &doc.OpenQuestions[i]
		pq := progressQ{Number: q.Number, Text: q.Title, Resolved: q.Resolved != nil}
		if q.Resolved != nil {
			pq.Choice = q.Resolved.Choice
		}
		out.OpenQuestions = append(out.OpenQuestions, pq)
	}
	if n := len(out.OpenQuestions); n > 0 {
		fmt.Fprintf(&text, "%s open questions\n", itoa(n))
	}

	return textResult(text.String()), out, nil
}

// progressOf counts one phase.
func progressOf(p *impl.Phase) progressPhase {
	out := progressPhase{
		Index: p.Index,
		Token: p.Token,
		Title: p.Title,
		Tasks: make([]progressTask, 0, len(p.Tasks)),
	}
	for i := range p.Tasks {
		task := &p.Tasks[i]
		pt := progressTask{
			ID:       task.ID,
			Text:     task.Text,
			Checked:  task.Checked,
			Deferred: task.Deferred != nil,
			Skipped:  task.Skipped != nil,
			Verify:   task.Verify,
		}
		switch {
		case task.Deferred != nil:
			pt.Note = task.Deferred.Note
		case task.Skipped != nil:
			pt.Note = task.Skipped.Note
		}
		out.Tasks = append(out.Tasks, pt)

		if task.Skipped != nil {
			continue
		}
		out.Total++
		if task.Checked {
			out.Done++
		}
		if task.Deferred != nil {
			out.Deferred++
		}
	}

	return out
}
