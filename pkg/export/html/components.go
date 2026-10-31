package html

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// componentOf is the component a section is laid out with: its kind's
// row's, or text for a kind the table does not list.
func componentOf(s section) component {
	if r, ok := rowFor(s.Kind); ok {
		return r.Component
	}

	return compText
}

// sectionWriter lays a section out with one component, and reports false
// when it found nothing it could lay out.
type sectionWriter func(d *doc, w *bytes.Buffer, s section) (bool, error)

// sectionWriters maps each component but text and goals to its writer.
var sectionWriters = map[component]sectionWriter{
	compScope: func(d *doc, w *bytes.Buffer, s section) (bool, error) {
		return d.childCards(w, s, "g2", map[string]cardStyle{
			"in-scope": {"ok", "In scope"}, "out-of-scope": {"bad", "Out of scope"},
		})
	},
	compConsequences: func(d *doc, w *bytes.Buffer, s section) (bool, error) {
		if d.adrDoc() == nil {
			return false, nil
		}

		return d.childCards(w, s, "g3", map[string]cardStyle{
			"positive": {"ok", "Positive"}, "negative": {"bad", "Negative"}, "neutral": {"neutral", "Neutral"},
		})
	},
	compLastVerified: func(d *doc, w *bytes.Buffer, s section) (bool, error) {
		return d.labelledCard(w, s, "card", "Last verified")
	},
	compChecklist:    (*doc).checklist,
	compDecision:     (*doc).decision,
	compAlternatives: (*doc).alternatives,
	compRisks:        (*doc).risks,
	compCriteria:     (*doc).criteria,
	compPhases:       (*doc).phase,
	compFindings:     (*doc).findings,
	compVerdict:      (*doc).verdict,
	compProcedures:   (*doc).procedure,
	compScenarios:    (*doc).scenario,
	compQuestions:    (*doc).questions,
}

// writeSection lays one section out with its component. A component whose
// typed reading failed, or that finds nothing it can lay out, writes the
// section as text, so nothing in the document is lost.
func (d *doc) writeSection(w *bytes.Buffer, s section, comp component) error {
	if err := d.text(w, s.From, s.Body-1); err != nil {
		return err
	}

	var (
		done bool
		err  error
	)

	if write, ok := sectionWriters[comp]; ok {
		done, err = write(d, w, s)
	}

	if err != nil || done {
		return err
	}

	return d.text(w, s.Body, s.Through)
}

// text renders lines from through through as markdown.
func (d *doc) text(w *bytes.Buffer, from, through int) error {
	if from > through {
		return nil
	}

	return d.md.spanTo(w, from, through, d.skip, nil)
}

// textWith is text with the nodes on the lines in replace written by their
// replacers.
func (d *doc) textWith(w *bytes.Buffer, from, through int, replace map[int]replacer) error {
	if from > through {
		return nil
	}

	return d.md.spanTo(w, from, through, d.skip, replace)
}

// children are the regions directly inside a depth-0 section's region.
func (d *doc) children(s section) []docparse.Region {
	var out []docparse.Region

	for _, r := range d.regions {
		if r.Depth == 1 && r.Start >= s.Body && r.End <= s.Through {
			out = append(out, r)
		}
	}

	return out
}

// childrenOf are the regions of kind directly inside r.
func (d *doc) childrenOf(r docparse.Region, kind string) []docparse.Region {
	var out []docparse.Region

	for _, c := range d.regions {
		if c.Depth == r.Depth+1 && c.Start > r.Start && c.End < r.End && c.Kind == kind {
			out = append(out, c)
		}
	}

	return out
}

// region is the section's own depth-0 region.
func (d *doc) region(s section) (docparse.Region, bool) {
	for _, r := range d.regions {
		if r.Depth == 0 && r.Start+1 == s.Body {
			return r, true
		}
	}

	return docparse.Region{}, false
}

// headingIn is the first heading on lines from through through.
func (d *doc) headingIn(from, through int) *docparse.Heading {
	return firstHeading(d.headings, from, through)
}

