package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/donaldgifford/docz/v2/internal/authorize"
	"github.com/donaldgifford/docz/v2/internal/search"
)

// fakeSearcher captures the SearchParams it receives and returns a canned result.
type fakeSearcher struct {
	got    search.SearchParams
	calls  int
	result search.SearchResult
	err    error
}

func (f *fakeSearcher) Search(_ context.Context, p *search.SearchParams) (search.SearchResult, error) {
	f.got = *p
	f.calls++
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
			Created: "2026-01-15", UpdatedAt: "2025-06-22T18:04:11Z",
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

	// Both seeded repo ids reached the searcher, and the query params mapped.
	if len(fs.got.AllowedRepoIDs) != 2 || fs.got.AllowedRepoIDs[0] != 1 || fs.got.AllowedRepoIDs[1] != 2 {
		t.Errorf("AllowedRepoIDs = %v, want [1 2] from the authorize seam", fs.got.AllowedRepoIDs)
	}
	if fs.got.Query != "logging" || fs.got.Type != "frameworks" {
		t.Errorf("search params = %+v, want q=logging type=frameworks", fs.got)
	}

	var body struct {
		Query          string `json:"query"`
		EstimatedTotal int64  `json:"estimated_total_hits"`
		Hits           []struct {
			Repo      string `json:"repo"`
			DocID     string `json:"doc_id"`
			Created   string `json:"created"`
			UpdatedAt string `json:"updated_at"`
			Snippet   string `json:"snippet"`
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
	// Both dates reach the wire under their contract spellings.
	if body.Hits[0].Created != "2026-01-15" || body.Hits[0].UpdatedAt != "2025-06-22T18:04:11Z" {
		t.Errorf("hit dates = %q/%q, want 2026-01-15 and 2025-06-22T18:04:11Z",
			body.Hits[0].Created, body.Hits[0].UpdatedAt)
	}
	if body.Facets["type"]["frameworks"] != 1 {
		t.Errorf("facets = %+v, want type.frameworks=1", body.Facets)
	}
}

// TestSearchSortParameter covers the validated sort: an accepted token
// reaches the search layer, an absent one leaves the search unsorted, and an
// unrecognized one is rejected before any search runs.
func TestSearchSortParameter(t *testing.T) {
	t.Run("an accepted token reaches the searcher", func(t *testing.T) {
		st := seededStore()
		fs := &fakeSearcher{}
		r := chi.NewRouter()
		NewHandlerWithSearch(st, fs).Mount(r, authorize.Middleware(authorize.NewAllReposAuthorizer(st)))

		rec := doGet(t, r, "/api/v1/search?q=x&sort=updated_at:desc")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if fs.got.Sort != search.SortUpdatedDesc {
			t.Errorf("Sort = %q, want %q", fs.got.Sort, search.SortUpdatedDesc)
		}
	})

	t.Run("an absent sort leaves the search unsorted", func(t *testing.T) {
		st := seededStore()
		fs := &fakeSearcher{}
		r := chi.NewRouter()
		NewHandlerWithSearch(st, fs).Mount(r, authorize.Middleware(authorize.NewAllReposAuthorizer(st)))

		doGet(t, r, "/api/v1/search?q=x")
		if fs.got.Sort != "" {
			t.Errorf("Sort = %q, want empty so ranking stays relevance-only", fs.got.Sort)
		}
	})

	t.Run("an unknown token is a 400 and never searches", func(t *testing.T) {
		st := seededStore()
		fs := &fakeSearcher{}
		r := chi.NewRouter()
		NewHandlerWithSearch(st, fs).Mount(r, authorize.Middleware(authorize.NewAllReposAuthorizer(st)))

		rec := doGet(t, r, "/api/v1/search?q=x&sort=bogus")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		var body struct {
			Error string `json:"error"`
		}
		mustDecode(t, rec, &body)
		if body.Error != "invalid sort" {
			t.Errorf("error = %q, want %q", body.Error, "invalid sort")
		}
		// Rejection happens before the search: a bad token must not cost a
		// Meilisearch round trip, and must not return results in an order
		// the caller did not ask for.
		if fs.calls != 0 {
			t.Errorf("searcher called %d times, want 0", fs.calls)
		}
	})
}

// TestSearchSourceFilter covers the record-kind filter. Unlike sort it is not
// validated: an unknown value reaches the search layer and matches nothing,
// matching how the repo/type/status/author filters already behave.
func TestSearchSourceFilter(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"pages only", "source=page", search.SourcePage},
		{"documents only", "source=doc", search.SourceDoc},
		{"an unknown value is passed through, not rejected", "source=nonsense", "nonsense"},
		{"absent leaves both kinds", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := seededStore()
			fs := &fakeSearcher{}
			r := chi.NewRouter()
			NewHandlerWithSearch(st, fs).Mount(r, authorize.Middleware(authorize.NewAllReposAuthorizer(st)))

			rec := doGet(t, r, "/api/v1/search?q=x&"+tt.query)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if fs.got.Source != tt.want {
				t.Errorf("Source = %q, want %q", fs.got.Source, tt.want)
			}
		})
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
