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
)

const (
	beginMarker = "<!-- BEGIN DOCZ AUTO-GENERATED -->"
	endMarker   = "<!-- END DOCZ AUTO-GENERATED -->"
)

// UpdateAction names the kind of work UpdateReadme / DryRunReadme
// performed. The cmd layer switches on this value to format the
// user-facing message; index/* never produces English strings.
type UpdateAction int

const (
	// ActionCreated indicates UpdateReadme wrote a brand-new README
	// from the caller-provided index header.
	ActionCreated UpdateAction = iota + 1
	// ActionUpdated indicates UpdateReadme found existing markers
	// and rewrote the auto-generated table between them.
	ActionUpdated
	// ActionNoMarkers indicates the target README exists but has no
	// DOCZ auto-generated markers; the file is left unchanged.
	ActionNoMarkers
	// ActionDryRunCreated is the dry-run analogue of ActionCreated.
	// Body holds the README content that would have been written.
	ActionDryRunCreated
	// ActionDryRunUpdated is the dry-run analogue of ActionUpdated.
	// Body holds the README content that would have been written.
	ActionDryRunUpdated
)

// UpdateOutcome is the typed result of UpdateReadme / DryRunReadme. The
// Action discriminator drives caller-side message formatting; Body is
// populated for the two dry-run actions so the cmd layer can print the
// would-be content.
type UpdateOutcome struct {
	Action UpdateAction
	Path   string
	Body   string
}

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
// the DOCZ markers. If the file doesn't exist, it is created with the
// provided header (Action=ActionCreated). If it exists with markers, the
// table is rewritten (Action=ActionUpdated). If it exists without markers,
// nothing is written and Action=ActionNoMarkers — the caller decides how to
// surface that. The header is resolved by the caller (see
// template.ResolveIndexHeader) so this package stays a pure marker-splicer.
func UpdateReadme(readmePath, header, tableContent string) (UpdateOutcome, error) {
	data, err := os.ReadFile(readmePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return createNewReadme(readmePath, header, tableContent)
		}
		return UpdateOutcome{}, fmt.Errorf("reading %s: %w", readmePath, err)
	}

	content := string(data)
	newContent, ok := spliceMarkers(content, tableContent)
	if !ok {
		return UpdateOutcome{Action: ActionNoMarkers, Path: readmePath}, nil
	}

	if err := os.WriteFile(readmePath, []byte(newContent), config.FileMode); err != nil {
		return UpdateOutcome{}, fmt.Errorf("writing %s: %w", readmePath, err)
	}

	return UpdateOutcome{Action: ActionUpdated, Path: readmePath}, nil
}

// DryRunReadme returns what UpdateReadme would write without modifying
// files. Action is one of ActionDryRunCreated, ActionDryRunUpdated, or
// ActionNoMarkers; Body holds the would-be content for the two dry-run
// success cases.
func DryRunReadme(readmePath, header, tableContent string) (UpdateOutcome, error) {
	data, err := os.ReadFile(readmePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			body := header + beginMarker + "\n" + tableContent + endMarker + "\n"
			return UpdateOutcome{
				Action: ActionDryRunCreated,
				Path:   readmePath,
				Body:   body,
			}, nil
		}
		return UpdateOutcome{}, fmt.Errorf("reading %s: %w", readmePath, err)
	}

	content := string(data)
	body, ok := spliceMarkers(content, tableContent)
	if !ok {
		return UpdateOutcome{Action: ActionNoMarkers, Path: readmePath}, nil
	}

	return UpdateOutcome{
		Action: ActionDryRunUpdated,
		Path:   readmePath,
		Body:   body,
	}, nil
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

func createNewReadme(path, header, tableContent string) (UpdateOutcome, error) {
	content := header + beginMarker + "\n" + tableContent + endMarker + "\n"

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, config.DirMode); err != nil {
		return UpdateOutcome{}, fmt.Errorf("creating directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), config.FileMode); err != nil {
		return UpdateOutcome{}, fmt.Errorf("writing %s: %w", path, err)
	}

	return UpdateOutcome{Action: ActionCreated, Path: path}, nil
}
