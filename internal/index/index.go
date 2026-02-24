// Package index provides document scanning and README index generation.
package index

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/donaldgifford/docz/internal/document"
	doctemplate "github.com/donaldgifford/docz/internal/template"
)

const (
	beginMarker = "<!-- BEGIN DOCZ AUTO-GENERATED -->"
	endMarker   = "<!-- END DOCZ AUTO-GENERATED -->"
)

var docFilePattern = regexp.MustCompile(`^\d+-.*\.md$`)

// DocEntry holds the frontmatter plus the source filename for a single
// document found during a directory scan.
type DocEntry struct {
	document.Frontmatter
	Filename string
}

// ScanDocuments reads all NNNN-*.md files in dir, parses their YAML
// frontmatter, and returns them sorted by ID. Files without valid
// frontmatter are silently skipped.
func ScanDocuments(dir string) ([]DocEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	var docs []DocEntry
	for _, entry := range entries {
		if entry.IsDir() || !docFilePattern.MatchString(entry.Name()) {
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		fm, err := document.ParseFrontmatter(content)
		if err != nil {
			continue // silently skip files without valid frontmatter
		}

		docs = append(docs, DocEntry{
			Frontmatter: fm,
			Filename:    entry.Name(),
		})
	}

	sort.Slice(docs, func(i, j int) bool {
		return docs[i].ID < docs[j].ID
	})

	return docs, nil
}

// GenerateTable produces a markdown table from a list of document entries.
func GenerateTable(docs []DocEntry, heading string) string {
	var sb strings.Builder

	sb.WriteString("## " + heading + "\n\n")
	sb.WriteString("| ID | Title | Status | Date | Author | Link |\n")
	sb.WriteString("|----|-------|--------|------|--------|------|\n")

	for _, doc := range docs {
		fmt.Fprintf(&sb, "| %s | %s | %s | %s | %s | [%s](%s) |\n",
			doc.ID, doc.Title, doc.Status, doc.Created, doc.Author,
			doc.Filename, doc.Filename)
	}

	return sb.String()
}

// UpdateReadme updates the auto-generated section of a README file between
// the DOCZ markers. If the file exists but has no markers, it is not modified
// and a warning is returned. If the file doesn't exist, it is created with
// the default index header.
func UpdateReadme(readmePath, typeName, tableContent string) (string, error) {
	data, err := os.ReadFile(readmePath)
	if err != nil {
		if os.IsNotExist(err) {
			return createNewReadme(readmePath, typeName, tableContent)
		}
		return "", fmt.Errorf("reading %s: %w", readmePath, err)
	}

	content := string(data)
	newContent, ok := spliceMarkers(content, tableContent)
	if !ok {
		msg := fmt.Sprintf("Warning: %s has no DOCZ auto-generated markers. "+
			"Run 'docz init --force' or manually add markers to update it.", readmePath)
		return msg, nil
	}

	if err := os.WriteFile(readmePath, []byte(newContent), 0o644); err != nil {
		return "", fmt.Errorf("writing %s: %w", readmePath, err)
	}

	return fmt.Sprintf("Updated %s", readmePath), nil
}

// DryRunReadme returns what UpdateReadme would write without modifying files.
func DryRunReadme(readmePath, typeName, tableContent string) (string, error) {
	data, err := os.ReadFile(readmePath)
	if err != nil {
		if os.IsNotExist(err) {
			header, headerErr := doctemplate.EmbeddedIndexHeader(typeName)
			if headerErr != nil {
				return "", headerErr
			}
			return header + beginMarker + "\n" + tableContent + endMarker + "\n", nil
		}
		return "", fmt.Errorf("reading %s: %w", readmePath, err)
	}

	content := string(data)
	result, ok := spliceMarkers(content, tableContent)
	if !ok {
		return fmt.Sprintf("Warning: %s has no DOCZ markers, would be skipped.", readmePath), nil
	}

	return result, nil
}

// spliceMarkers replaces the content between the begin and end markers with
// new table content. Returns the new content and true if markers were found.
func spliceMarkers(content, tableContent string) (string, bool) {
	before, afterBegin, foundBegin := strings.Cut(content, beginMarker)
	if !foundBegin {
		return "", false
	}
	_, afterEnd, foundEnd := strings.Cut(afterBegin, endMarker)
	if !foundEnd {
		return "", false
	}
	return before + beginMarker + "\n" + tableContent + endMarker + afterEnd, true
}

func createNewReadme(path, typeName, tableContent string) (string, error) {
	header, err := doctemplate.EmbeddedIndexHeader(typeName)
	if err != nil {
		return "", fmt.Errorf("loading index header for %s: %w", typeName, err)
	}

	content := header + beginMarker + "\n" + tableContent + endMarker + "\n"

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("creating directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("writing %s: %w", path, err)
	}

	return fmt.Sprintf("Created %s", path), nil
}
