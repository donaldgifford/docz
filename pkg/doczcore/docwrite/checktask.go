package docwrite

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

// Sentinel errors returned by SetTaskState and CheckTask, distinguishable
// with errors.Is.
var (
	// ErrLineOutOfRange is returned when the requested line number does
	// not exist in the file (line < 1 or past the last LF-delimited
	// line).
	ErrLineOutOfRange = errors.New("line out of range")

	// ErrNotTaskItem is returned when the target line is not a checkbox
	// task item as docparse.TaskItems defines one — a "-" or "*" bullet
	// followed by "[ ]", "[x]", or "[X]".
	ErrNotTaskItem = errors.New("line is not a checkbox task item")

	// ErrTaskAlreadyChecked is returned when the target line is a task
	// item whose checkbox is already checked.
	ErrTaskAlreadyChecked = errors.New("task already checked")

	// ErrTaskAlreadyUnchecked is returned when unchecking a task item whose
	// checkbox is already clear.
	//
	// The mirror of ErrTaskAlreadyChecked, and separate from it so a caller
	// can tell which direction it asked for. Both exist because the helpers
	// always write when invoked: the no-op short-circuit belongs to the cmd
	// layer (DESIGN-0005 Decision 8), so the library says "that is already
	// the state" rather than silently rewriting the same bytes.
	ErrTaskAlreadyUnchecked = errors.New("task already unchecked")
)

// CheckTask flips the unchecked checkbox task item on the given 1-based
// line of path to checked ("[ ]" -> "[x]").
//
// It is SetTaskState(path, line, true), kept as its own name because that is
// what IMPL-0011's callers ask for and the direction reads better at the call
// site than a boolean does.
func CheckTask(path string, line int) error {
	return SetTaskState(path, line, true)
}

// SetTaskState sets the checkbox task item on the given 1-based line of path
// to checked or unchecked. Only the state byte inside the three-byte marker
// changes; every other byte of the file is preserved, so the resulting diff is
// a single line (DESIGN-0005's byte-preservation contract, extended to
// checkboxes by ADR-0001).
//
// The target line is validated with docparse.TaskItems, so a
// docparse.TaskItem.Line is accepted by construction. Line accounting is
// LF-only, matching docparse's.
//
// Errors:
//   - ErrLineOutOfRange if line does not exist in the file.
//   - ErrNotTaskItem if the line is not a checkbox task item.
//   - ErrTaskAlreadyChecked or ErrTaskAlreadyUnchecked if the item already
//     holds the requested state.
//   - ErrUnsupportedLineEndings if the file uses CR/CRLF endings.
//   - os.ReadFile / os.WriteFile errors, wrapped with path.
func SetTaskState(path string, line int, checked bool) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	out, err := SetTaskStateBytes(content, line, checked)
	if err != nil {
		return fmt.Errorf("%s: line %d: %w", path, line, err)
	}

	if err := os.WriteFile(path, out, config.FileMode); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	return nil
}

// SetTaskStateBytes is SetTaskState without the filesystem: it returns the
// rewritten document and never reads or writes a file.
//
// This is the byte core (DESIGN-0014 §2.7), for a consumer holding bytes it
// fetched rather than a path it can write — docz-api reads through the GitHub
// API and commits through it too. The input is not modified: the splice lands
// in a copy, so a caller may keep or reuse what it passed in.
//
// Errors are bare sentinels, with no path or line in them. The caller knows
// which line it asked for, and the wrapper above adds both.
func SetTaskStateBytes(doc []byte, line int, checked bool) ([]byte, error) {
	// Reject CR/CRLF up front: the line scan below assumes LF, and a
	// silent rewrite of a CRLF file would corrupt its endings.
	if bytes.IndexByte(doc, '\r') >= 0 {
		return nil, ErrUnsupportedLineEndings
	}

	if line < 1 {
		return nil, ErrLineOutOfRange
	}

	// Walk LF boundaries to the start of the target line. Lines are
	// what strings.Split on "\n" yields, so a file ending in a newline
	// has a final empty line — consistent with docparse's accounting.
	lineStart := 0

	for cur := 1; cur < line; cur++ {
		rel := bytes.IndexByte(doc[lineStart:], '\n')
		if rel < 0 {
			return nil, ErrLineOutOfRange
		}

		lineStart += rel + 1
	}

	lineBytes, _ := nextLine(doc, lineStart)

	items := docparse.TaskItems(lineBytes)
	if len(items) == 0 {
		return nil, ErrNotTaskItem
	}

	if items[0].Checked == checked {
		if checked {
			return nil, ErrTaskAlreadyChecked
		}

		return nil, ErrTaskAlreadyUnchecked
	}

	out := bytes.Clone(doc)

	// On a validated task line the first '[' opens the checkbox; the
	// state byte follows it.
	markerRel := bytes.IndexByte(lineBytes, '[')
	out[lineStart+markerRel+1] = stateByte(checked)

	return out, nil
}

// stateByte is what goes between the brackets. Lower-case "x" for checked,
// which is what every template and every document in the corpus writes, even
// though docparse reads "X" too.
func stateByte(checked bool) byte {
	if checked {
		return 'x'
	}

	return ' '
}
