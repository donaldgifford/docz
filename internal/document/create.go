package document

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	doctemplate "github.com/donaldgifford/docz/internal/template"
)

// CreateOptions holds the inputs for creating a new document.
type CreateOptions struct {
	Type    string // Document type (rfc, adr, design, impl)
	Title   string // Document title
	Author  string // Author name
	Status  string // Initial status
	Prefix  string // ID prefix (e.g., "RFC")
	IDWidth int    // Zero-pad width (e.g., 4)
	DocsDir string // Base docs directory
	TypeDir string // Type subdirectory relative to DocsDir

	// TemplatePath is an explicit template file path from config. Empty uses
	// the default resolution (local override > embedded).
	TemplatePath string
}

// CreateResult contains the output from a successful document creation.
type CreateResult struct {
	FilePath string // Path to the created file
	Number   string // Zero-padded document number
	Filename string // Filename only (e.g., "0001-my-doc.md")
}

var idPattern = regexp.MustCompile(`^(\d+)-.*\.md$`)

// Create generates a new document from a template and writes it to the
// appropriate directory with an auto-incremented ID.
func Create(opts CreateOptions) (CreateResult, error) {
	dir := filepath.Join(opts.DocsDir, opts.TypeDir)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return CreateResult{}, fmt.Errorf("creating directory %s: %w", dir, err)
	}

	nextID, err := nextID(dir)
	if err != nil {
		return CreateResult{}, err
	}

	number := fmt.Sprintf("%0*d", opts.IDWidth, nextID)
	slug := doctemplate.Slugify(opts.Title)
	filename := number + "-" + slug + ".md"
	filePath := filepath.Join(dir, filename)

	if _, statErr := os.Stat(filePath); statErr == nil {
		return CreateResult{}, fmt.Errorf("file already exists: %s", filePath)
	}

	tmplContent, err := doctemplate.Resolve(opts.Type, opts.TemplatePath, opts.DocsDir)
	if err != nil {
		return CreateResult{}, fmt.Errorf("resolving template: %w", err)
	}

	data := doctemplate.TemplateData{
		Number:   number,
		Title:    opts.Title,
		Date:     currentDate(),
		Author:   opts.Author,
		Status:   opts.Status,
		Type:     opts.Type,
		Prefix:   opts.Prefix,
		Slug:     slug,
		Filename: filename,
	}

	rendered, err := doctemplate.Render(tmplContent, data)
	if err != nil {
		return CreateResult{}, fmt.Errorf("rendering template: %w", err)
	}

	if err := os.WriteFile(filePath, []byte(rendered), 0o644); err != nil {
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
func nextID(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 1, nil //nolint:nilerr // empty or missing directory starts at 1
	}

	maxID := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := idPattern.FindStringSubmatch(entry.Name())
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

	return maxID + 1, nil
}

func currentDate() string {
	return fmt.Sprintf("%d-%02d-%02d",
		timeNow().Year(), timeNow().Month(), timeNow().Day())
}
