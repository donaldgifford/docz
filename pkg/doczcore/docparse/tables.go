package docparse

import (
	"regexp"
	"strings"
)

// Table is a GFM pipe table: a header row, a delimiter row that is
// dropped, and the body rows under them.
//
// The alignment the delimiter row encodes is not reported. It is a
// rendering instruction, not a fact about what the table says, and no
// docz consumer has ever needed it; a later version can add it without
// changing what Header and Rows mean.
type Table struct {
	// Header holds the header cells, trimmed, with inline markdown kept
	// verbatim.
	Header []string

	// Rows holds the body rows in document order. A row may be shorter or
	// longer than Header: markdown does not require them to agree, and
	// padding or truncating here would invent content. Consumers index
	// defensively or report the mismatch as a finding.
	Rows [][]string

	// Line is the 1-based line number of the header row.
	Line int
}

// delimiterCell matches one cell of a GFM delimiter row: at least one
// dash, with optional leading and trailing colons for alignment.
var delimiterCell = regexp.MustCompile(`^:?-+:?$`)

// Tables extracts every GFM pipe table from content. A table is a line
// containing a pipe, followed by a delimiter row whose every cell is
// dashes with optional alignment colons; body rows continue until a line
// that is not a table row.
//
// Tables inside fenced code blocks are skipped using the same fence rule
// as Headings.
//
// Leading and trailing pipes are optional, as in GFM, and a pipe escaped
// with a backslash does not split a cell. Cells are trimmed; inline
// markdown is kept verbatim, so a link in a File column comes back as the
// link.
func Tables(content []byte) []Table {
	lines := strings.Split(string(content), "\n")

	var tables []Table

	inCodeBlock := false

	for i := 0; i < len(lines); i++ {
		if isFenceToggle(lines[i]) {
			inCodeBlock = !inCodeBlock

			continue
		}

		if inCodeBlock {
			continue
		}

		// A header needs a delimiter row directly under it. Without that
		// rule any prose line containing a pipe would start a table.
		if i+1 >= len(lines) || !isTableRow(lines[i]) || !isDelimiterRow(lines[i+1]) {
			continue
		}

		table := Table{
			Header: splitRow(lines[i]),
			Line:   i + 1,
		}

		// Skip the header and its delimiter, then take rows until the
		// table stops.
		j := i + 2
		for ; j < len(lines) && !isFenceToggle(lines[j]) && isTableRow(lines[j]); j++ {
			table.Rows = append(table.Rows, splitRow(lines[j]))
		}

		tables = append(tables, table)

		// Resume at the line that ended the table so a fence or a second
		// table immediately after is seen.
		i = j - 1
	}

	return tables
}

// isTableRow reports whether the line could be part of a pipe table: it
// has content and contains at least one unescaped pipe.
func isTableRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}

	for i := range len(trimmed) {
		if trimmed[i] == '|' && (i == 0 || trimmed[i-1] != '\\') {
			return true
		}
	}

	return false
}

// isDelimiterRow reports whether the line is a GFM delimiter row: at
// least one cell, every cell dashes with optional alignment colons.
func isDelimiterRow(line string) bool {
	if !isTableRow(line) {
		return false
	}

	cells := splitRow(line)
	if len(cells) == 0 {
		return false
	}

	for _, c := range cells {
		if !delimiterCell.MatchString(c) {
			return false
		}
	}

	return true
}

// splitRow splits a table row into trimmed cells. One leading and one
// trailing pipe are dropped, as GFM allows them to be present or absent,
// and a backslash-escaped pipe stays inside its cell.
func splitRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")

	// Only an unescaped trailing pipe is a delimiter.
	if strings.HasSuffix(trimmed, "|") && !strings.HasSuffix(trimmed, `\|`) {
		trimmed = trimmed[:len(trimmed)-1]
	}

	var (
		cells []string
		cur   strings.Builder
	)

	for i := 0; i < len(trimmed); i++ {
		switch {
		case trimmed[i] == '\\' && i+1 < len(trimmed) && trimmed[i+1] == '|':
			// Keep the escape as written: Text is verbatim markdown.
			cur.WriteString(`\|`)
			i++
		case trimmed[i] == '|':
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(trimmed[i])
		}
	}

	cells = append(cells, strings.TrimSpace(cur.String()))

	return cells
}
