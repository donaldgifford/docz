package confluence

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Comments is what an update did with the page's inline comments
// (DESIGN-0021 §4).
type Comments struct {
	// Kept counts the comments re-anchored on the new body.
	Kept int `json:"kept"`
	// Lost holds the text each comment that lost its anchor was made on.
	Lost []string `json:"lost,omitempty"`
}

// The storage namespaces, as the decoder reports them inside the root
// wellFormed and these readers wrap a body in.
const (
	nsAC = "http://atlassian.com/content"
	nsRI = "http://atlassian.com/resource/identifier"
)

// rootOpen declares the prefixes a storage body uses without declaring.
const rootOpen = `<root xmlns:ac="` + nsAC + `" xmlns:ri="` + nsRI + `">`

// marker is one inline comment's anchor: the comment's ref and the text it
// wraps, by text content.
type marker struct {
	ref  string
	text string
}

// newDecoder reads body inside the root, as wellFormed does.
func newDecoder(body []byte) *xml.Decoder {
	d := xml.NewDecoder(io.MultiReader(strings.NewReader(rootOpen), bytes.NewReader(body), strings.NewReader("</root>")))
	d.Strict = true
	d.Entity = xml.HTMLEntity

	return d
}

// collectMarkers reads a page's ac:inline-comment-marker elements in
// document order. A marker whose text spans other elements is read by its
// text content; one with no ref or no text is not a comment's anchor. A
// body that stops parsing yields the markers read before it stopped.
func collectMarkers(body []byte) []marker {
	var (
		out   []marker
		open  []*strings.Builder
		refs  []string
		depth []int
		level int
	)

	d := newDecoder(body)

	for {
		tok, err := d.Token()
		if err != nil {
			return out
		}

		switch t := tok.(type) {
		case xml.StartElement:
			level++

			if t.Name.Space == nsAC && t.Name.Local == "inline-comment-marker" {
				open = append(open, &strings.Builder{})
				refs = append(refs, attr(t, "ref"))
				depth = append(depth, level)
			}
		case xml.EndElement:
			if n := len(depth); n > 0 && depth[n-1] == level {
				if text := open[n-1].String(); refs[n-1] != "" && text != "" {
					out = append(out, marker{ref: refs[n-1], text: text})
				}

				open, refs, depth = open[:n-1], refs[:n-1], depth[:n-1]
			}

			level--
		case xml.CharData:
			for _, b := range open {
				b.Write(t)
			}
		}
	}
}

// attr returns the value of the attribute with the given local name.
func attr(t xml.StartElement, local string) string {
	for _, a := range t.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}

	return ""
}

// textNode is one run of character data a comment may anchor on: its
// decoded text, and for each decoded byte the offset in the body where it
// starts (with one more entry for the end).
type textNode struct {
	text string
	raw  []int
}

// anchorable lists the ac: elements whose character data a reader sees.
// Text under any other ac: or ri: element (a macro parameter, a task id, a
// mermaid diagram's attributes) is never an anchor.
var anchorable = map[string]bool{
	"structured-macro": true,
	"rich-text-body":   true,
	"task-list":        true,
	"task":             true,
	"task-body":        true,
	"layout":           true,
	"layout-section":   true,
	"layout-cell":      true,
}

// scope tracks whether the decoder is under an element whose text is
// never an anchor.
type scope struct {
	stack   []bool
	blocked int
}

func (s *scope) open(name xml.Name) {
	blocks := name.Space == nsRI || (name.Space == nsAC && !anchorable[name.Local])
	s.stack = append(s.stack, blocks)

	if blocks {
		s.blocked++
	}
}

func (s *scope) close() {
	n := len(s.stack)
	if n == 0 {
		return
	}

	if s.stack[n-1] {
		s.blocked--
	}

	s.stack = s.stack[:n-1]
}

// textNodes lists the character data of a rendered body a comment may
// anchor on, with byte offsets into body. CDATA sections never qualify.
func textNodes(body []byte) ([]textNode, error) {
	var (
		out []textNode
		sc  scope
	)

	d := newDecoder(body)
	prefix := len(rootOpen)

	for {
		start := int(d.InputOffset()) - prefix

		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return out, nil
		}

		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			sc.open(t.Name)
		case xml.EndElement:
			sc.close()
		case xml.CharData:
			end := int(d.InputOffset()) - prefix
			if n, ok := anchorText(body, start, end, sc.blocked > 0); ok {
				out = append(out, n)
			}
		}
	}
}

// anchorText is the text node for body[start:end], unless it is blocked,
// out of range, or a CDATA section.
func anchorText(body []byte, start, end int, blocked bool) (textNode, bool) {
	if blocked || start < 0 || end > len(body) || bytes.HasPrefix(body[start:end], []byte("<![CDATA[")) {
		return textNode{}, false
	}

	return decodeText(body[start:end], start)
}

