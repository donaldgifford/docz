//go:build integration

package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/donaldgifford/docz-api/internal/authorize"
	"github.com/donaldgifford/docz-api/internal/httpapi"
	"github.com/donaldgifford/docz-api/internal/ingest"
	"github.com/donaldgifford/docz-api/internal/store"
)

// pageHit is the slice of the search wire shape the pages e2e asserts on.
type pageHit struct {
	Source string `json:"source"`
	Repo   string `json:"repo"`
	Path   string `json:"path"`
}

// searchHits runs a query through a search-enabled mux and returns the hits.
func searchHits(t *testing.T, mux http.Handler, q string) []pageHit {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/search?q="+q, http.NoBody)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("search %q status = %d (body %q)", q, rec.Code, rec.Body.String())
	}
	var body struct {
		Hits []pageHit `json:"hits"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode search response %q: %v", rec.Body.String(), err)
	}
	return body.Hits
}

// TestE2ERepoPagesServeAndDisable proves the api: block end to end
// (IMPL-0007, the TestE2ERepoChangelogServeAndDisable shape): onboarding a
// repo with an enabled block through the real pipeline (real Postgres + real
// Meilisearch) lists and serves its pages — a directory page, a file page,
// and an additional doc — and makes them findable in search with
// source "page". Disabling the block at HEAD deletes the rows (list empty,
// every path 404) and purges the pages from the index.
func TestE2ERepoPagesServeAndDisable(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // hermetic doczcfg.Load
	ctx := t.Context()

	meili := startMeili(t)

	const instID int64 = 904
	if err := testStore.UpsertInstallation(ctx, store.InstallationInput{
		ID: instID, AccountLogin: fixtureOwner, AccountType: "Organization",
	}); err != nil {
		t.Fatalf("seed installation: %v", err)
	}
	run := func(snap *ingest.RepoSnapshot) {
		t.Helper()
		if _, err := ingest.NewService(testStore, staticFetcher{snap: snap}, meili).
			Run(ctx, instID, fixtureOwner, "paged"); err != nil {
			t.Fatalf("ingest acme/paged: %v", err)
		}
	}

	const configWithAPI = fixtureConfig + `api:
  enabled: true
  additional_docs: [CONTRIBUTING.md]
`
	blobs := []ingest.BlobEntry{
		{Path: "docs/frameworks/0001-intro.md", GitSHA: "g1", Content: doc("FW-0001", "Intro", "# Intro")},
		// IMPL-0009: the enabled type dir's own README publishes nothing —
		// docz-site owns the type surface at /:owner/:repo/frameworks, so a
		// page row here would duplicate it. Its distinctive word (Pangolin)
		// is what the search assertion below hunts for.
		{Path: "docs/frameworks/README.md", GitSHA: "g5", Content: []byte("# Frameworks\n\nPangolin index table.\n")},
		{Path: "docs/guides/README.md", GitSHA: "g2", Content: []byte("# Guides\n\nZebra guide directory.\n")},
		{Path: "docs/examples/example1.md", GitSHA: "g3", Content: []byte("# Example One\n\nQuokka walkthrough example.\n")},
		{Path: "CONTRIBUTING.md", GitSHA: "g4", Content: []byte("# Contributing\n\nWombat contribution rules.\n")},
	}
	run(&ingest.RepoSnapshot{
		HeadSHA: "h1", DefaultBranch: "main", ConfigYAML: []byte(configWithAPI), Blobs: blobs,
	})

	searchMux := chi.NewRouter()
	httpapi.NewHandlerWithSearch(testStore, meili).
		Mount(searchMux, authorize.Middleware(authorize.NewAllReposAuthorizer(testStore)))

	t.Run("pages listed and served", func(t *testing.T) {
		var list struct {
			Pages []struct {
				Path  string `json:"path"`
				Title string `json:"title"`
			} `json:"pages"`
		}
		if code := getJSON(t, "/api/v1/repos/acme/paged/pages", &list); code != http.StatusOK {
			t.Fatalf("list pages status = %d", code)
		}
		if len(list.Pages) != 3 {
			t.Fatalf("pages = %+v, want 3 (additional doc, file page, directory page)", list.Pages)
		}
		// ListRepoPages orders by path.
		wantPaths := []string{"CONTRIBUTING.md", "examples/example1.md", "guides"}
		for i, want := range wantPaths {
			if list.Pages[i].Path != want {
				t.Errorf("pages[%d].path = %q, want %q", i, list.Pages[i].Path, want)
			}
		}

		for path, wantRaw := range map[string]string{
			"guides":                 "# Guides\n\nZebra guide directory.\n",           // directory page via its README
			"examples%2Fexample1.md": "# Example One\n\nQuokka walkthrough example.\n", // file page, percent-encoded
			"CONTRIBUTING.md":        "# Contributing\n\nWombat contribution rules.\n", // additional doc
		} {
			var page struct {
				Repo  string `json:"repo"`
				RawMD string `json:"raw_md"`
			}
			if code := getJSON(t, "/api/v1/repos/acme/paged/pages/"+path, &page); code != http.StatusOK {
				t.Fatalf("get page %q status = %d", path, code)
			}
			if page.Repo != "acme/paged" || page.RawMD != wantRaw {
				t.Errorf("page %q = %+v, want the ingested markdown", path, page)
			}
		}
	})

	// IMPL-0009: the enabled type dir publishes nothing. Its README is
	// ingested and classified, but never becomes a page row — the list above
	// already proves the count, and these prove the path and the index.
	t.Run("type dir publishes no page", func(t *testing.T) {
		if code := getJSON(t, "/api/v1/repos/acme/paged/pages/frameworks", nil); code != http.StatusNotFound {
			t.Errorf("type dir page status = %d, want 404 (type dirs publish nothing)", code)
		}
		for _, h := range searchHits(t, searchMux, "pangolin") {
			if h.Source == "page" {
				t.Errorf("type-dir README indexed as a page: %+v", h)
			}
		}
	})

	t.Run("pages findable in search with source page", func(t *testing.T) {
		for term, wantPath := range map[string]string{
			"wombat": "CONTRIBUTING.md",
			"quokka": "examples/example1.md",
		} {
			hits := searchHits(t, searchMux, term)
			if len(hits) == 0 {
				t.Fatalf("no hits for %q after onboard", term)
			}
			if hits[0].Source != "page" || hits[0].Path != wantPath || hits[0].Repo != "acme/paged" {
				t.Errorf("hit for %q = %+v, want source page at %q", term, hits[0], wantPath)
			}
		}
	})

	// Block removed at HEAD (same tree): pages are desired state, so the
	// re-ingest deletes every row, nulls the api columns, and purges the index.
	run(&ingest.RepoSnapshot{
		HeadSHA: "h2", DefaultBranch: "main", ConfigYAML: []byte(fixtureConfig), Blobs: blobs,
	})

	t.Run("disable at HEAD empties list, 404s pages, purges index", func(t *testing.T) {
		var list struct {
			Pages []struct{ Path string } `json:"pages"`
		}
		if code := getJSON(t, "/api/v1/repos/acme/paged/pages", &list); code != http.StatusOK {
			t.Fatalf("list pages status = %d, want 200 (empty list, not 404)", code)
		}
		if len(list.Pages) != 0 {
			t.Errorf("pages after disable = %+v, want none", list.Pages)
		}
		for _, path := range []string{"guides", "examples%2Fexample1.md", "CONTRIBUTING.md"} {
			if code := getJSON(t, "/api/v1/repos/acme/paged/pages/"+path, nil); code != http.StatusNotFound {
				t.Errorf("page %q after disable: status = %d, want 404", path, code)
			}
		}
		for _, term := range []string{"wombat", "quokka"} {
			for _, h := range searchHits(t, searchMux, term) {
				if h.Source == "page" && h.Repo == "acme/paged" {
					t.Errorf("page hit for %q survived the disable: %+v", term, h)
				}
			}
		}
	})
}
