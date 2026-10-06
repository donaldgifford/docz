package consumer

// The Confluence export (IMPL-0023): pkg/export/confluence exercised from
// outside the module. Render proves the pure half is reachable with bytes
// alone, which is what docz-api's job will call; Export proves the
// operation is, against a Client this file defines, so a consumer can
// supply its own transport.

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
	"github.com/donaldgifford/docz/v2/pkg/export/confluence"
)

func TestExternalConsumerRendersADocument(t *testing.T) {
	src := []byte("---\nid: RFC-0001\ntitle: \"A proposal\"\nstatus: Draft\n---\n\n# RFC-0001: A proposal\n\n## Summary\n\nSome **text**.\n")

	got, err := confluence.Render(src, confluence.RenderOptions{})
	if err != nil {
		t.Fatalf("Render = %v, want nil", err)
	}

	if got.ID != "RFC-0001" || got.Title != "RFC-0001: A proposal" || !strings.HasPrefix(got.Hash, "sha256:") {
		t.Errorf("Render = %s %q %s", got.ID, got.Title, got.Hash)
	}

	if !strings.Contains(string(got.Body), "<strong>text</strong>") {
		t.Errorf("body lacks the bold text:\n%s", got.Body)
	}
}

// consumerClient is the smallest Client that lets Export create pages.
type consumerClient struct {
	pages map[string]*confluence.Page
}

func (*consumerClient) SpaceID(context.Context, string) (string, error) { return "S", nil }

func (c *consumerClient) FindPage(_ context.Context, _, title string) (*confluence.Page, error) {
	return c.pages[title], nil
}

func (c *consumerClient) CreatePage(_ context.Context, p *confluence.NewPage) (*confluence.Page, error) {
	page := &confluence.Page{ID: strconv.Itoa(len(c.pages) + 1), Title: p.Title, ParentID: p.ParentID, Version: 1}
	c.pages[p.Title] = page

	return page, nil
}

func (*consumerClient) UpdatePage(context.Context, string, *confluence.PageUpdate) (*confluence.Page, error) {
	return nil, context.Canceled
}

func (*consumerClient) Property(context.Context, string, string) (*confluence.Property, error) {
	return nil, nil
}

func (*consumerClient) SetProperty(context.Context, string, *confluence.Property) error { return nil }

func (*consumerClient) Children(context.Context, string) ([]confluence.Page, error) { return nil, nil }

func TestExternalConsumerExportsARepo(t *testing.T) {
	r := repoFixture(t)
	r.Cfg.Sync.Confluence.Enabled = true
	r.Cfg.Sync.Confluence.Space = "DOCZ"
	r.Cfg.Sync.Confluence.Parent = "docz"

	for _, title := range []string{"One", "Two"} {
		if _, err := r.Create(t.Context(), repo.CreateOptions{Type: "adr", Title: title, Author: "T"}); err != nil {
			t.Fatalf("Create = %v, want nil", err)
		}
	}

	report, err := confluence.Export(t.Context(), r, confluence.ExportOptions{
		Client: &consumerClient{pages: make(map[string]*confluence.Page)},
		Types:  []string{"adr"},
	})
	if err != nil {
		t.Fatalf("Export = %v, want nil", err)
	}

	var docs int

	for _, p := range report.Pages {
		if p.Action != confluence.Created {
			t.Errorf("%s: %s, want created", p.Title, p.Action)
		}

		if p.ID != "" {
			docs++
		}
	}

	if docs != 2 {
		t.Errorf("%d documents created, want 2", docs)
	}
}
