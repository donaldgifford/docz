package kinds

import (
	"strconv"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

// Decision is one row of a decisions table: a question and how it was
// settled.
type Decision struct {
	// Number is the question number from the table's first column, or 0
	// when the row does not carry one. The corpus writes an em dash there
	// for a row that records an amendment rather than answering a
	// numbered question, and those rows are real decisions worth
	// reporting.
	Number int

	// Question is the question column, inline markdown kept.
	Question string

	// Resolution is the decision column, inline markdown kept.
	Resolution string

	// Line is the 1-based line of the row, counted from the start of the
	// region.
	Line int
}

// Decisions returns the rows of the region's first table that has a
// Question column and a Decision or Resolution column.
//
// Only the first such table. A region with two of them is ambiguous about
// which one records the decisions, and picking the first is both the
// answer the corpus wants and a rule a reader can predict.
//
// Column position is not fixed: the header is matched by name, folded, so
// a document that orders its columns differently or titles the third one
// "Resolution" still parses. A table without both columns is skipped
// rather than guessed at.
//
// Four spellings of the decision column are accepted, because the corpus
// writes four. ADR-0001's table is headed "Choice" and carries a fourth
// "Notes" column; a reader that only knew "Decision" and "Resolution" could
// not read this repo's own ADR-0001, and validate would report a finding
// against a document that is written correctly. Extra columns are ignored.
func Decisions(region []byte) []Decision {
	lines := bodyLines(region)

	for _, table := range docparse.Tables([]byte(strings.Join(lines, "\n"))) {
		number, question, resolution := decisionColumns(table.Header)
		if question < 0 || resolution < 0 {
			continue
		}

		out := make([]Decision, 0, len(table.Rows))

		for i, row := range table.Rows {
			if question >= len(row) || resolution >= len(row) {
				continue
			}

			out = append(out, Decision{
				Number:     decisionNumber(row, number),
				Question:   strings.TrimSpace(row[question]),
				Resolution: strings.TrimSpace(row[resolution]),
				// Header, then the delimiter row, then the body.
				Line: table.Line + 2 + i,
			})
		}

		if len(out) == 0 {
			return nil
		}

		return out
	}

	return nil
}

// decisionColumns locates the number, question, and decision columns by
// header name. Any of the three may be absent, reported as -1.
func decisionColumns(header []string) (number, question, resolution int) {
	number, question, resolution = -1, -1, -1

	for i, cell := range header {
		switch strings.ToLower(strings.TrimSpace(cell)) {
		case "#", "no", "no.", "num", "number":
			if number < 0 {
				number = i
			}
		case "question", "open question":
			if question < 0 {
				question = i
			}
		case "decision", "resolution", "answer", "choice":
			if resolution < 0 {
				resolution = i
			}
		}
	}

	return number, question, resolution
}

// decisionNumber reads the row's number column, or 0 when the table has
// none or the cell is not a number.
func decisionNumber(row []string, column int) int {
	if column < 0 || column >= len(row) {
		return 0
	}

	n, err := strconv.Atoi(strings.TrimSpace(row[column]))
	if err != nil {
		return 0
	}

	return n
}
