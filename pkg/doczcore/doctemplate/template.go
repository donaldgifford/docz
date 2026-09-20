// Package doctemplate provides docz's embedded templates, the resolution order
// that lets a repo override them, and the rendering that turns one into a
// document.
//
// Promoted whole from internal/template (IMPL-0018 Phase 2, DESIGN-0014 §2.7).
// The embed.FS stays unexported and the template *contents* are outside the
// semver contract: they are observable through `docz template show`, and a
// consumer that wants a template's text calls Resolve and treats the result as
// data. What is governed is the resolution order, the render data, and the
// errors.
package doctemplate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// Data holds all variables available for document-template rendering.
// (Renamed from TemplateData in IMPL-0008 Phase 10 — the package
// qualifier `template.Data` reads better than the stuttering
// `template.TemplateData`.)
//
// Type and Status are typed wrappers (DESIGN-0004 §F) so a stray
// title or filename string in their place fails to compile. The
// underlying kind is string, so text/template renders them via the
// underlying value without a Stringer; no template-syntax change.
type Data struct {
	Number   string         // Zero-padded document ID (e.g., "0001")
	Title    string         // Document title as provided
	Date     string         // Creation date (YYYY-MM-DD)
	Author   string         // Author name
	Status   config.Status  // Initial status
	Type     config.DocType // Document type (e.g., "RFC")
	Prefix   string         // ID prefix from config (e.g., "RFC")
	Slug     string         // Kebab-case title
	Filename string         // Generated filename (e.g., "0001-api-rate-limiting.md")
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
//
// A type with none of the three is ErrNoTemplate, wrapped with the type name
// and the on-disk path that was looked for. That is issue #92: a custom type
// whose config names no template, or names one that is not there, used to fail
// with the embedded FS's "file does not exist" — an error that told a person
// neither which type was wrong nor where to put the file. It is also what
// ADR-0003's retired plan: block hits once the type leaves the registry.
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
	body, err := EmbeddedDocumentTemplate(config.DocType(docType))
	if err != nil {
		return "", fmt.Errorf(
			"%w: type %q has no embedded template, no template: path in config, "+
				"and no file at %s", ErrNoTemplate, docType, localPath)
	}

	return body, nil
}

// IndexHeaderData is the render context for the generic fallback index
// header (templates/index_default.md). Type-specific embedded headers and
// on-disk overrides are emitted verbatim and ignore this data.
type IndexHeaderData struct {
	TypeName    string // canonical type name, e.g. "frameworks"
	PluralLabel string // display label, e.g. "Frameworks"
}

// ResolveIndexHeader returns the index-header prose for docType — the prose
// spliced above the auto-generated table in a type's README — checking
// override sources in order:
//  1. On-disk override at <docsDir>/templates/index_<docType>.md (verbatim)
//  2. Embedded type-specific header templates/index_<docType>.md (verbatim)
//  3. Embedded generic header templates/index_default.md (rendered with data)
//
// Only tier 3 is rendered through text/template; tiers 1 and 2 are returned
// byte-for-byte so a literal "{{" in a user override or a built-in header is
// never reinterpreted (DESIGN-0006 Decision 2). This mirrors Resolve's
// disk-override → embedded fallback shape for body templates, closing the
// asymmetry where index headers were embedded-only.
func ResolveIndexHeader(docType, docsDir string, data IndexHeaderData) (string, error) {
	// 1. On-disk override.
	localPath := filepath.Join(docsDir, config.TemplatesDir, "index_"+docType+".md")
	if b, err := os.ReadFile(localPath); err == nil {
		return string(b), nil
	}

	// 2. Embedded type-specific header (hits for the six built-ins).
	if b, err := templateFS.ReadFile("templates/index_" + docType + ".md"); err == nil {
		return string(b), nil
	}

	// 3. Embedded generic fallback, rendered with data.
	b, err := templateFS.ReadFile("templates/index_default.md")
	if err != nil {
		return "", fmt.Errorf("reading embedded default index header: %w", err)
	}
	return renderIndexHeader(string(b), data)
}

// renderIndexHeader executes the generic index-header template with data.
func renderIndexHeader(tmplContent string, data IndexHeaderData) (string, error) {
	t, err := template.New("index_header").Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("parsing index header template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing index header template: %w", err)
	}

	return buf.String(), nil
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
func Render(tmplContent string, data *Data) (string, error) {
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
