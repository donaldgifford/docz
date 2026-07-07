//go:build integration

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
			{Path: "docs/frameworks/0001-intro.md", GitSHA: "g1", Content: doc("FW-0001", "Logging", "Adopt structured logging across services.")},
			{Path: "docs/frameworks/0002-next.md", GitSHA: "g2", Content: doc("FW-0002", "Tracing", "A distributed tracing plan.")},
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

	var body struct {
		Query          string `json:"query"`
		EstimatedTotal int64  `json:"estimated_total_hits"`
		Hits           []struct {
			Repo    string `json:"repo"`
			DocID   string `json:"doc_id"`
			Type    string `json:"type"`
			Snippet string `json:"snippet"`
		} `json:"hits"`
		Facets map[string]map[string]int64 `json:"facets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode search response %q: %v", rec.Body.String(), err)
	}

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
}
