package template

import (
	"embed"
	"fmt"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
)

//go:embed templates/*.md templates/*.tmpl
var templateFS embed.FS

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
