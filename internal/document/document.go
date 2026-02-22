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

	// Find the closing delimiter.
	rest := content[3:]
	rest = bytes.TrimLeft(rest, " \t")
	if len(rest) == 0 || rest[0] != '\n' {
		return fm, ErrNoFrontmatter
	}
	rest = rest[1:]

	yamlBlock, _, found := bytes.Cut(rest, []byte("\n---"))
	if !found {
		return fm, fmt.Errorf("unterminated YAML frontmatter: missing closing ---")
	}
	if err := yaml.Unmarshal(yamlBlock, &fm); err != nil {
		return fm, fmt.Errorf("parsing YAML frontmatter: %w", err)
	}

	return fm, nil
}
