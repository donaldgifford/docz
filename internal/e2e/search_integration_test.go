//go:build integration

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/donaldgifford/docz-api/internal/authorize"
	"github.com/donaldgifford/docz-api/internal/httpapi"
	"github.com/donaldgifford/docz-api/internal/ingest"
	"github.com/donaldgifford/docz-api/internal/search"
	"github.com/donaldgifford/docz-api/internal/store"
)

// startMeili boots a Meilisearch container for one test and returns a Client
// with its index ensured. The container is terminated on test cleanup.
func startMeili(t *testing.T) *search.Client {
	t.Helper()
	ctx := context.Background()

	const masterKey = "e2e-master-key"
	req := testcontainers.ContainerRequest{
		Image:        "getmeili/meilisearch:v1.12",
		ExposedPorts: []string{"7700/tcp"},
		Env: map[string]string{
			"MEILI_MASTER_KEY":   masterKey,
			"MEILI_NO_ANALYTICS": "true",
		},
		WaitingFor: wait.ForHTTP("/health").
			WithPort("7700/tcp").
			WithStatusCodeMatcher(func(status int) bool { return status == http.StatusOK }).
			WithStartupTimeout(60 * time.Second),
	}
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start meilisearch: %v", err)
	}
	t.Cleanup(func() {
		if terr := ctr.Terminate(ctx); terr != nil {
			t.Logf("terminate meilisearch: %v", terr)
		}
	})

	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatalf("meili host: %v", err)
	}
	port, err := ctr.MappedPort(ctx, "7700/tcp")
	if err != nil {
		t.Fatalf("meili port: %v", err)
	}

	client := search.New("http://"+host+":"+port.Port(), masterKey)
	if err := client.EnsureIndex(ctx); err != nil {
		t.Fatalf("ensure index: %v", err)
	}
	return client
}

