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

	for _, tb := range d.tabs {
		p := panel{Key: tb.Key, Label: tb.Label}

		var body bytes.Buffer

		for i, s := range tb.Sections {
			skip := 0
			if i == 0 && s.Heading != nil && strings.EqualFold(strings.TrimSpace(stripComments(s.Heading.Text)), tb.Label) {
				skip = s.Heading.Line
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
