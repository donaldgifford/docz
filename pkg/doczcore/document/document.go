// Package document provides docz frontmatter parsing and document scanning.
//
// It is the read side of the docz core: parsing already-fetched bytes
// (ParseFrontmatter), reading and scanning files on disk (LoadFrontmatter,
// ScanDocuments), and the docz filename convention (IsDoczFile). The
// CLI-only write side — creating documents and mutating frontmatter status
// in place — lives in internal/docwrite (DESIGN-0007).
package document

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
)

// Frontmatter holds the YAML frontmatter metadata from a document file.
//
// Status is the typed config.Status wrapper (DESIGN-0004 §F) so a
// mistaken assignment from a generic string field (e.g. Title) is a
// compile error rather than a silent mis-classification at list/index
// time. yaml/v3 unmarshals the string scalar into the typed field
// without a custom UnmarshalYAML.
type Frontmatter struct {
	ID      string        `yaml:"id"`
	Title   string        `yaml:"title"`
	Status  config.Status `yaml:"status"`
	Author  string        `yaml:"author"`
	Created string        `yaml:"created"`
}

// ErrNoFrontmatter is returned when a file has no YAML frontmatter delimiters.
var ErrNoFrontmatter = errors.New("no YAML frontmatter found")

// ParseFrontmatter extracts and parses YAML frontmatter from file content.
// Frontmatter must be delimited by "---" lines at the start of the file.
func ParseFrontmatter(content []byte) (Frontmatter, error) {
	var fm Frontmatter

	content = bytes.TrimLeft(content, "\n\r")
	if !bytes.HasPrefix(content, []byte("---")) {
		return fm, ErrNoFrontmatter
	}

	// Find the closing delimiter. The opening `---` must be followed by
	// a newline; accept either LF or CRLF so files authored on Windows
	// parse the same as LF-only files. The closing `\n---` lookup below
	// already tolerates CRLF because the trailing `\r` lands on the
	// preceding line.
	rest := content[3:]
	rest = bytes.TrimLeft(rest, " \t")
	switch {
	case len(rest) >= 2 && rest[0] == '\r' && rest[1] == '\n':
		rest = rest[2:]
	case len(rest) >= 1 && rest[0] == '\n':
		rest = rest[1:]
	default:
		return fm, ErrNoFrontmatter
	}

	yamlBlock, _, found := bytes.Cut(rest, []byte("\n---"))
	if !found {
		return fm, errors.New("unterminated YAML frontmatter: missing closing ---")
	}
	if err := yaml.Unmarshal(yamlBlock, &fm); err != nil {
		return fm, fmt.Errorf("parsing YAML frontmatter: %w", err)
	}

	return fm, nil
}

// LoadFrontmatter reads path and parses its YAML frontmatter. The raw
// file bytes are returned alongside the parsed Frontmatter so callers
// that also want the body (e.g. ScanDocuments) get both in one call.
//
// Contract:
//   - Returns (fm, content, nil) when the file is readable and contains
//     valid frontmatter.
//   - Returns (zero Frontmatter, content, ErrNoFrontmatter) when the
//     file is readable but has no frontmatter delimiters. Callers that
//     can derive metadata another way (filename, first H1) should use
//     errors.Is(err, ErrNoFrontmatter) and fall back; this is not a
//     fatal failure.
//   - All other returns are fatal read or parse errors.
func LoadFrontmatter(path string) (Frontmatter, []byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Frontmatter{}, nil, fmt.Errorf("reading %s: %w", path, err)
	}
	fm, err := ParseFrontmatter(content)
	if err != nil {
		return Frontmatter{}, content, err
	}
	return fm, content, nil
}
