// Package template provides embedded templates, resolution, and rendering.
package template

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/donaldgifford/docz/internal/config"
)

// TemplateData holds all variables available for template rendering.
type TemplateData struct {
	Number   string // Zero-padded document ID (e.g., "0001")
	Title    string // Document title as provided
	Date     string // Creation date (YYYY-MM-DD)
	Author   string // Author name
	Status   string // Initial status
	Type     string // Document type (e.g., "RFC")
	Prefix   string // ID prefix from config (e.g., "RFC")
	Slug     string // Kebab-case title
	Filename string // Generated filename (e.g., "0001-api-rate-limiting.md")
}

var (
	nonSlugChar     = regexp.MustCompile(`[^a-z0-9-]`)
	multipleHyphens = regexp.MustCompile(`-{2,}`)
)

// maxSlugLength caps generated slugs at 64 characters so resulting filenames
// stay comfortably below typical filesystem path limits when combined with the
// document ID prefix and directory path.
const maxSlugLength = 64

// FilenameSlug converts a document title into a kebab-case identifier
// suitable for use in a filename (e.g. "API Rate Limiting!" →
// "api-rate-limiting"). It is paired with the document ID and used by
// internal/document/create.go to produce names like
// "0001-api-rate-limiting.md".
//
// Transformation: lowercase, spaces to hyphens, strip every character
// that is not a-z / 0-9 / hyphen, collapse runs of hyphens, trim
// leading/trailing hyphens, then truncate to 64 characters on a word
// boundary (last hyphen before the cap).
//
// This is distinct from toc.AnchorSlug, which generates GitHub-style
// markdown anchor slugs and preserves spaces as hyphens differently.
func FilenameSlug(title string) string {
	s := strings.ToLower(title)
	s = strings.ReplaceAll(s, " ", "-")
	s = nonSlugChar.ReplaceAllString(s, "")
	s = multipleHyphens.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	if len(s) > maxSlugLength {
		s = s[:maxSlugLength]
		// Trim at last hyphen to avoid cutting mid-word.
		if idx := strings.LastIndex(s, "-"); idx > 0 {
			s = s[:idx]
		}
	}

	return s
}

// Resolve returns the template content for the given document type, checking
// override sources in order:
//  1. Explicit path from config (configPath)
//  2. Local override file at docsDir/templates/<docType>.md
//  3. Embedded default template
func Resolve(docType, configPath, docsDir string) (string, error) {
	// 1. Explicit config path.
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return "", fmt.Errorf("reading template from config path %q: %w", configPath, err)
		}
		return string(data), nil
	}

	// 2. Local override.
	localPath := filepath.Join(docsDir, config.TemplatesDir, docType+".md")
	if data, err := os.ReadFile(localPath); err == nil {
		return string(data), nil
	}

	// 3. Embedded default.
	return EmbeddedDocumentTemplate(docType)
}

// WikiIndexType represents a single document type entry for the wiki index template.
type WikiIndexType struct {
	Name     string // canonical type name (e.g., "rfc")
	NavTitle string // display name (e.g., "RFCs")
	Dir      string // directory relative to docs_dir (e.g., "rfc")
}

// WikiIndexData holds variables for the wiki index template.
type WikiIndexData struct {
	SiteName string
	Types    []WikiIndexType
}

// ResolveWikiIndex returns the wiki index template content, checking for a
// local override at <docsDir>/templates/wiki_index.md before falling back to
// the embedded default.
func ResolveWikiIndex(docsDir string) (string, error) {
	localPath := filepath.Join(docsDir, config.TemplatesDir, "wiki_index.md")
	if data, err := os.ReadFile(localPath); err == nil {
		return string(data), nil
	}

	return EmbeddedWikiIndex()
}

// RenderWikiIndex executes the wiki index template with the provided data.
func RenderWikiIndex(tmplContent string, data *WikiIndexData) (string, error) {
	t, err := template.New("wiki_index").Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("parsing wiki index template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing wiki index template: %w", err)
	}

	return buf.String(), nil
}

// Render executes a Go text/template with the provided data and returns the
// rendered output.
func Render(tmplContent string, data *TemplateData) (string, error) {
	t, err := template.New("doc").Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}
