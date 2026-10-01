package runbook

import (
	"slices"
	"strconv"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

// expectedLabels are the two bold spellings of the Expected line, the colon
// inside the bold and outside it, as kinds.Field accepts for every field.
var expectedLabels = []string{"**Expected:**", "**Expected**:"}

// node is a step under construction. The tree is built with pointers, so a
// child can be appended to a parent the walk has already moved past, and
// turned into values once the region is read.
type node struct {
	step     Step
	indent   int
	parts    []string
	expected []string
	children []*node
}

// walker reads one steps or rollback region (DESIGN-0019 §4, rules 1–6).
//
// It is a single pass over the region's lines with a stack of open steps. An
// ordered item closes every open step at its indent or deeper and opens a new
// one under whatever is left. Any other line belongs to the deepest open step
// it is indented under, and a line at a step's own indent or shallower closes
// it. Blank lines close nothing, because a step's command and its Expected
// line sit below it after one.
type walker struct {
	offset int
	root   string
	items  map[int]docparse.ListItem

	roots []*node
	stack []*node

	// expecting is the step whose Expected line is still being continued; a
	// blank line or a new item ends it.
	expecting *node

	// fence is the open fenced block, nil outside one.
	fence *fence
}

// fence is a fenced block being read, and the step it belongs to.
type fence struct {
	owner   *node
	command Command
	indent  int
	body    []string
}

// parseSteps reads the steps of one steps or rollback region.
//
// root is what the top-level IDs are built on: "2." numbers them "2.1",
// "2.2"; "2.R" numbers a rollback "2.R1"; "S1." numbers a scenario's "S1.1".
// Children always extend their parent's ID with ".<n>".
//
// Line numbers are the document's, not the region's, so a Step's Line is an
// address an editor or a consumer can act on.
func parseSteps(lines []string, at docparse.Region, root string) []Step {
	region := regionLines(lines, at)

	w := &walker{
		offset: at.Start,
		root:   root,
		items:  listItemsByLine(region),
	}

	for n, line := range region {
		w.line(n+1, line)
	}

	w.closeFence()

	if len(w.roots) == 0 {
		return nil
	}

	return build(w.roots)
}

// listItemsByLine indexes a region's list items by their region line. The
// facts layer is fence-aware, so a "1." inside a command is never a step.
func listItemsByLine(region []string) map[int]docparse.ListItem {
	items := docparse.ListItems([]byte(strings.Join(region, "\n")))

	out := make(map[int]docparse.ListItem, len(items))
	for _, item := range items {
		out[item.Line] = item
	}

	return out
}

// line consumes one region line, n being its 1-based region line number.
func (w *walker) line(n int, line string) {
	trimmed := strings.TrimSpace(line)

	if w.fence != nil {
		if isFence(trimmed) {
			w.touch(n)
			w.closeFence()

			return
		}

		w.fence.body = append(w.fence.body, dedent(line, w.fence.indent))
		w.touch(n)

		return
	}

	if isFence(trimmed) {
		w.openFence(n, line, trimmed)

		return
	}

	if trimmed == "" {
		w.expecting = nil

		return
	}

	if item, ok := w.items[n]; ok {
		w.item(n, item, trimmed)

		return
	}

	if strings.HasPrefix(trimmed, "#") {
		// A heading ends every step. The region's own heading is its first
		// line, and a deeper one is a document structuring its steps some
		// other way; neither belongs to a step's text.
		w.stack = nil
		w.expecting = nil

		return
	}

	w.close(indentOf(line))

	top := w.top()
	if top == nil {
		return
	}

	w.touch(n)

	if rest, ok := cutExpected(trimmed); ok {
		top.expected = []string{rest}
		w.expecting = top

		return
	}

	if w.expecting == top {
		top.expected = append(top.expected, trimmed)

		return
	}

	top.parts = append(top.parts, trimmed)
}

// item consumes a list item line. An ordered item is a step; a bullet is a
// note on the step it is indented under, or nobody's when it sits at step
// level, where validate reports it as runbook.step.unordered.
func (w *walker) item(n int, item docparse.ListItem, trimmed string) {
	w.expecting = nil
	w.close(item.Indent)

	if !item.Ordered {
		top := w.top()
		if top == nil {
			return
		}

		w.touch(n)
		top.parts = append(top.parts, trimmed)

		return
	}

	step := &node{indent: item.Indent, parts: []string{item.Text}}
	step.step.Line = w.offset + n
	step.step.EndLine = step.step.Line

	if parent := w.top(); parent != nil {
		parent.children = append(parent.children, step)
		step.step.ID = parent.step.ID + "." + strconv.Itoa(len(parent.children))
	} else {
		w.roots = append(w.roots, step)
		step.step.ID = w.root + strconv.Itoa(len(w.roots))
	}

	w.stack = append(w.stack, step)
	w.touch(n)
}

// openFence starts a fenced block. It belongs to the deepest open step,
// whatever its indent (rule 4): a command under a step is the step's even
// when the author did not indent it.
func (w *walker) openFence(n int, line, trimmed string) {
	w.expecting = nil

	lang := strings.TrimSpace(strings.TrimLeft(trimmed, "`"))
	if i := strings.IndexAny(lang, " \t"); i >= 0 {
		lang = lang[:i]
	}

	w.fence = &fence{
		owner:   w.top(),
		command: Command{Lang: lang, Line: w.offset + n},
		indent:  indentOf(line),
	}

	w.touch(n)
}

// closeFence attaches the open block, if any, to its step. A block with no
// step above it is prose that happens to be code, and is dropped.
func (w *walker) closeFence() {
	f := w.fence
	if f == nil {
		return
	}

	w.fence = nil

	if f.owner == nil {
		return
	}

	f.command.Body = strings.Join(f.body, "\n")
	f.owner.step.Commands = append(f.owner.step.Commands, f.command)
}

// close ends every open step whose indent is at or deeper than indent.
func (w *walker) close(indent int) {
	for len(w.stack) > 0 && w.stack[len(w.stack)-1].indent >= indent {
		w.stack = w.stack[:len(w.stack)-1]
	}

	if w.expecting != nil && !slices.Contains(w.stack, w.expecting) {
		w.expecting = nil
	}
}

func (w *walker) top() *node {
	if len(w.stack) == 0 {
		return nil
	}

	return w.stack[len(w.stack)-1]
}

// touch extends every open step to region line n: a child's line is its
// parent's too, which is what makes a parent's EndLine cover its children.
func (w *walker) touch(n int) {
	for _, s := range w.stack {
		s.step.EndLine = w.offset + n
	}
}

// build turns the tree into values, folding each step's text.
func build(nodes []*node) []Step {
	out := make([]Step, 0, len(nodes))

	for _, n := range nodes {
		step := n.step
		step.Text = fold(n.parts)
		step.Expected = fold(n.expected)

		if len(n.children) > 0 {
			step.Children = build(n.children)
		}

		out = append(out, step)
	}

	return out
}

// fold joins lines with single spaces and removes comments, so a wrapped
// step compares equal to the same step on one line and the template's
// guidance never reads as text.
func fold(parts []string) string {
	return strings.Join(strings.Fields(stripComments(strings.Join(parts, " "))), " ")
}

// cutExpected strips an Expected label from a line.
func cutExpected(line string) (string, bool) {
	for _, label := range expectedLabels {
		if rest, ok := strings.CutPrefix(line, label); ok {
			return strings.TrimSpace(rest), true
		}
	}

	return "", false
}

// isFence is docparse's fence rule: a trimmed line opening with three
// backticks toggles a block. Duplicated rather than exported from the facts
// layer, which keeps its rule to itself.
func isFence(trimmed string) bool {
	return strings.HasPrefix(trimmed, "```")
}

// dedent removes up to indent leading whitespace characters, which is the
// fence's own indentation: a command written under a numbered step is
// indented to sit inside the item, and that is not part of the command.
func dedent(line string, indent int) string {
	i := 0
	for i < indent && i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}

	return line[i:]
}