// headingText renders a heading's inline content, its id left off: the
// caller puts the anchor on the element that stands for the heading.
func (d *doc) headingText(h *docparse.Heading) ([]byte, error) {
	n := d.md.topAt(h.Line)
	if n == nil {
		return []byte(html.EscapeString(strings.TrimSpace(stripComments(h.Text)))), nil
	}

	var out bytes.Buffer
	if err := d.md.children(&out, n); err != nil {
		return nil, err
	}

	d.md.nodes.closeOpen(&out)

	return bytes.TrimSpace(out.Bytes()), nil
}

// idAttr is ` id="<slug>"`, or "" for no heading.
func idAttr(h *docparse.Heading) string {
	if h == nil || h.Slug == "" {
		return ""
	}

	return ` id="` + html.EscapeString(h.Slug) + `"`
}

// cardStyle is a child region's card: its variant and the label it takes
// when it has no heading.
type cardStyle struct {
	Class, Label string
}

// card writes r as a card: the heading's text as the label and its anchor
// on the card, then the rest of the region.
func (d *doc) card(w *bytes.Buffer, class, fallback string, from, through int) error {
	h := d.headingIn(from, through)

	label := []byte(html.EscapeString(fallback))
	body := from

	if h != nil && h.Line == d.firstContentLine(from, through) {
		t, err := d.headingText(h)
		if err != nil {
			return err
		}

		if len(t) > 0 {
			label = t
		}

		body = h.Line + 1
	} else {
		h = nil
	}

	put(w, `<div class="`+class+`"`+idAttr(h)+">\n")
	put(w, `<p class="card-label">`)
	w.Write(label)
	put(w, "</p>\n")

	if err := d.text(w, body, through); err != nil {
		return err
	}

	put(w, "</div>\n")

	return nil
}

// firstContentLine is the first line from through through holding
// anything but blank space or a comment, 0 when none does.
func (d *doc) firstContentLine(from, through int) int {
	for n := from; n <= through && n <= len(d.lines); n++ {
		t := strings.TrimSpace(d.lines[n-1])
		if t == "" || strings.HasPrefix(t, "<!--") && strings.HasSuffix(t, "-->") {
			continue
		}

		return n
	}

	return 0
}

// childCards writes a section whose region holds child regions as a grid
// of cards, one per child of a kind in styles: the text before the first
// child, the grid, then the text after the last.
func (d *doc) childCards(w *bytes.Buffer, s section, grid string, styles map[string]cardStyle) (bool, error) {
	var cards []docparse.Region

	for _, c := range d.children(s) {
		if _, ok := styles[c.Kind]; ok {
			cards = append(cards, c)
		}
	}

	if len(cards) == 0 {
		return false, nil
	}

	if err := d.text(w, s.Body, cards[0].Start-1); err != nil {
		return false, err
	}

	put(w, `<div class="grid `+grid+`">`+"\n")

	for _, c := range cards {
		st := styles[c.Kind]
		if err := d.card(w, "card "+st.Class, st.Label, c.Start+1, c.End-1); err != nil {
			return false, err
		}
	}

	put(w, "</div>\n")

	return true, d.text(w, cards[len(cards)-1].End+1, s.Through)
}

// goals writes a run of goals and non-goals sections as one grid: an ok
// card for goals, a neutral one for non-goals.
func (d *doc) goals(w *bytes.Buffer, run []section) error {
	for _, s := range run {
		if err := d.text(w, s.From, s.Body-1); err != nil {
			return err
		}
	}

	put(w, `<div class="grid g2">`+"\n")

	for _, s := range run {
		class, label := "card ok", "Goals"
		if s.Kind == "non-goals" {
			class, label = "card neutral", "Non-goals"
		}

		if err := d.card(w, class, label, s.Body, s.Through); err != nil {
			return err
		}
	}

	put(w, "</div>\n")

	return nil
}

