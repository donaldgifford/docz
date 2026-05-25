package template

import (
	"embed"
	"fmt"
)

//go:embed templates/*.md templates/*.tmpl
var templateFS embed.FS

// EmbeddedDocumentTemplate returns the embedded default template for the given
// document type. Valid types: rfc, adr, design, impl.
func EmbeddedDocumentTemplate(docType string) (string, error) {
	data, err := templateFS.ReadFile("templates/" + docType + ".md")
	if err != nil {
		return "", fmt.Errorf("no embedded template for type %q: %w", docType, err)
	}
	return string(data), nil
}

// EmbeddedIndexHeader returns the embedded default index header template for
// the given document type. Valid types: rfc, adr, design, impl.
func EmbeddedIndexHeader(docType string) (string, error) {
	data, err := templateFS.ReadFile("templates/index_" + docType + ".md")
	if err != nil {
		return "", fmt.Errorf("no embedded index header for type %q: %w", docType, err)
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
