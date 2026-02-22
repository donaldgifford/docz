package template

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"
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
	nonAlphanumHyphen = regexp.MustCompile(`[^a-z0-9-]`)
	multipleHyphens   = regexp.MustCompile(`-{2,}`)
)

// Slugify converts a title to kebab-case.
//
// Transformation: lowercase, spaces to hyphens, strip non-alphanumeric
// characters (except hyphens), collapse multiple hyphens, trim
// leading/trailing hyphens.
func Slugify(title string) string {
	s := strings.ToLower(title)
	s = strings.ReplaceAll(s, " ", "-")
	s = nonAlphanumHyphen.ReplaceAllString(s, "")
	s = multipleHyphens.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
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
	localPath := docsDir + "/templates/" + docType + ".md"
	if data, err := os.ReadFile(localPath); err == nil {
		return string(data), nil
	}

	// 3. Embedded default.
	return EmbeddedDocumentTemplate(docType)
}

// Render executes a Go text/template with the provided data and returns the
// rendered output.
func Render(tmplContent string, data TemplateData) (string, error) {
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
