package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/donaldgifford/docz-api/internal/authorize"
	"github.com/donaldgifford/docz-api/internal/search"
)

// fakeSearcher captures the SearchParams it receives and returns a canned result.
type fakeSearcher struct {
	got    search.SearchParams
	result search.SearchResult
	err    error
}

func (f *fakeSearcher) Search(_ context.Context, p *search.SearchParams) (search.SearchResult, error) {
	f.got = *p
	return f.result, f.err
}

func TestSearchEndpoint(t *testing.T) {
	st := seededStore()
	fs := &fakeSearcher{result: search.SearchResult{
		Query:          "logging",
		EstimatedTotal: 1,
		Hits: []search.SearchHit{{
			Repo: "acme/platform", DocID: "FW-0001", Type: "frameworks",
			Title: "Intro", Status: "Draft", Author: "Jane",
			Snippet: "…structured <em>logging</em>…",
		}},
		Facets: map[string]search.FacetMap{"type": {"frameworks": 1}},
	}}

	r := chi.NewRouter()
	NewHandlerWithSearch(st, fs).Mount(r, authorize.Middleware(authorize.NewAllReposAuthorizer(st)))

	rec := doGet(t, r, "/api/v1/search?q=logging&type=frameworks")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	// The authorized repo id (1) reached the searcher, and the query params mapped.
	if len(fs.got.AllowedRepoIDs) != 1 || fs.got.AllowedRepoIDs[0] != 1 {
		t.Errorf("AllowedRepoIDs = %v, want [1] from the authorize seam", fs.got.AllowedRepoIDs)
	}
	if fs.got.Query != "logging" || fs.got.Type != "frameworks" {
		t.Errorf("search params = %+v, want q=logging type=frameworks", fs.got)
	}

	var body struct {
		Query          string `json:"query"`
		EstimatedTotal int64  `json:"estimated_total_hits"`
		Hits           []struct {
			Repo    string `json:"repo"`
			DocID   string `json:"doc_id"`
			Snippet string `json:"snippet"`
		} `json:"hits"`
		Facets map[string]map[string]int64 `json:"facets"`
	}
	mustDecode(t, rec, &body)
	if body.Query != "logging" || body.EstimatedTotal != 1 {
		t.Errorf("body = %+v", body)
	}
	if len(body.Hits) != 1 || body.Hits[0].DocID != "FW-0001" || body.Hits[0].Snippet == "" {
		t.Errorf("hits = %+v", body.Hits)
	}
	if body.Facets["type"]["frameworks"] != 1 {
		t.Errorf("facets = %+v, want type.frameworks=1", body.Facets)
	}
}

func TestSearchInjectsAuthorizedRepoScope(t *testing.T) {
	st := seededStore()
	fs := &fakeSearcher{}

	r := chi.NewRouter()
	NewHandlerWithSearch(st, fs).Mount(r,
		authorize.Middleware(fixedAuthorizer{allowed: authorize.AllowedRepos{999}}))

	doGet(t, r, "/api/v1/search?q=x")

	// The seam injects exactly the authorizer's set, not the store's repos.
	if len(fs.got.AllowedRepoIDs) != 1 || fs.got.AllowedRepoIDs[0] != 999 {
		t.Errorf("AllowedRepoIDs = %v, want [999] from the authorize seam", fs.got.AllowedRepoIDs)
	}
}

func TestSearchRouteAbsentWithoutSearcher(t *testing.T) {
	st := seededStore()
	// NewHandler (no searcher) must not mount /search.
	srv := testServer(st, authorize.NewAllReposAuthorizer(st))

	rec := doGet(t, srv, "/api/v1/search?q=x")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (route not mounted without a searcher)", rec.Code)
	}
}
