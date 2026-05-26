package document

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// docFilePattern matches docz-managed filenames like "0001-some-slug.md".
// Lives in internal/document so a single canonical pattern is shared by
// the scan, README index, and wiki nav builders.
var docFilePattern = regexp.MustCompile(`^\d+-.*\.md$`)

// DocEntry pairs a document's parsed Frontmatter with its source filename
// and the raw file bytes read during scanning.
//
// Content holds the verbatim file contents read by ScanDocuments. It is
// populated unconditionally so downstream callers — notably the cmd/update
// ToC pass — can operate on the bytes without re-reading the file. The
// expected memory ceiling at CLI scale is roughly the on-disk size of the
// scanned directory: a repo with 1000 docs × ~10KB averages 10MB resident,
// which is acceptable for a one-shot CLI. A future library consumer
// running ScanDocuments on very large trees should be aware of this
// assumption.
type DocEntry struct {
	Frontmatter
	Filename string
	Content  []byte
}

// ScanDocuments reads all NNNN-*.md files in dir, parses their YAML
// frontmatter, and returns them sorted by ID. Files without valid
// frontmatter are silently skipped. A missing directory returns
// (nil, nil) — callers usually want that to mean "nothing to do".
func ScanDocuments(dir string) ([]DocEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	var docs []DocEntry
	for _, entry := range entries {
		if entry.IsDir() || !docFilePattern.MatchString(entry.Name()) {
			continue
		}

		fm, content, err := LoadFrontmatter(filepath.Join(dir, entry.Name()))
		if err != nil {
			// Silently skip unreadable files and files without
			// frontmatter; ScanDocuments treats both as "not a
			// docz document". Surfaced as errors by callers that
			// want stricter behavior (none today).
			continue
		}

		docs = append(docs, DocEntry{
			Frontmatter: fm,
			Filename:    entry.Name(),
			Content:     content,
		})
	}

	slices.SortFunc(docs, func(a, b DocEntry) int {
		return strings.Compare(a.ID, b.ID)
	})

	return docs, nil
}
