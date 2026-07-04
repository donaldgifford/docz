package docwrite

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	doctemplate "github.com/donaldgifford/docz/internal/template"
	"github.com/donaldgifford/docz/pkg/doczcore/config"
	"github.com/donaldgifford/docz/pkg/doczcore/document"
)

// CreateOptions holds the inputs for creating a new document.
//
// Type is the typed config.DocType wrapper (DESIGN-0004 §F) so a
// stray status or title string in its place is a compile error.
// Status stays plain string here: it flows in from a CLI flag where
// the value is whatever the user typed, and is validated against
// TypeConfig.Statuses higher up the stack.
type CreateOptions struct {
	Type    config.DocType // Document type (rfc, adr, design, impl, ...)
	Title   string         // Document title
	Author  string         // Author name
	Status  string         // Initial status
	Prefix  string         // ID prefix (e.g., "RFC")
	IDWidth int            // Zero-pad width (e.g., 4)
	DocsDir string         // Base docs directory
	TypeDir string         // Type subdirectory relative to DocsDir

	// TemplatePath is an explicit template file path from config. Empty uses
	// the default resolution (local override > embedded).
	TemplatePath string

	// CreatedAt is the document creation timestamp written to the
	// rendered template's `Date` field. Zero value falls back to
	// time.Now() so callers without a time source still get the
	// expected behavior; cmd/create.go populates this from
	// runner.Now() so tests can pin time without touching package
	// globals.
	CreatedAt time.Time
}

// CreateResult contains the output from a successful document creation.
type CreateResult struct {
	FilePath string // Path to the created file
	Number   string // Zero-padded document number
	Filename string // Filename only (e.g., "0001-my-doc.md")
}

// Create generates a new document from a template and writes it to the
// appropriate directory with an auto-incremented ID.
func Create(opts *CreateOptions) (CreateResult, error) {
	dir := filepath.Join(opts.DocsDir, opts.TypeDir)

	if err := os.MkdirAll(dir, config.DirMode); err != nil {
		return CreateResult{}, fmt.Errorf("creating directory %s: %w", dir, err)
	}

	number := fmt.Sprintf("%0*d", opts.IDWidth, nextID(dir))
	slug := doctemplate.FilenameSlug(opts.Title)
	filename := number + "-" + slug + ".md"
	filePath := filepath.Join(dir, filename)

	if _, statErr := os.Stat(filePath); statErr == nil {
		return CreateResult{}, fmt.Errorf("file already exists: %s", filePath)
	}

	tmplContent, err := doctemplate.Resolve(string(opts.Type), opts.TemplatePath, opts.DocsDir)
	if err != nil {
		return CreateResult{}, fmt.Errorf("resolving template: %w", err)
	}

	createdAt := opts.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	data := doctemplate.Data{
		Number:   number,
		Title:    opts.Title,
		Date:     createdAt.Format(time.DateOnly),
		Author:   opts.Author,
		Status:   config.Status(opts.Status),
		Type:     opts.Type,
		Prefix:   opts.Prefix,
		Slug:     slug,
		Filename: filename,
	}

	rendered, err := doctemplate.Render(tmplContent, &data)
	if err != nil {
		return CreateResult{}, fmt.Errorf("rendering template: %w", err)
	}

	if err := os.WriteFile(filePath, []byte(rendered), config.FileMode); err != nil {
		return CreateResult{}, fmt.Errorf("writing file: %w", err)
	}

	return CreateResult{
		FilePath: filePath,
		Number:   number,
		Filename: filename,
	}, nil
}

// nextID scans the directory for existing NNNN-*.md files and returns the
// next sequential ID number.
func nextID(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 1 // empty or missing directory starts at 1
	}

	maxID := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := document.DoczFilePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}
		id, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}
		if id > maxID {
			maxID = id
		}
	}

	return maxID + 1
}