// stepLevelBullets returns the document lines of the bullets in a steps
// region that sit at step level: at or shallower than its shallowest ordered
// item.
//
// It reports nothing for a region with no ordered item at all. That region is
// the generic tier's steps.not-ordered, and saying it again here, once per
// bullet, would report one mistake many times.
func stepLevelBullets(lines []string, at docparse.Region) []int {
	region := regionLines(lines, at)
	items := docparse.ListItems([]byte(strings.Join(region, "\n")))

	level := -1

	for _, item := range items {
		if item.Ordered && (level < 0 || item.Indent < level) {
			level = item.Indent
		}
	}

	if level < 0 {
		return nil
	}

	var out []int

	for _, item := range items {
		if !item.Ordered && item.Indent <= level {
			out = append(out, at.Start+item.Line)
		}
	}

	return out
}

// regionLines returns the lines strictly inside a region, which is the
// heading and the content under it. Region line n is document line
// at.Start+n.
func regionLines(lines []string, at docparse.Region) []string {
	from := max(at.Start, 0)
	through := min(at.End-1, len(lines))

	if through <= from {
		return nil
	}

	return lines[from:through]
}

// indentOf counts leading whitespace characters, tabs as one, matching
// docparse.ListItem.Indent so the two agree about which lines are deeper
// than an item.
func indentOf(line string) int {
	for i, r := range line {
		if r != ' ' && r != '\t' {
			return i
		}
	}

	return len(line)
}

// stripComments removes HTML comments, including ones spanning lines, the
// way the kinds readers do.
func stripComments(s string) string {
	var sb strings.Builder

	for {
		before, rest, found := strings.Cut(s, "<!--")
		sb.WriteString(before)

		if !found {
			return sb.String()
		}

		_, after, closed := strings.Cut(rest, "-->")
		if !closed {
			return sb.String()
		}

		s = after
	}
}