// labelledCard writes the region as one card labelled by its heading, or
// by label when it has none.
func (d *doc) labelledCard(w *bytes.Buffer, s section, class, label string) (bool, error) {
	return true, d.card(w, class, label, s.Body, s.Through)
}

// firstBlock is the line of the first top-level node after the region's
// heading, 0 when there is none.
func (d *doc) firstBlock(s section) int {
	h := d.headingIn(s.Body, s.Through)

	for n := d.md.root.FirstChild(); n != nil; n = n.NextSibling() {
		line := d.md.startLine(n)
		if line < s.Body || line > s.Through || h != nil && line <= h.Line || d.skip[line] {
			continue
		}

		if hb, ok := n.(*ast.HTMLBlock); ok && isComment(strings.TrimSpace(string(blockLines(hb, d.md.src)))) {
			continue
		}

		return line
	}

	return 0
}

// decision writes an ADR's decision: its first block in a callout, then
// the rest as text.
func (d *doc) decision(w *bytes.Buffer, s section) (bool, error) {
	first := d.firstBlock(s)
	if first == 0 {
		return false, nil
	}

	return true, d.textWith(w, s.Body, s.Through, map[int]replacer{
		first: func(w *bytes.Buffer, n ast.Node) error {
			put(w, `<div class="callout">`+"\n")

			if err := d.md.render(w, n); err != nil {
				return err
			}

			put(w, "</div>\n")

			return nil
		},
	})
}

// chosen reads an alternative as the one picked: "(chosen)" or "(selected)"
// in its title, or a text that opens by saying so.
var chosen = regexp.MustCompile(`(?i)\((chosen|selected)\b|^\W*(chosen|selected)\b`)

// alternatives writes one card per alternative kinds.Alternatives finds:
// a level-3 section's heading and body, or a bold-led bullet's name and
// text. The chosen one is an accent card.
func (d *doc) alternatives(w *bytes.Buffer, s section) (bool, error) {
	r, ok := d.region(s)
	if !ok {
		return false, nil
	}

	alts := kinds.ShiftAlternatives(kinds.Alternatives(kinds.RegionBytes(d.body, r)), r)
	if len(alts) == 0 {
		return false, nil
	}

	class := func(a kinds.Alternative) string {
		if chosen.MatchString(a.Title) || chosen.MatchString(a.Text) {
			return "card accent"
		}

		return "card"
	}

	if d.headingAt(alts[0].Line) != nil {
		return d.altSections(w, s, alts, class)
	}

	return d.altList(w, s, alts, class)
}

// altSections writes alternatives that are level-3 sections as one card
// each, after the prose above the first.
func (d *doc) altSections(w *bytes.Buffer, s section, alts []kinds.Alternative, class func(kinds.Alternative) string) (bool, error) {
	if err := d.text(w, s.Body, alts[0].Line-1); err != nil {
		return false, err
	}

	put(w, `<div class="grid g2">`+"\n")

	for i, a := range alts {
		end := s.Through
		if i+1 < len(alts) {
			end = alts[i+1].Line - 1
		}

		if err := d.card(w, class(a), a.Title, a.Line, end); err != nil {
			return false, err
		}
	}

	put(w, "</div>\n")

	return true, nil
}

// altList writes alternatives that are bold-led bullets by replacing each
// list in the region with a grid of cards.
func (d *doc) altList(w *bytes.Buffer, s section, alts []kinds.Alternative, class func(kinds.Alternative) string) (bool, error) {
	byLine := make(map[int]kinds.Alternative, len(alts))
	for _, a := range alts {
		byLine[a.Line] = a
	}

	replace := make(map[int]replacer)

	for n := d.md.root.FirstChild(); n != nil; n = n.NextSibling() {
		list, ok := n.(*ast.List)
		if !ok {
			continue
		}

		line := d.md.startLine(list)
		if line < s.Body || line > s.Through {
			continue
		}

		replace[line] = func(w *bytes.Buffer, _ ast.Node) error {
			put(w, `<div class="grid g2">`+"\n")

			for it := list.FirstChild(); it != nil; it = it.NextSibling() {
				a, ok := byLine[d.md.startLine(it)]
				if !ok {
					put(w, `<div class="card">`)

					if err := d.md.children(w, it); err != nil {
						return err
					}

					put(w, "</div>\n")

					continue
				}

				if err := d.altCard(w, class(a), a); err != nil {
					return err
				}
			}

			put(w, "</div>\n")

			return nil
		}
	}

	if len(replace) == 0 {
		return false, nil
	}

	return true, d.textWith(w, s.Body, s.Through, replace)
}