// TestE2ESearchAfterOnboard proves the phase's headline criterion end-to-end:
// onboarding a repo through the real ingest pipeline (real Postgres + real
// Meilisearch indexer) makes its documents searchable via GET /api/v1/search,
// returning hits, facet counts, and highlighted snippets.
func TestE2ESearchAfterOnboard(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // hermetic doczcfg.Load
	ctx := t.Context()

	meili := startMeili(t)

	const instID int64 = 950
	if err := testStore.UpsertInstallation(ctx, store.InstallationInput{
		ID: instID, AccountLogin: fixtureOwner, AccountType: "Organization",
	}); err != nil {
		t.Fatalf("seed installation: %v", err)
	}

	snap := &ingest.RepoSnapshot{
		HeadSHA:       "s1",
		DefaultBranch: "main",
		ConfigYAML:    []byte(fixtureConfig),
		Blobs: []ingest.BlobEntry{
			{
				Path: "docs/frameworks/0001-intro.md", GitSHA: "g1",
				Content: docCreated("FW-0001", "Logging",
					"Adopt structured logging across services.", "2026-01-15"),
			},
			{
				Path: "docs/frameworks/0002-next.md", GitSHA: "g2",
				Content: docCreated("FW-0002", "Tracing",
					"A distributed tracing plan for logging spans.", "2026-03-02"),
			},
		},
	}

	// Onboard through the real pipeline with the real Meilisearch indexer.
	if _, err := ingest.NewService(testStore, staticFetcher{snap: snap}, meili).
		Run(ctx, instID, fixtureOwner, "searchable"); err != nil {
		t.Fatalf("onboard: %v", err)
	}

	// A search-enabled mux over the same store, wired exactly as main does.
	r := chi.NewRouter()
	httpapi.NewHandlerWithSearch(testStore, meili).
		Mount(r, authorize.Middleware(authorize.NewAllReposAuthorizer(testStore)))

	rec := httptest.NewRecorder()
	sreq := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/search?q=logging", http.NoBody)
	r.ServeHTTP(rec, sreq)
	if rec.Code != http.StatusOK {
		t.Fatalf("search status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}

	body := decodeSearch(t, rec)

	if body.Query != "logging" {
		t.Errorf("query = %q, want logging", body.Query)
	}
	if len(body.Hits) == 0 {
		t.Fatalf("no hits for q=logging after onboard")
	}
	// FW-0001 ("Logging" + "structured logging" body) is the match.
	hit := body.Hits[0]
	if hit.DocID != "FW-0001" || hit.Repo != "acme/searchable" || hit.Type != "frameworks" {
		t.Errorf("top hit = %+v, want FW-0001 in acme/searchable", hit)
	}
	if !strings.Contains(hit.Snippet, "<em>") || !strings.Contains(hit.Snippet, "</em>") {
		t.Errorf("snippet = %q, want an <em>-highlighted match", hit.Snippet)
	}
	if body.Facets["type"]["frameworks"] == 0 {
		t.Errorf("facets = %+v, want a frameworks type count", body.Facets)
	}

	t.Run("hit dates agree with the document endpoint", func(t *testing.T) {
		// The authored date survives ingest unchanged.
		if hit.Created != "2026-01-15" {
			t.Errorf("hit created = %q, want the frontmatter date 2026-01-15", hit.Created)
		}
		// The stamp is RFC3339 in UTC — the wire promise, on a value that
		// travelled Postgres -> index -> search response.
		if _, err := time.Parse(time.RFC3339, hit.UpdatedAt); err != nil {
			t.Fatalf("hit updated_at %q is not RFC3339: %v", hit.UpdatedAt, err)
		}
		if !strings.HasSuffix(hit.UpdatedAt, "Z") {
			t.Errorf("hit updated_at = %q, want a UTC Z suffix on every host", hit.UpdatedAt)
		}

		// The same field on the document endpoint must be byte-identical:
		// one logical field, two spellings of the same row.
		drec := httptest.NewRecorder()
		dreq := httptest.NewRequestWithContext(ctx, http.MethodGet,
			"/api/v1/repos/acme/searchable/types/frameworks/docs/FW-0001", http.NoBody)
		r.ServeHTTP(drec, dreq)
		if drec.Code != http.StatusOK {
			t.Fatalf("getDoc status = %d, want 200", drec.Code)
		}
		var docBody struct {
			Created   string `json:"created"`
			UpdatedAt string `json:"updated_at"`
		}
		if err := json.Unmarshal(drec.Body.Bytes(), &docBody); err != nil {
			t.Fatalf("decode document %q: %v", drec.Body.String(), err)
		}
		if docBody.UpdatedAt != hit.UpdatedAt {
			t.Errorf("updated_at differs between endpoints: document %q, hit %q",
				docBody.UpdatedAt, hit.UpdatedAt)
		}
		if docBody.Created != hit.Created {
			t.Errorf("created differs between endpoints: document %q, hit %q",
				docBody.Created, hit.Created)
		}
	})

	t.Run("sorting by created orders the results", func(t *testing.T) {
		// Both documents landed in one reconcile, so they share an
		// updated_at to the microsecond and only created can separate them.
		desc := searchJSON(t, r, ctx, "/api/v1/search?q=logging&sort=created:desc")
		if got := docIDs(desc.Hits); !slices.Equal(got, []string{"FW-0002", "FW-0001"}) {
			t.Errorf("created:desc = %v, want [FW-0002 FW-0001]", got)
		}
		asc := searchJSON(t, r, ctx, "/api/v1/search?q=logging&sort=created:asc")
		if got := docIDs(asc.Hits); !slices.Equal(got, []string{"FW-0001", "FW-0002"}) {
			t.Errorf("created:asc = %v, want [FW-0001 FW-0002]", got)
		}
	})

	t.Run("the source filter narrows to documents", func(t *testing.T) {
		res := searchJSON(t, r, ctx, "/api/v1/search?q=logging&source=doc")
		if len(res.Hits) != 2 {
			t.Errorf("source=doc returned %d hits, want both documents", len(res.Hits))
		}
		pages := searchJSON(t, r, ctx, "/api/v1/search?q=logging&source=page")
		if len(pages.Hits) != 0 {
			t.Errorf("source=page returned %d hits, want none — this repo publishes no pages",
				len(pages.Hits))
		}
	})

	t.Run("an unknown sort is rejected by the real router", func(t *testing.T) {
		brec := httptest.NewRecorder()
		breq := httptest.NewRequestWithContext(ctx, http.MethodGet,
			"/api/v1/search?q=logging&sort=bogus", http.NoBody)
		r.ServeHTTP(brec, breq)
		if brec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body %q)", brec.Code, brec.Body.String())
		}
		var errBody struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(brec.Body.Bytes(), &errBody); err != nil {
			t.Fatalf("decode error body %q: %v", brec.Body.String(), err)
		}
		if errBody.Error != "invalid sort" {
			t.Errorf("error = %q, want %q", errBody.Error, "invalid sort")
		}
	})
}

// searchHit mirrors the SearchHit wire shape the contract promises. It is
// declared here rather than imported so the test reads the JSON a consumer
// would, not the Go struct that produced it.
type searchHit struct {
	Repo      string `json:"repo"`
	DocID     string `json:"doc_id"`
	Type      string `json:"type"`
	Created   string `json:"created"`
	UpdatedAt string `json:"updated_at"`
	Snippet   string `json:"snippet"`
}

// searchResponse is the decode target for the search wire shape.
type searchResponse struct {
	Query          string                      `json:"query"`
	EstimatedTotal int64                       `json:"estimated_total_hits"`
	Hits           []searchHit                 `json:"hits"`
	Facets         map[string]map[string]int64 `json:"facets"`
}

func decodeSearch(t *testing.T, rec *httptest.ResponseRecorder) searchResponse {
	t.Helper()
	var body searchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode search response %q: %v", rec.Body.String(), err)
	}
	return body
}

// searchJSON runs one search against the real router and decodes it.
func searchJSON(t *testing.T, r chi.Router, ctx context.Context, target string) searchResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200 (body %q)", target, rec.Code, rec.Body.String())
	}
	return decodeSearch(t, rec)
}

// docIDs lists the hit doc ids in result order.
func docIDs(hits []searchHit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.DocID
	}
	return out
}
