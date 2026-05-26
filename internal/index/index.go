// Package index provides README index generation for docz document
// directories. Scanning lives in internal/document; this package only
// builds the table markdown and splices it between the BEGIN/END
// markers in each type's README.md.
package index

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/donaldgifford/docz/internal/config"
	"github.com/donaldgifford/docz/internal/document"
	doctemplate "github.com/donaldgifford/docz/internal/template"
)

const (
	beginMarker = "<!-- BEGIN DOCZ AUTO-GENERATED -->"
	endMarker   = "<!-- END DOCZ AUTO-GENERATED -->"
)

// GenerateTable produces a markdown table from a list of document entries.
func GenerateTable(docs []document.DocEntry, heading string) string {
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
		if errors.Is(err, fs.ErrNotExist) {
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

	if err := os.WriteFile(readmePath, []byte(newContent), config.FileMode); err != nil {
		return "", fmt.Errorf("writing %s: %w", readmePath, err)
	}

	return fmt.Sprintf("Updated %s", readmePath), nil
}

// DryRunReadme returns what UpdateReadme would write without modifying files.
func DryRunReadme(readmePath, typeName, tableContent string) (string, error) {
	data, err := os.ReadFile(readmePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
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
	if err := os.MkdirAll(dir, config.DirMode); err != nil {
		return "", fmt.Errorf("creating directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), config.FileMode); err != nil {
		return "", fmt.Errorf("writing %s: %w", path, err)
	}

	return fmt.Sprintf("Created %s", path), nil
}
