package template

import (
	"embed"
	"fmt"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

//go:embed templates/*.md templates/*.tmpl templates/schema/*.md
var templateFS embed.FS

// DefaultTemplateName is the type name whose template and schema scaffold a
// custom type: a body of a ToC pair, a summary, and references, and a schema
// that requires only the ToC and references (DESIGN-0015 §3).
const DefaultTemplateName = "default"

// EmbeddedDocumentTemplate returns the embedded default template for the given
// document type. Valid types: rfc, adr, design, impl, plan, investigation.
//
// docType is the typed config.DocType (DESIGN-0004 §F) so a stray
// status or path string fails to compile here rather than producing a
// confusing "no embedded template for type %q" miss at runtime.
func EmbeddedDocumentTemplate(docType config.DocType) (string, error) {
	data, err := templateFS.ReadFile("templates/" + string(docType) + ".md")
	if err != nil {
		return "", fmt.Errorf("no embedded template for type %q: %w", docType, err)
	}
	return string(data), nil
}

// EmbeddedSchema returns the embedded marker skeleton for the given schema
// name — one per built-in type, plus DefaultTemplateName for a custom type
// that has not scaffolded its own (DESIGN-0015 §3).
//
// A skeleton is a markdown body of nothing but region markers, read by the
// same walker as a document, so there is no schema language and nothing a
// skeleton can require that a document cannot show.
//
// The name is taken as a plain filename, so a caller must not pass a path: a
// name with a separator or a dot segment misses rather than escaping the
// embedded tree, because embed.FS rejects it.
func EmbeddedSchema(name string) (string, error) {
	data, err := templateFS.ReadFile("templates/schema/" + name + ".md")
	if err != nil {
		return "", fmt.Errorf("no embedded schema %q: %w", name, err)
	}

	return string(data), nil
}

// EmbeddedWikiIndex returns the embedded default wiki index template
// used to generate docs/index.md.
func EmbeddedWikiIndex() (string, error) {
	data, err := templateFS.ReadFile("templates/wiki_index.md")
	if err != nil {
		return "", fmt.Errorf("reading embedded wiki index template: %w", err)
	}
	return string(data), nil
}

// EmbeddedDoczYAML returns the embedded text/template source for the
// `.docz.yaml` config file produced by `docz init`. Callers render it
// with `text/template`, passing a `config.Config` as the template data.
func EmbeddedDoczYAML() (string, error) {
	data, err := templateFS.ReadFile("templates/docz_yaml.tmpl")
	if err != nil {
		return "", fmt.Errorf("reading embedded docz yaml template: %w", err)
	}
	return string(data), nil
}