// altCard writes a bullet alternative: its name as the label, its text
// under it.
func (d *doc) altCard(w *bytes.Buffer, class string, a kinds.Alternative) error {
	title := a.Title
	if a.Label != "" {
		title = a.Label + ". " + title
	}

	label, err := d.md.inline(title, a.Line)
	if err != nil {
		return err
	}

	put(w, `<div class="`+class+`"><p class="card-label">`)
	w.Write(label)
	put(w, "</p>")

	if a.Text != "" {
		text, err := d.md.inline(a.Text, a.Line)
		if err != nil {
			return err
		}

		put(w, "<p>")
		w.Write(text)
		put(w, "</p>")
	}

	put(w, "</div>\n")

	return nil
}

// headingAt is the heading on line, nil when there is none.
func (d *doc) headingAt(line int) *docparse.Heading {
	for i := range d.headings {
		if d.headings[i].Line == line {
			return &d.headings[i]
		}
	}

	return nil
}

// risks writes the region as text with its tables' likelihood and impact
// cells tinted.
func (d *doc) risks(w *bytes.Buffer, s section) (bool, error) {
	d.md.nodes.tint = true
	defer func() { d.md.nodes.tint = false }()

	return true, d.text(w, s.Body, s.Through)
}

// criteria writes each criterion kinds.Criteria finds as an ok card,
// keeping everything around the lists, a bold group label included.
func (d *doc) criteria(w *bytes.Buffer, s section) (bool, error) {
	r, ok := d.region(s)
	if !ok {
		return false, nil
	}

	crit := kinds.ShiftCriteria(kinds.Criteria(kinds.RegionBytes(d.body, r)), r)
	if len(crit) == 0 {
		return false, nil
	}

	return true, d.textWith(w, s.Body, s.Through, d.cardLists(s, crit, "card ok"))
}

// cardLists replaces each list holding a criterion with a grid of cards,
// one per item.
func (d *doc) cardLists(s section, crit []kinds.Criterion, class string) map[int]replacer {
	lines := make(map[int]bool, len(crit))
	for _, c := range crit {
		lines[c.Line] = true
	}

	replace := make(map[int]replacer)

	for n := d.md.root.FirstChild(); n != nil; n = n.NextSibling() {
		list, ok := n.(*ast.List)
		if !ok {
			continue
		}

		line := d.md.startLine(list)
		if line < s.Body || line > s.Through || !lines[line] {
			continue
		}

		replace[line] = func(w *bytes.Buffer, _ ast.Node) error {
			put(w, `<div class="grid g2">`+"\n")

			for it := list.FirstChild(); it != nil; it = it.NextSibling() {
				put(w, `<div class="`+class+`">`)

				if err := d.itemContent(w, it); err != nil {
					return err
				}

				put(w, "</div>\n")
			}

			put(w, "</div>\n")

			return nil
		}
	}

	return replace
}

// itemContent writes a list item's content: the item's own li and
// checkbox are the caller's to write.
func (d *doc) itemContent(w *bytes.Buffer, it ast.Node) error {
	var buf bytes.Buffer
	if err := d.md.children(&buf, it); err != nil {
		return err
	}

	w.Write(bytes.TrimSpace(buf.Bytes()))

	return nil
}

