package consumer

// The Confluence export (IMPL-0023): pkg/export/confluence exercised from
// outside the module. Render proves the pure half is reachable with bytes
// alone, which is what docz-api's job will call; Export proves the
// operation is, against a Client this file defines, so a consumer can
// supply its own transport.

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"

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

// consumerClient is the smallest Client that lets Export create pages and
// a folder.
type consumerClient struct {
	pages   map[string]*confluence.Page
	folders map[string]*confluence.Folder
}

func newConsumerClient() *consumerClient {
	return &consumerClient{pages: make(map[string]*confluence.Page), folders: make(map[string]*confluence.Folder)}
}

func (*consumerClient) SpaceID(context.Context, string) (string, error) { return "S", nil }

func (*consumerClient) SpaceHome(context.Context, string) (string, error) { return "H", nil }

func (c *consumerClient) FindPage(_ context.Context, _, title string) (*confluence.Page, error) {
	return c.pages[title], nil
}

func (*consumerClient) Page(context.Context, string) (*confluence.Page, error) { return nil, nil }

func (*consumerClient) Body(context.Context, string) ([]byte, error) { return nil, nil }

func (*consumerClient) Folder(context.Context, string) (*confluence.Folder, error) { return nil, nil }

func (c *consumerClient) CreateFolder(_ context.Context, f *confluence.NewFolder) (*confluence.Folder, error) {
	folder := &confluence.Folder{ID: "f" + strconv.Itoa(len(c.folders)+1), Title: f.Title, ParentID: f.ParentID, SpaceID: f.SpaceID}
	c.folders[f.Title] = folder

	return folder, nil
}

func (c *consumerClient) CreatePage(_ context.Context, p *confluence.NewPage) (*confluence.Page, error) {
	page := &confluence.Page{ID: strconv.Itoa(len(c.pages) + 1), Title: p.Title, ParentID: p.ParentID, Version: 1}
	c.pages[p.Title] = page

	return page, nil
}

func (*consumerClient) UpdatePage(context.Context, string, *confluence.PageUpdate) (*confluence.Page, error) {
	return nil, context.Canceled
}

func (*consumerClient) Property(context.Context, confluence.Target, string) (*confluence.Property, error) {
	return nil, nil
}

func (*consumerClient) SetProperty(context.Context, confluence.Target, *confluence.Property) error {
	return nil
}

func (*consumerClient) Children(context.Context, confluence.Target) ([]confluence.Node, error) {
	return nil, nil
}

func TestExternalConsumerExportsARepo(t *testing.T) {
	r := repoFixture(t)
	r.Cfg.Sync.Confluence.Enabled = true
	r.Cfg.Sync.Confluence.Space = "DOCZ"
	r.Cfg.Sync.Confluence.Parent = "docz"
	r.Cfg.Sync.Confluence.Layout = config.LayoutPage

	for _, title := range []string{"One", "Two"} {
		if _, err := r.Create(t.Context(), repo.CreateOptions{Type: "adr", Title: title, Author: "T"}); err != nil {
			t.Fatalf("Create = %v, want nil", err)
		}
	}

	report, err := confluence.Export(t.Context(), r, confluence.ExportOptions{
		Client: newConsumerClient(),
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

// TestExternalConsumerExportsAFetchedTree is docz-api's shape: the files
// come from memory, the root points nowhere, and the repository is named,
// so the pages land in its folder under prefixed titles.
func TestExternalConsumerExportsAFetchedTree(t *testing.T) {
	built := repoFixture(t)

	if _, err := built.Create(t.Context(), repo.CreateOptions{Type: "adr", Title: "One", Author: "T", Update: true}); err != nil {
		t.Fatalf("Create = %v, want nil", err)
	}

	files := fstest.MapFS{}

	err := fs.WalkDir(os.DirFS(built.Root), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		b, err := os.ReadFile(filepath.Join(built.Root, filepath.FromSlash(p)))
		files[p] = &fstest.MapFile{Data: b}

		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	cfg := *built.Cfg
	cfg.Sync.Confluence.Enabled = true
	cfg.Sync.Confluence.Space = "DOCZ"
	cfg.Sync.Confluence.Types = []string{"adr"}

	client := newConsumerClient()

	report, err := confluence.Export(t.Context(), &repo.Repo{Root: filepath.Join(t.TempDir(), "absent"), Cfg: &cfg},
		confluence.ExportOptions{Client: client, FS: files, Repository: "acme/widgets", Overwrite: true})
	if err != nil {
		t.Fatalf("Export = %v, want nil", err)
	}

	if report.Folder == nil || report.Folder.Title != "widgets" || client.folders["widgets"].ParentID != "" {
		t.Fatalf("folder %+v; want widgets at the top of the space", report.Folder)
	}

	for _, title := range []string{"widgets", "widgets: ADRs", "widgets: ADR-0001: One"} {
		if p := client.pages[title]; p == nil {
			t.Errorf("no page %q in %v", title, client.pages)
		}
	}

	if p := client.pages["widgets: ADRs"]; p != nil && p.ParentID != report.Folder.ID {
		t.Errorf("the type page is under %q; want the folder", p.ParentID)
	}
}
