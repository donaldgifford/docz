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
	// Kept counts the comments anchored on the new body, the ones carried
	// from the current body and the ones restored from the property alike.
	Kept int `json:"kept"`
	// Restored holds the text of each comment that had lost its anchor on an
	// earlier update and found it again on this one.
	Restored []string `json:"restored,omitempty"`
	// Lost holds the text of each comment with no anchor after this update,
	// whether it lost it now or on an earlier one. The anchor is remembered
	// in the page's property and tried again on every update until the page
	// no longer lists the comment.
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

// contextLen is how many bytes of text either side of a quote an anchor
// remembers, cut to a character boundary.
const contextLen = 32

// anchor is where one inline comment sits (INV-0021): the comment's ref,
// the text it wraps, up to contextLen bytes of the text before and after it
// in the same block, and the heading above it. The quote finds the place;
// the context tells two occurrences apart; the ref is what Confluence
// attaches the comment to, so writing it back under the same ref restores
// the comment with nothing done to the comment itself. An anchor is kept in
// the page's docz property between updates.
type anchor struct {
	Ref     string `json:"ref"`
	Quote   string `json:"quote"`
	Prefix  string `json:"prefix,omitempty"`
	Suffix  string `json:"suffix,omitempty"`
	Heading string `json:"heading,omitempty"`
}

