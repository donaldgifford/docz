// Package docwrite provides the CLI-only write side of docz documents:
// creating new documents from templates (Create) and mutating frontmatter
// status in place (SetStatus). It is intentionally internal — the public
// pkg/doczcore surface is read-only (DESIGN-0007) — and depends on the
// read-side pkg/doczcore/document for the shared parsing primitives.
package docwrite

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
	"github.com/donaldgifford/docz/pkg/doczcore/document"
)

// ErrStatusFieldMissing is returned by SetStatus when a file has valid
// frontmatter delimiters but no usable status: key — either the key is
// absent, or its value uses an unsupported YAML shape (block scalar,
// flow mapping/sequence, anchor, or alias) that the byte-level mutator
// deliberately refuses to rewrite.
var ErrStatusFieldMissing = errors.New("no status field in frontmatter")

// ErrUnsupportedLineEndings is returned by SetStatus when a file uses CR
// or CRLF line endings. The byte-level mutator only supports LF, matching
// docz's Unix-only stance (DESIGN-0005 Decision 7).
var ErrUnsupportedLineEndings = errors.New("unsupported line endings (want LF)")

// statusKeyRE matches a frontmatter status line anchored at column 0,
// consuming the `status:` key and the spacing that precedes the value.
// The value itself and any trailing inline comment are parsed by hand in
// parseStatusValue rather than captured here: a single mega-regex cannot
// both recognize the three YAML scalar shapes (bare, "double", 'single')
// and cleanly separate a bare value from a trailing `# comment` without
// over-capturing, and the byte-preservation contract (DESIGN-0005
// §Frontmatter mutation) requires that separation to be exact.
var statusKeyRE = regexp.MustCompile(`^status:[ \t]*`)

// SetStatus rewrites the status: field in path's YAML frontmatter to
// newStatus and returns the old status it replaced. Only the value bytes
// change: the key token, the colon, the spacing, the quoting glyphs, any
// trailing comment, and every other line are preserved byte-for-byte, so
// the resulting diff is a single value substitution (DESIGN-0005
// §Frontmatter mutation).
//
// Errors:
//   - ErrNoFrontmatter if path has no leading --- block.
//   - ErrStatusFieldMissing if the block exists but has no usable
//     status: key.
//   - ErrUnsupportedLineEndings if the file uses CR/CRLF endings.
//   - os.ReadFile / os.WriteFile errors, wrapped with path.
//
// The helper always writes when it locates a status field; the cmd layer
// owns the current-vs-new short-circuit (DESIGN-0005 Decision 8). Calling
// SetStatus with the value a file already holds is a no-op at the byte
// level — it produces identical output.
func SetStatus(path, newStatus string) (oldStatus string, err error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}

	// Reject CR/CRLF up front: the line scan below assumes LF, and a
	// silent rewrite of a CRLF file would corrupt its endings.
	if bytes.IndexByte(content, '\r') >= 0 {
		return "", fmt.Errorf("%s: %w", path, ErrUnsupportedLineEndings)
	}

	start, end, oldStatus, err := findStatusValue(content)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}

	out := make([]byte, 0, len(content)-(end-start)+len(newStatus))
	out = append(out, content[:start]...)
	out = append(out, newStatus...)
	out = append(out, content[end:]...)

	if err := os.WriteFile(path, out, config.FileMode); err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}

	return oldStatus, nil
}

// findStatusValue locates the byte range [start, end) of the status value
// inside content's YAML frontmatter and returns the current value. The
// returned offsets cover only the value bytes; surrounding quotes,
// spacing, and comments fall outside the range so the caller can splice a
// replacement in place.
func findStatusValue(content []byte) (start, end int, value string, err error) {
	blockStart, blockEnd, ok := frontmatterBounds(content)
	if !ok {
		return 0, 0, "", document.ErrNoFrontmatter
	}

	block := content[blockStart:blockEnd]
	for lineOff := 0; lineOff < len(block); {
		line, next := nextLine(block, lineOff)
		if loc := statusKeyRE.FindIndex(line); loc != nil {
			prefixLen := loc[1]
			valStart, valEnd, val, perr := parseStatusValue(line, prefixLen)
			if perr != nil {
				return 0, 0, "", perr
			}
			abs := blockStart + lineOff
			return abs + valStart, abs + valEnd, val, nil
		}
		lineOff = next
	}

	return 0, 0, "", ErrStatusFieldMissing
}

