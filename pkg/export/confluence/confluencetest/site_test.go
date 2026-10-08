package confluencetest_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
	"github.com/donaldgifford/docz/v2/pkg/export/confluence"
	"github.com/donaldgifford/docz/v2/pkg/export/confluence/confluencetest"
)

// fixture is a two-document repository in the folder layout.
func fixture(t *testing.T) *repo.Repo {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Sync.Confluence = config.ConfluenceSyncConfig{
		Enabled: true, Site: confluencetest.DefaultURL, Space: "DOCZ", Folder: "docz",
		Layout:  config.LayoutFolder,
		Mermaid: config.MermaidSyncConfig{Viewer: config.MermaidViewerAuto},
	}

	rp := &repo.Repo{Root: t.TempDir(), Cfg: &cfg}
	if _, err := rp.Init(t.Context(), repo.InitOptions{}); err != nil {
		t.Fatal(err)
	}

	for name, body := range map[string]string{
		"docs/rfc/0001-first.md":  "---\nid: RFC-0001\ntitle: First\nstatus: Draft\n---\n\n# RFC-0001: First\n\nSee [ADR-0001](../adr/0001-second.md).\n",
		"docs/adr/0001-second.md": "---\nid: ADR-0001\ntitle: Second\nstatus: Proposed\n---\n\n# ADR-0001: Second\n\nText.\n",
	} {
		p := filepath.Join(rp.Root, filepath.FromSlash(name))
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := rp.Update(t.Context(), nil, repo.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	return rp
}

// opts is the server's options over client.
func opts(client confluence.Client) confluence.ExportOptions {
	return confluence.ExportOptions{Client: client, Repository: "o/r", Overwrite: true}
}

// shape is a site's tree as "kind title <- parent title" lines.
func shape(site *confluencetest.Site) []string {
	nodes := site.Pages()
	titles := map[string]string{}

	for _, n := range nodes {
		titles[n.ID] = n.Title
	}

	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		parent := titles[n.ParentID]
		if parent == "" {
			parent = n.ParentID
		}

		out = append(out, n.Type+" "+n.Title+" <- "+parent)
	}

	slices.Sort(out)

	return out
}

// TestHandlerMatchesClient runs one export through the Site as a Client and
// another through HTTPClient over its Handler: both must build the same
// tree, and a second run over HTTP must change nothing.
func TestHandlerMatchesClient(t *testing.T) {
	t.Parallel()

	rp := fixture(t)
	direct := confluencetest.New("DOCZ")
	if _, err := confluence.Export(t.Context(), rp, opts(direct)); err != nil {
		t.Fatal(err)
	}

	served := confluencetest.New("DOCZ")
	served.RequireAuth("me@example.com", "secret")
	hc := served.HTTPClient(t, "me@example.com", "secret")

	rep, err := confluence.Export(t.Context(), rp, opts(hc))
	if err != nil {
		t.Fatal(err)
	}

	if got, want := shape(served), shape(direct); !slices.Equal(got, want) {
		t.Errorf("over HTTP:\n%v\nin memory:\n%v", got, want)
	}

	if rep.Folder == nil || served.FolderByTitle("docz") == nil {
		t.Fatalf("no folder: %+v", rep.Folder)
	}

	served.Writes()

	again, err := confluence.Export(t.Context(), rp, opts(hc))
	if err != nil {
		t.Fatal(err)
	}

	if w := served.Writes(); len(w) != 0 {
		t.Errorf("second run wrote %v", w)
	}

	for _, r := range again.Pages {
		if r.Action != confluence.Unchanged {
			t.Errorf("%s: %s, want unchanged", r.Title, r.Action)
		}
	}
}

// TestHandlerRejectsABadToken: with RequireAuth set, a wrong token is a
// *confluence.AuthError from the real client.
func TestHandlerRejectsABadToken(t *testing.T) {
	t.Parallel()

	site := confluencetest.New("DOCZ")
	site.RequireAuth("me@example.com", "secret")

	_, err := confluence.Export(t.Context(), fixture(t), opts(site.HTTPClient(t, "me@example.com", "wrong")))

	var ae *confluence.AuthError
	if !errors.As(err, &ae) || ae.Status != 401 {
		t.Fatalf("err = %v, want a 401 AuthError", err)
	}
}

// TestFailOn: an injected 500 on one page fails that page and the run goes
// on; Clear lets the next run create it.
func TestFailOn(t *testing.T) {
	t.Parallel()

	rp := fixture(t)
	site := confluencetest.New("DOCZ")
	site.FailOn("create", "docz: RFC-0001: First", 500)

	rep, err := confluence.Export(t.Context(), rp, opts(site.HTTPClient(t, "e", "t")))
	if err == nil {
		t.Fatal("want the failed page's error")
	}

	if rep.Count(confluence.Failed) != 1 {
		t.Errorf("failed = %d, want 1", rep.Count(confluence.Failed))
	}

	site.Clear()

	if _, err := confluence.Export(t.Context(), rp, opts(site)); err != nil {
		t.Fatal(err)
	}

	if site.PageByTitle("docz: RFC-0001: First") == nil {
		t.Error("the page was not created after Clear")
	}
}
