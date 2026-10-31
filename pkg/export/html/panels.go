package html

import (
	"bytes"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

// panel is one tab's rendered content.
type panel struct {
	Key, Label string
	// Anchor is the id the panel title carries: the anchor of the heading
	// the title stands in for, "" when it stands in for none.
	Anchor string
	Body   []byte
	// Contents lists the panel's headings at its shallowest level, when
	// there are more than four; the panel opens with them, standing in for
	// the ToC the page drops.
	Contents []docparse.Heading
}

// contentsMin is the most headings a panel shows without a list of them.
const contentsMin = 4

// panels renders each tab's sections. When a section's heading reads as
// the tab's label, the panel title stands in for it: the heading is not
// repeated and its anchor moves to the title. The footnote list, if any,
// closes the last panel.
func (d *doc) panels() ([]panel, error) {
	out := make([]panel, 0, len(d.tabs))

	skip := make(map[int]bool)
	if err := d.findSummary(skip); err != nil {
		return nil, err
	}

	for _, tb := range d.tabs {
		p := panel{Key: tb.Key, Label: tb.Label}

		var body bytes.Buffer

		for i, s := range tb.Sections {
			if i == 0 && s.Heading != nil && strings.EqualFold(strings.TrimSpace(stripComments(s.Heading.Text)), tb.Label) {
				skip[s.Heading.Line] = true
				p.Anchor = s.Heading.Slug
			}

			b, err := d.md.span(s.From, s.Through, skip)
			if err != nil {
				return nil, err
			}

			body.Write(b)
		}

		p.Body = body.Bytes()
		p.Contents = d.contents(tb, p.Anchor)
		out = append(out, p)
	}

	notes, err := d.md.footnotes()
	if err != nil {
		return nil, err
	}

	if len(notes) > 0 && len(out) > 0 {
		last := &out[len(out)-1]
		last.Body = append(last.Body, notes...)
	}

	return out, nil
}

// contents is the tab's headings at the shallowest level present, except
// the one the panel title stands in for, when there are more than
// contentsMin of them.
func (d *doc) contents(tb tab, title string) []docparse.Heading {
	var in []docparse.Heading

	for _, h := range d.headings {
		for _, s := range tb.Sections {
			if h.Line >= s.From && h.Line <= s.Through && (title == "" || h.Slug != title) {
				in = append(in, h)
			}
		}
	}

	if len(in) == 0 {
		return nil
	}

	top := in[0].Level
	for _, h := range in {
		top = min(top, h.Level)
	}

	var out []docparse.Heading

	for _, h := range in {
		if h.Level == top {
			h.Text = strings.TrimSpace(stripComments(h.Text))
			out = append(out, h)
		}
	}

	if len(out) <= contentsMin {
		return nil
	}

	return out
}

// summaryKinds are the regions whose lead paragraph can be the header
// summary, in no order: the first one in the document wins.
var summaryKinds = map[string]bool{
	"summary": true, "overview": true, "objective": true,
	"question": true, "hypothesis": true, "context": true,
}

// findSummary sets the header summary from the first summary-like region's
// lead paragraph and adds that paragraph's line to skip, so the panel does
// not repeat it. A region that opens with anything but a paragraph gives
// no summary.
func (d *doc) findSummary(skip map[int]bool) error {
	for _, tb := range d.tabs {
		for _, s := range tb.Sections {
			if !summaryKinds[s.Kind] {
				continue
			}

			para, line, err := d.md.leadParagraph(s.From, s.Through)
			if err != nil {
				return err
			}

			if line > 0 {
				d.summary = para
				skip[line] = true
			}

			return nil
		}
	}

	return nil
}
