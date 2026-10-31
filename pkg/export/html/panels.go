package html

import (
	"bytes"
	"strings"
)

// panel is one tab's rendered content.
type panel struct {
	Key, Label string
	// Anchor is the id the panel title carries: the anchor of the heading
	// the title stands in for, "" when it stands in for none.
	Anchor string
	Body   []byte
}

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
