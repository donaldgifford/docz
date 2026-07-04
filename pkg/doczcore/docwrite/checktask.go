package docwrite

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
	"github.com/donaldgifford/docz/pkg/doczcore/docparse"
)

// Sentinel errors returned by CheckTask, distinguishable with errors.Is.
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
)

// CheckTask flips the unchecked checkbox task item on the given 1-based
// line of path to checked ("[ ]" -> "[x]"). Only the state byte inside
// the three-byte marker changes; every other byte of the file is
// preserved, so the resulting diff is a single line (DESIGN-0005's
// byte-preservation contract, extended to checkboxes by ADR-0001).
//
// The target line is validated with docparse.TaskItems, so a
// docparse.TaskItem.Line for an unchecked item is accepted by
// construction. Line accounting is LF-only, matching docparse's.
//
// Errors:
//   - ErrLineOutOfRange if line does not exist in the file.
//   - ErrNotTaskItem if the line is not a checkbox task item.
//   - ErrTaskAlreadyChecked if the item is already checked.
//   - ErrUnsupportedLineEndings if the file uses CR/CRLF endings.
//   - os.ReadFile / os.WriteFile errors, wrapped with path.
func CheckTask(path string, line int) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	// Reject CR/CRLF up front: the line scan below assumes LF, and a
	// silent rewrite of a CRLF file would corrupt its endings.
	if bytes.IndexByte(content, '\r') >= 0 {
		return fmt.Errorf("%s: %w", path, ErrUnsupportedLineEndings)
	}

	if line < 1 {
		return fmt.Errorf("%s: line %d: %w", path, line, ErrLineOutOfRange)
	}

	// Walk LF boundaries to the start of the target line. Lines are
	// what strings.Split on "\n" yields, so a file ending in a newline
	// has a final empty line — consistent with docparse's accounting.
	lineStart := 0
	for cur := 1; cur < line; cur++ {
		rel := bytes.IndexByte(content[lineStart:], '\n')
		if rel < 0 {
			return fmt.Errorf("%s: line %d: %w", path, line, ErrLineOutOfRange)
		}
		lineStart += rel + 1
	}
	lineBytes, _ := nextLine(content, lineStart)

	items := docparse.TaskItems(lineBytes)
	if len(items) == 0 {
		return fmt.Errorf("%s: line %d: %w", path, line, ErrNotTaskItem)
	}
	if items[0].Checked {
		return fmt.Errorf("%s: line %d: %w", path, line, ErrTaskAlreadyChecked)
	}

	// On a validated task line the first '[' opens the checkbox; the
	// state byte follows it.
	markerRel := bytes.IndexByte(lineBytes, '[')
	content[lineStart+markerRel+1] = 'x'

	if err := os.WriteFile(path, content, config.FileMode); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	return nil
}
