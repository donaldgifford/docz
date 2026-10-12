package doctemplate

import (
	"io/fs"
	"strings"
	"testing"
	"unicode/utf8"
)

// maxTemplateLine is markdownlint MD013's default line length.
const maxTemplateLine = 80

// TestEmbeddedTemplatesFitMD013 keeps every line docz writes from its own
// templates within MD013's default, so a document created by docz passes a
// repository that leaves line length on (#127). Table rows and fenced
// lines are exempt, as MD013's own defaults leave a table's width to its
// cells; the docz_yaml.tmpl config is not markdown.
func TestEmbeddedTemplatesFitMD013(t *testing.T) {
	t.Parallel()

	paths, err := fs.Glob(templateFS, "templates/*.md")
	if err != nil {
		t.Fatal(err)
	}
	schemas, err := fs.Glob(templateFS, "templates/schema/*.md")
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range append(paths, schemas...) {
		body, err := fs.ReadFile(templateFS, path)
		if err != nil {
			t.Fatal(err)
		}

		inFence := false
		for i, line := range strings.Split(string(body), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") {
				inFence = !inFence
				continue
			}
			if inFence || strings.HasPrefix(trimmed, "|") {
				continue
			}
			if n := utf8.RuneCountInString(line); n > maxTemplateLine {
				t.Errorf("%s:%d is %d characters, over %d: %s", path, i+1, n, maxTemplateLine, line)
			}
		}
	}
}