// frontmatterBounds returns the byte offsets bracketing the YAML body of
// content's frontmatter: blockStart points just past the opening "---\n"
// and blockEnd points at the newline preceding the closing "---". A
// single leading blank line is tolerated (IMPL-0011 Phase 1: "line 0 or
// 1"). ok is false when no well-formed --- block is present.
func frontmatterBounds(content []byte) (blockStart, blockEnd int, ok bool) {
	offset := 0
	if len(content) > 0 && content[0] == '\n' {
		offset = 1
	}
	if !bytes.HasPrefix(content[offset:], []byte("---\n")) {
		return 0, 0, false
	}
	blockStart = offset + len("---\n")

	// The closing delimiter must be a full "---" line: a "\n---" run
	// followed by a newline or EOF. Requiring the line boundary stops a
	// body line like "---more" from being mistaken for the close.
	sub := content[blockStart:]
	for searchOff := 0; ; {
		rel := bytes.Index(sub[searchOff:], []byte("\n---"))
		if rel < 0 {
			return 0, 0, false
		}
		matchAt := searchOff + rel
		after := matchAt + len("\n---")
		if after == len(sub) || sub[after] == '\n' {
			return blockStart, blockStart + matchAt + 1, true
		}
		searchOff = matchAt + 1
	}
}

// nextLine returns the line of b starting at off (excluding its trailing
// newline) and the offset at which the following line begins. When b has
// no further newline, next equals len(b).
func nextLine(b []byte, off int) (line []byte, next int) {
	rel := bytes.IndexByte(b[off:], '\n')
	if rel < 0 {
		return b[off:], len(b)
	}
	return b[off : off+rel], off + rel + 1
}

// parseStatusValue splits the value out of a status line whose key and
// leading spacing occupy line[:prefixLen]. It returns the byte range of
// the value within the line and the value string, dispatching on the
// first value byte to recognize the three supported scalar shapes.
// Unsupported shapes (block scalars, flow collections, anchors, aliases)
// and unterminated quotes yield ErrStatusFieldMissing.
func parseStatusValue(line []byte, prefixLen int) (valStart, valEnd int, value string, err error) {
	rest := line[prefixLen:]
	if len(rest) == 0 {
		// `status:` with no value — an empty bare scalar.
		return prefixLen, prefixLen, "", nil
	}

	switch rest[0] {
	case '"':
		return quotedValue(rest, prefixLen, '"')
	case '\'':
		return quotedValue(rest, prefixLen, '\'')
	case '|', '>', '{', '[', '&', '*':
		return 0, 0, "", fmt.Errorf(
			"status value uses unsupported YAML shape %q: %w",
			string(rest[0]), ErrStatusFieldMissing,
		)
	default:
		n := bareValueLen(rest)
		return prefixLen, prefixLen + n, string(rest[:n]), nil
	}
}

// quotedValue extracts the value of a quoted scalar whose opening quote is
// rest[0]. prefixLen is the value region's offset within the source line.
// The returned range covers only the bytes between the quotes, leaving the
// glyphs in place for the caller to preserve.
func quotedValue(rest []byte, prefixLen int, quote byte) (valStart, valEnd int, value string, err error) {
	closeRel := bytes.IndexByte(rest[1:], quote)
	if closeRel < 0 {
		return 0, 0, "", fmt.Errorf(
			"unterminated %c-quoted status value: %w", quote, ErrStatusFieldMissing,
		)
	}
	valStart = prefixLen + 1
	valEnd = valStart + closeRel
	return valStart, valEnd, string(rest[1 : 1+closeRel]), nil
}

// bareValueLen returns the length of the bare scalar at the start of rest,
// stopping before an inline comment (a '#' preceded by whitespace) and
// trimming the whitespace that separates the value from that comment or
// the line end. rest is non-empty and rest[0] is not a quote or shape
// indicator.
func bareValueLen(rest []byte) int {
	if rest[0] == '#' {
		return 0
	}
	end := len(rest)
	for i := 1; i < len(rest); i++ {
		if rest[i] == '#' && (rest[i-1] == ' ' || rest[i-1] == '\t') {
			end = i
			break
		}
	}
	for end > 0 && (rest[end-1] == ' ' || rest[end-1] == '\t') {
		end--
	}
	return end
}