// newDecoder reads body inside the root, as wellFormed does.
func newDecoder(body []byte) *xml.Decoder {
	d := xml.NewDecoder(io.MultiReader(strings.NewReader(rootOpen), bytes.NewReader(body), strings.NewReader("</root>")))
	d.Strict = true
	d.Entity = xml.HTMLEntity

	return d
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
// decoded text, for each decoded byte the offset in the body where it
// starts (with one more entry for the end), and where it sits in its
// block's text.
type textNode struct {
	text  string
	raw   []int
	block int
	off   int
}

// block is one run of text a reader sees as a unit, a paragraph, a heading,
// a list item, or a cell, flattened across the inline elements in it, with
// the heading above it.
type block struct {
	text    string
	heading string
}

// markerElem is the ac: element an inline comment anchors to.
const markerElem = "inline-comment-marker"

// anchorable lists the ac: elements whose character data a reader sees.
// Text under any other ac: or ri: element (a macro parameter, a task id, a
// mermaid diagram's attributes) is never an anchor. The marker itself is
// transparent, so the text it wraps reads as part of its block.
var anchorable = map[string]bool{
	"structured-macro": true,
	"rich-text-body":   true,
	"task-list":        true,
	"task":             true,
	"task-body":        true,
	"layout":           true,
	"layout-section":   true,
	"layout-cell":      true,
	markerElem:         true,
}

// inline lists the elements that do not end a block: the text either side
// of them is one run to a reader, so it is one run to an anchor's context.
var inline = map[string]bool{
	"a": true, "abbr": true, "b": true, "br": true, "code": true, "del": true, "em": true, "i": true,
	"ins": true, "kbd": true, "s": true, "small": true, "span": true, "strong": true, "sub": true,
	"sup": true, "time": true, "u": true,
}

// inlineAC lists the ac: elements that do not end a block.
var inlineAC = map[string]bool{markerElem: true, "link": true, "link-body": true, "plain-text-link-body": true}

// splitsBlock reports whether an element's start or end ends the block.
func splitsBlock(name xml.Name) bool {
	switch name.Space {
	case nsAC:
		return !inlineAC[name.Local]
	case nsRI:
		return false
	default:
		return !inline[name.Local]
	}
}

// isHeading reports whether an element is h1 to h6.
func isHeading(name xml.Name) bool {
	return name.Space == "" && len(name.Local) == 2 && name.Local[0] == 'h' && name.Local[1] >= '1' && name.Local[1] <= '6'
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

// markerSpan is a marker found in a walked body: its ref and where its
// text starts and ends in its block's text. A marker whose text crosses a
// block boundary ends where its first block does.
type markerSpan struct {
	ref        string
	block      int
	start, end int
}

// walked is what walkBody reads from a body.
type walked struct {
	nodes   []textNode
	blocks  []block
	markers []markerSpan
}

// walker is walkBody's state.
type walker struct {
	w       walked
	sc      scope
	cur     strings.Builder
	heading string
	// headed is the depth of the heading element the current block is in,
	// or zero.
	headed int
	depth  int
	open   []markerSpan
}

// flush ends the current block.
func (wk *walker) flush() {
	text := wk.cur.String()

	for i := range wk.open {
		if wk.open[i].end < 0 {
			wk.open[i].end = len(text)
			wk.w.markers = append(wk.w.markers, wk.open[i])
		}
	}

	wk.open = wk.open[:0]

	wk.w.blocks = append(wk.w.blocks, block{text: text, heading: wk.heading})

	if wk.headed > 0 && strings.TrimSpace(text) != "" {
		wk.heading = strings.TrimSpace(text)
	}

	wk.cur.Reset()
}

// walkBody reads a body's blocks, the text nodes a comment may anchor on, and
// the markers it carries, in document order. CDATA sections never qualify
// as text. A body that stops parsing is an error; the markers read before
// it stopped are returned with it.
func walkBody(body []byte) (walked, error) {
	wk := &walker{}
	d := newDecoder(body)
	prefix := len(rootOpen)

	for {
		start := int(d.InputOffset()) - prefix

		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			wk.flush()

			return wk.w, nil
		}

		if err != nil {
			wk.flush()

			return wk.w, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			wk.start(t)
		case xml.EndElement:
			wk.end(t)
		case xml.CharData:
			wk.text(body, start, int(d.InputOffset())-prefix)
		}
	}
}

func (wk *walker) start(t xml.StartElement) {
	wk.depth++

	if splitsBlock(t.Name) {
		wk.flush()
	}

	if isHeading(t.Name) && wk.headed == 0 {
		wk.headed = wk.depth
	}

	wk.sc.open(t.Name)

	if t.Name.Space == nsAC && t.Name.Local == markerElem {
		if ref := attr(t, "ref"); ref != "" {
			wk.open = append(wk.open, markerSpan{ref: ref, block: len(wk.w.blocks), start: wk.cur.Len(), end: -1})
		}
	}
}

func (wk *walker) end(t xml.EndElement) {
	if t.Name.Space == nsAC && t.Name.Local == markerElem {
		for i := len(wk.open) - 1; i >= 0; i-- {
			if wk.open[i].end < 0 {
				wk.open[i].end = wk.cur.Len()
				wk.w.markers = append(wk.w.markers, wk.open[i])
				wk.open = slices.Delete(wk.open, i, i+1)

				break
			}
		}
	}

	wk.sc.close()

	if splitsBlock(t.Name) {
		wk.flush()
	}

	if wk.headed == wk.depth {
		wk.headed = 0
	}

	wk.depth--
}

func (wk *walker) text(body []byte, start, end int) {
	n, ok := anchorText(body, start, end, wk.sc.blocked > 0)
	if !ok {
		return
	}

	n.block, n.off = len(wk.w.blocks), wk.cur.Len()
	wk.cur.WriteString(n.text)
	wk.w.nodes = append(wk.w.nodes, n)
}

// collectMarkers reads a page's inline comment anchors in document order.
// A marker whose text spans other elements is read by its text content;
// one with no ref or no text is not a comment's anchor. A body that stops
// parsing yields the anchors read before it stopped.
func collectMarkers(body []byte) []anchor {
	w, _ := walkBody(body) //nolint:errcheck // a body that stops parsing yields the markers read before it stopped

	var out []anchor

	for _, m := range w.markers {
		if m.block >= len(w.blocks) {
			continue
		}

		b := w.blocks[m.block]
		if m.start >= m.end || m.end > len(b.text) {
			continue
		}

		out = append(out, anchor{
			Ref:     m.ref,
			Quote:   b.text[m.start:m.end],
			Prefix:  tail(b.text[:m.start]),
			Suffix:  head(b.text[m.end:]),
			Heading: b.heading,
		})
	}

	return out
}

// tail is the last contextLen bytes of s, cut to a character boundary.
func tail(s string) string {
	if len(s) <= contextLen {
		return s
	}

	at := len(s) - contextLen
	for at < len(s) && !utf8.RuneStart(s[at]) {
		at++
	}

	return s[at:]
}

// head is the first contextLen bytes of s, cut to a character boundary.
func head(s string) string {
	if len(s) <= contextLen {
		return s
	}

	at := contextLen
	for at > 0 && !utf8.RuneStart(s[at]) {
		at--
	}

	return s[:at]
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

// carryMarkers wraps each anchor's quote in the rendered body and returns
// the anchors it placed and the ones it could not. A quote is placed where
// it occurs inside one anchorable text node, on character and entity
// boundaries, outside the spans placed before it: at its one occurrence,
// or, among several, at the one whose surrounding text and heading match
// the anchor's best, when one does. rendered is never modified; the output
// is rendered plus the marker elements and nothing else.
func carryMarkers(rendered []byte, anchors []anchor) (out []byte, placed, lost []anchor) {
	if len(anchors) == 0 {
		return rendered, nil, nil
	}

	w, err := walkBody(rendered)
	if err != nil {
		return rendered, nil, slices.Clone(anchors)
	}

	var spans []span

	for _, a := range anchors {
		if s, ok := place(&w, &a, spans); ok {
			spans = append(spans, s)
			placed = append(placed, a)
		} else {
			lost = append(lost, a)
		}
	}

	return splice(rendered, spans), placed, lost
}

// candidate is one place a quote could go.
type candidate struct {
	span  span
	score int
}

// place finds where an anchor's quote goes: its one occurrence, or the
// best-matching one of several.
func place(w *walked, a *anchor, placed []span) (span, bool) {
	if a.Quote == "" || !utf8.ValidString(a.Quote) {
		return span{}, false
	}

	var found []candidate

	for i := range w.nodes {
		node := &w.nodes[i]

		for from := 0; ; {
			at := strings.Index(node.text[from:], a.Quote)
			if at < 0 {
				break
			}

			at += from
			s := span{start: node.raw[at], end: node.raw[at+len(a.Quote)], ref: a.Ref}
			from = at + 1

			if overlaps(s, placed) || !onBoundary(node, at, len(a.Quote)) {
				continue
			}

			found = append(found, candidate{span: s, score: score(w, node, at, a)})
		}
	}

	return choose(found)
}

// choose picks the single candidate, or the one that scores strictly best.
func choose(found []candidate) (span, bool) {
	switch len(found) {
	case 0:
		return span{}, false
	case 1:
		return found[0].span, true
	}

	best, tie := found[0], false

	for _, c := range found[1:] {
		switch {
		case c.score > best.score:
			best, tie = c, false
		case c.score == best.score:
			tie = true
		}
	}

	if tie || best.score == 0 {
		return span{}, false
	}

	return best.span, true
}

// score is how well the text around an occurrence matches the anchor's
// context: the bytes of prefix and suffix that agree, plus a heading that
// matches, which outweighs any context of a different heading.
func score(w *walked, node *textNode, at int, a *anchor) int {
	if node.block >= len(w.blocks) {
		return 0
	}

	b := &w.blocks[node.block]
	pos := node.off + at
	n := commonSuffix(b.text[:pos], a.Prefix) + commonPrefix(b.text[pos+len(a.Quote):], a.Suffix)

	if a.Heading != "" && b.heading == a.Heading {
		n += 2 * contextLen
	}

	return n
}

// commonPrefix is the length of the longest prefix a and b share.
func commonPrefix(a, b string) int {
	n := min(len(a), len(b))
	i := 0

	for i < n && a[i] == b[i] {
		i++
	}

	return i
}

// commonSuffix is the length of the longest suffix a and b share.
func commonSuffix(a, b string) int {
	n := min(len(a), len(b))
	i := 0

	for i < n && a[len(a)-1-i] == b[len(b)-1-i] {
		i++
	}

	return i
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

// texts lists the anchors' quotes.
func texts(anchors []anchor) []string {
	if len(anchors) == 0 {
		return nil
	}

	out := make([]string, 0, len(anchors))
	for i := range anchors {
		out = append(out, anchors[i].Quote)
	}

	return out
}
