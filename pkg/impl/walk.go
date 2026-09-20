package impl

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

var (
	// verifyLine matches a continuation line that names a command to run.
	// Case-insensitive, and the bold spelling the corpus sometimes uses is
	// accepted, because an author writing "**verify:**" meant the same thing.
	verifyLine = regexp.MustCompile(`(?i)^\*{0,2}verify\*{0,2}\s*:`)

	// deferredMarker matches the deferred token and the dash that introduces
	// its note. Three dashes because the corpus uses all three: a hyphen, an
	// en dash, and an em dash. The bold is optional and the note may be
	// absent, so a bare "deferred" with no dash still marks the task.
	deferredMarker = regexp.MustCompile(`(?i)\*{0,2}deferred\*{0,2}\s*(?:[-–—]\s*(.*))?$`)

	// humanRequired strips the qualification the corpus writes before a
	// deferred note's real text, so Note is the reason rather than the
	// category.
	humanRequired = regexp.MustCompile(`(?i)^human[ _-]?required\s*:?\s*`)

	// skippedText matches a struck-through task followed by a skipped note:
	// "~~do the thing~~ — skipped: no longer needed".
	skippedText = regexp.MustCompile(`(?i)^~~(.*?)~~\s*[-–—]?\s*skipped\s*:?\s*(.*)$`)

	// backtickSpan captures the contents of a code span, for a verify line's
	// command.
	backtickSpan = regexp.MustCompile("`([^`]+)`")
)

// parseTasks reads the tasks of one tasks region.
//
// A task is a checkbox at indent 0 inside the region. A nested checkbox is
// never a task (INV-0010 Obs 3): the corpus uses them as notes under a task,
// and counting them would make progress depend on how someone indented.
//
// Line numbers are the document's, not the region's, because Line is the
// splice target docwrite.CheckTask writes at.
func parseTasks(lines []string, at docparse.Region, token string) []Task {
	region := regionLines(lines, at)
	items := docparse.TaskItems([]byte(strings.Join(region, "\n")))

	starts := make(map[int]bool, len(items))
	for _, item := range items {
		starts[item.Line] = true
	}

	// Every list item, not only the checkboxes: a plain bullet ends the task
	// above it just as a checkbox does, and treating it as a continuation
	// would fold a stray bullet into the previous task's text.
	for _, item := range docparse.ListItems([]byte(strings.Join(region, "\n"))) {
		starts[item.Line] = true
	}

	var out []Task

	index := 0

	for _, item := range items {
		if item.Indent > 0 {
			continue
		}

		index++
		out = append(out, buildTask(region, starts, item, at.Start, token, index))
	}

	return out
}

// regionLines returns the lines strictly inside a region, which is the
// heading and the content under it.
func regionLines(lines []string, at docparse.Region) []string {
	from := at.Start
	if from < 0 {
		from = 0
	}

	through := at.End - 1
	if through > len(lines) {
		through = len(lines)
	}

	if through <= from {
		return nil
	}

	return lines[from:through]
}

// buildTask folds a task's continuation lines in and reads its state off
// them.
func buildTask(
	region []string, starts map[int]bool, item docparse.TaskItem,
	offset int, token string, index int,
) Task {
	task := Task{
		ID:      fmt.Sprintf("%s.%d", token, index),
		Checked: item.Checked,
		Line:    offset + item.Line,
		EndLine: offset + item.Line,
	}

	// The task's own line and every continuation, each kept with the line it
	// came from: a deferred marker reports the line it is written on, which is
	// often a continuation rather than the checkbox.
	parts := []part{{text: item.Text, line: offset + item.Line}}

	for n := item.Line + 1; n <= len(region); n++ {
		line := region[n-1]
		if strings.TrimSpace(line) == "" || starts[n] {
			break
		}

		if indentOf(line) <= item.Indent {
			break
		}

		task.EndLine = offset + n
		trimmed := strings.TrimSpace(line)

		if verifyLine.MatchString(trimmed) {
			// The verify line is state, not text: it says how to check the
			// task, so folding it into Text would make every consumer that
			// matches on Text strip it again.
			if task.Verify == "" {
				task.Verify = commandIn(trimmed)
			}

			continue
		}

		parts = append(parts, part{text: trimmed, line: offset + n})
	}

	task.Text = strings.TrimSpace(foldParts(parts))
	task.Deferred = deferredIn(parts)
	task.Skipped = skippedIn(&task, offset+item.Line)

	if task.Deferred != nil {
		task.Text = strings.TrimSpace(stripDeferred(task.Text))
	}

	return task
}

// part is one line of a task: the checkbox itself or a continuation.
type part struct {
	text string
	line int
}

// foldParts joins a task's lines with single spaces, which is what makes a
// wrapped task compare equal to the same task written on one line.
func foldParts(parts []part) string {
	texts := make([]string, 0, len(parts))
	for _, p := range parts {
		texts = append(texts, p.text)
	}

	return strings.Join(texts, " ")
}

// commandIn returns the first code span on a verify line, which is the
// command. A verify line with no span names no command, and validate reports
// that as impl.task.verify-no-command: a verify step nobody can run is worse
// than none, because it reads as covered.
func commandIn(line string) string {
	m := backtickSpan.FindStringSubmatch(line)
	if m == nil {
		return ""
	}

	return strings.TrimSpace(m[1])
}

// deferredIn finds a deferred marker on the task line or any continuation.
//
// Both spellings the fleet uses are accepted: the prefix form docz-api
// IMPL-0006 writes, where the task text opens with "deferred - …", and the
// suffix form issue #100 asked for, where it closes with one.
func deferredIn(parts []part) *Marker {
	for _, p := range parts {
		m := deferredMarker.FindStringSubmatch(p.text)
		if m == nil {
			continue
		}

		note := humanRequired.ReplaceAllString(strings.TrimSpace(m[1]), "")

		return &Marker{Note: strings.TrimSpace(note), Line: p.line}
	}

	return nil
}

// stripDeferred removes the marker text from a task's Text, so Text is what
// the task asks for and Deferred carries why it is not happening.
func stripDeferred(text string) string {
	return strings.Trim(deferredMarker.ReplaceAllString(text, ""), " -–—:")
}

// skippedIn reads a skipped marker off a struck-through task, rewriting Text
// to the task as it was written before the strike.
//
// A skipped task keeps its ID. Renumbering the tasks after it would move
// every address in the phase the moment one task was abandoned, and those
// addresses are in commit messages and CI logs.
func skippedIn(task *Task, line int) *Marker {
	m := skippedText.FindStringSubmatch(task.Text)
	if m == nil {
		return nil
	}

	task.Text = strings.TrimSpace(m[1])

	return &Marker{Note: strings.TrimSpace(m[2]), Line: line}
}

// indentOf counts leading whitespace characters, tabs as one, matching
// docparse.TaskItem.Indent so the two agree about which lines are deeper
// than a bullet.
func indentOf(line string) int {
	for i, r := range line {
		if r != ' ' && r != '\t' {
			return i
		}
	}

	return len(line)
}
