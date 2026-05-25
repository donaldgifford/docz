// Package document provides frontmatter parsing and document creation.
package document

import (
	"bytes"
	"errors"
	"fmt"

	"go.yaml.in/yaml/v3"
)

// Frontmatter holds the YAML frontmatter metadata from a document file.
type Frontmatter struct {
	ID      string `yaml:"id"`
	Title   string `yaml:"title"`
	Status  string `yaml:"status"`
	Author  string `yaml:"author"`
	Created string `yaml:"created"`
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
