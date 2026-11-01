package consumer

// DESIGN-0025's HTML layer exercised from outside the module: pkg/theme,
// the one design an exported page inlines and docz-site imports, and
// pkg/export/html, which renders a document into a page from bytes alone.
// A consumer rendering its own docz HTML reaches the same stylesheet and
// runtime the docz binary embeds.

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/export/html"
	"github.com/donaldgifford/docz/v2/pkg/theme"
)

func TestExternalConsumerReadsTheThemes(t *testing.T) {
	names := theme.Names()
	if !slices.Equal(names, []string{"paper", "charcoal", "classic"}) {
		t.Fatalf("theme.Names() = %v, want [paper charcoal classic]", names)
	}

	for _, name := range names {
		css, err := theme.CSS(name)
		if err != nil {
			t.Fatalf("theme.CSS(%q): %v", name, err)
		}
		if !strings.Contains(css, `:root[data-theme="`+name+`"]`) {
			t.Errorf("theme.CSS(%q) does not carry its own tokens", name)
		}
	}

	if _, err := theme.CSS("nope"); !errors.Is(err, theme.ErrUnknownTheme) {
		t.Errorf("theme.CSS(nope) error = %v, want ErrUnknownTheme", err)
	}
	if theme.Runtime() == "" {
		t.Error("theme.Runtime() is empty")
	}
}

func TestExternalConsumerRendersAPage(t *testing.T) {
	src := []byte("---\nid: DESIGN-0001\ntitle: \"A design\"\nstatus: Draft\n---\n\n" +
		"# DESIGN-0001: A design\n\n## Overview\n\nSome **text** <script>x</script>.\n\n" +
		"## Goals\n\n- One\n")

	page, err := html.Render(src, html.RenderOptions{Type: "design", Site: "consumer"})
	if err != nil {
		t.Fatalf("Render = %v, want nil", err)
	}

	if page.ID != "DESIGN-0001" || !strings.HasPrefix(page.Hash, "sha256:") {
		t.Errorf("Render = %s %s", page.ID, page.Hash)
	}

	body := string(page.Body)
	for _, want := range []string{"<strong>text</strong>", `data-tab="overview"`, `data-tab="goals"`, "consumer · Design"} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q", want)
		}
	}

	if strings.Contains(body, "<script>x</script>") {
		t.Error("the document's script reached the page")
	}

	if _, err := html.Render([]byte("# no frontmatter\n"), html.RenderOptions{}); !errors.Is(err, html.ErrNoFrontmatter) {
		t.Errorf("Render(no frontmatter) = %v, want ErrNoFrontmatter", err)
	}
}