// decodeText decodes raw character data starting at offset base, keeping
// where each decoded byte came from. ok is false for an entity it cannot
// read, which leaves the node out rather than misplacing a marker.
func decodeText(raw []byte, base int) (textNode, bool) {
	var (
		text strings.Builder
		offs = make([]int, 0, len(raw)+1)
	)

	for i := 0; i < len(raw); {
		if raw[i] != '&' {
			text.WriteByte(raw[i])
			offs = append(offs, base+i)
			i++

			continue
		}

		semi := bytes.IndexByte(raw[i:], ';')
		if semi < 0 {
			return textNode{}, false
		}

		value, ok := entity(string(raw[i+1 : i+semi]))
		if !ok {
			return textNode{}, false
		}

		for range len(value) {
			offs = append(offs, base+i)
		}

		text.WriteString(value)
		i += semi + 1
	}

	offs = append(offs, base+len(raw))

	return textNode{text: text.String(), raw: offs}, true
}

// entity decodes one entity's name: numeric, XML's five, or HTML's.
func entity(name string) (string, bool) {
	if num, ok := strings.CutPrefix(name, "#"); ok {
		base := 10
		if hex, isHex := strings.CutPrefix(num, "x"); isHex {
			num, base = hex, 16
		}

		r, err := strconv.ParseInt(num, base, 32)
		if err != nil || !utf8.ValidRune(rune(r)) {
			return "", false
		}

		return string(rune(r)), true
	}

	switch name {
	case "amp":
		return "&", true
	case "lt":
		return "<", true
	case "gt":
		return ">", true
	case "quot":
		return `"`, true
	case "apos":
		return "'", true
	}

	v, ok := xml.HTMLEntity[name]

	return v, ok
}

// span is a byte range of the body a marker wraps.
type span struct {
	start, end int
	ref        string
}

// carryMarkers wraps each marker's text in the rendered body when it
// occurs exactly once inside one anchorable text node, outside the spans
// placed before it, and reports the text of every marker it could not
// place. rendered is never modified; the output is rendered plus the
// marker elements and nothing else.
func carryMarkers(rendered []byte, markers []marker) (out []byte, kept int, lost []string) {
	if len(markers) == 0 {
		return rendered, 0, nil
	}

	nodes, err := textNodes(rendered)
	if err != nil {
		lost = make([]string, 0, len(markers))
		for _, m := range markers {
			lost = append(lost, m.text)
		}

		return rendered, 0, lost
	}

	var placed []span

	for _, m := range markers {
		if s, ok := place(nodes, m, placed); ok {
			placed = append(placed, s)
		} else {
			lost = append(lost, m.text)
		}
	}

	return splice(rendered, placed), len(placed), lost
}

// place finds the one occurrence of a marker's text outside placed spans.
func place(nodes []textNode, m marker, placed []span) (span, bool) {
	var (
		found span
		n     int
	)

	if m.text == "" || !utf8.ValidString(m.text) {
		return found, false
	}

	for i := range nodes {
		node := &nodes[i]

		for from := 0; ; {
			at := strings.Index(node.text[from:], m.text)
			if at < 0 {
				break
			}

			at += from
			s := span{start: node.raw[at], end: node.raw[at+len(m.text)], ref: m.ref}
			from = at + 1

			if overlaps(s, placed) || !onBoundary(node, at, len(m.text)) {
				continue
			}

			found = s
			n++
		}
	}

	return found, n == 1
}

// onBoundary reports whether the decoded range [at, at+n) starts and ends
// between characters and between entities, so a marker never splits
// either.
func onBoundary(node *textNode, at, n int) bool {
	end := at + n
	startOK := utf8.RuneStart(node.text[at]) && (at == 0 || node.raw[at] != node.raw[at-1])
	endOK := end == len(node.text) || (utf8.RuneStart(node.text[end]) && node.raw[end] != node.raw[end-1])

	return startOK && endOK
}

func overlaps(s span, placed []span) bool {
	for _, p := range placed {
		if s.start < p.end && p.start < s.end {
			return true
		}
	}

	return false
}

// splice inserts each span's marker element into body.
func splice(body []byte, spans []span) []byte {
	spans = slices.Clone(spans)
	slices.SortFunc(spans, func(a, b span) int { return a.start - b.start })

	var b bytes.Buffer

	last := 0

	for _, s := range spans {
		b.Write(body[last:s.start])
		b.WriteString(`<ac:inline-comment-marker ac:ref="`)
		xml.EscapeText(&b, []byte(s.ref)) //nolint:errcheck,gosec // a bytes.Buffer write cannot fail
		b.WriteString(`">`)
		b.Write(body[s.start:s.end])
		b.WriteString(`</ac:inline-comment-marker>`)
		last = s.end
	}

	b.Write(body[last:])

	return b.Bytes()
}