// checklist writes the region's top-level lists as open tasks.
func (d *doc) checklist(w *bytes.Buffer, s section) (bool, error) {
	replace := make(map[int]replacer)

	for n := d.md.root.FirstChild(); n != nil; n = n.NextSibling() {
		list, ok := n.(*ast.List)
		if !ok {
			continue
		}

		line := d.md.startLine(list)
		if line < s.Body || line > s.Through {
			continue
		}

		replace[line] = func(w *bytes.Buffer, _ ast.Node) error {
			return d.openTasks(w, list, "")
		}
	}

	if len(replace) == 0 {
		return false, nil
	}

	return true, d.textWith(w, s.Body, s.Through, replace)
}

// openTasks writes a list as a task list every item of which is open.
func (d *doc) openTasks(w *bytes.Buffer, list ast.Node, id string) error {
	put(w, `<ul class="tasks"`+id+">\n")

	for it := list.FirstChild(); it != nil; it = it.NextSibling() {
		put(w, `<li data-state="open"><input type="checkbox" disabled><span class="text">`)

		if err := d.itemContent(w, it); err != nil {
			return err
		}

		put(w, "</span></li>\n")
	}

	put(w, "</ul>\n")

	return nil
}

// findings writes one card per level-3 heading in the region, each with
// what is under it; anything before the first stays text.
func (d *doc) findings(w *bytes.Buffer, s section) (bool, error) {
	var hs []docparse.Heading

	for _, h := range d.headings {
		if h.Line >= s.Body && h.Line <= s.Through && h.Level == 3 {
			hs = append(hs, h)
		}
	}

	if len(hs) == 0 {
		return false, nil
	}

	if err := d.text(w, s.Body, hs[0].Line-1); err != nil {
		return false, err
	}

	for i, h := range hs {
		end := s.Through
		if i+1 < len(hs) {
			end = hs[i+1].Line - 1
		}

		if err := d.card(w, "card", h.Text, h.Line, end); err != nil {
			return false, err
		}
	}

	return true, nil
}

// fieldLine is a scenario's bold field line, which its definition list
// shows instead.
var fieldLine = regexp.MustCompile(`^\s*\*\*(Alert|Likely cause)(:\*\*|\*\*:)`)

// answerLine reads the **Answer:** field, either bold spelling.
var answerLine = regexp.MustCompile(`^\s*\*\*Answer(:\*\*|\*\*:)`)

// verdict writes a conclusion: the **Answer:** line as a verdict, classed
// by investigation.Parse's reading of its first word, then the rest.
func (d *doc) verdict(w *bytes.Buffer, s section) (bool, error) {
	inv := d.invDoc()
	if inv == nil || inv.Answer == "" {
		return false, nil
	}

	line := 0

	for n := s.Body; n <= s.Through && n <= len(d.lines); n++ {
		if answerLine.MatchString(d.lines[n-1]) {
			line = n

			break
		}
	}

	if line == 0 || d.md.topAt(line) == nil {
		return false, nil
	}

	return true, d.textWith(w, s.Body, s.Through, map[int]replacer{
		line: func(w *bytes.Buffer, _ ast.Node) error {
			text, err := d.md.inline(inv.Answer, line)
			if err != nil {
				return err
			}

			put(w, `<div class="verdict`+verdictClass(inv.Verdict)+`"><p class="card-label">Answer</p><p>`)
			w.Write(text)
			put(w, "</p></div>\n")

			return nil
		},
	})
}

// progressBar writes a phase's progress: done over total and the bar.
func progressBar(w *bytes.Buffer, done, total int) {
	pct := 0
	if total > 0 {
		pct = done * 100 / total
	}

	putf(w, `<span class="progress">%d / %d</span></div>`+"\n", done, total)
	put(w, `<div class="bar"><i style="width: `+strconv.Itoa(pct)+`%"></i></div>`+"\n")
}

// problem records a typed layout that could not be used.
func (d *doc) problem(format string, args ...any) {
	d.problems = append(d.problems, fmt.Errorf("html: "+format, args...))
}
