//go:build integration

// Package search integration tests exercise a real Meilisearch (via
// testcontainers): index population, facet counts, snippet highlighting,
// deletion, and the authorize filter-injection seam. Only exported client
// methods are used, so these double as a usage contract for the package.
package search

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const meiliMasterKey = "test-master-key"

// testClient is a Meilisearch-backed Client shared across the integration tests;
// the container is started once in TestMain.
var testClient *Client

func TestMain(m *testing.M) {
	os.Exit(runMain(m))
}

func runMain(m *testing.M) int {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "getmeili/meilisearch:v1.12",
		ExposedPorts: []string{"7700/tcp"},
		Env: map[string]string{
			"MEILI_MASTER_KEY":   meiliMasterKey,
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
		log.Printf("start meilisearch: %v", err)
		return 1
	}
	defer func() {
		if terr := ctr.Terminate(ctx); terr != nil {
			log.Printf("terminate meilisearch: %v", terr)
		}
	}()

	host, err := ctr.Host(ctx)
	if err != nil {
		log.Printf("meili host: %v", err)
		return 1
	}
	port, err := ctr.MappedPort(ctx, "7700/tcp")
	if err != nil {
		log.Printf("meili port: %v", err)
		return 1
	}

	testClient = New("http://"+host+":"+port.Port(), meiliMasterKey)
	if err := testClient.EnsureIndex(ctx); err != nil {
		log.Printf("ensure index: %v", err)
		return 1
	}
	return m.Run()
}

// sampleDocs is a fixed three-document corpus spanning two repos and two types.
func sampleDocs() []IndexDoc {
	return []IndexDoc{
		{
			ID: "1_RFC-0001", Repo: "acme/platform", RepoID: 1, DocID: "RFC-0001",
			Type: "rfc", Title: "Structured logging", Status: "Accepted", Author: "Jane Dev",
			Created: "2026-01-15", Body: "We should adopt structured logging across services.",
			UpdatedAt: 1750615451,
		},
		{
			ID: "1_RFC-0002", Repo: "acme/platform", RepoID: 1, DocID: "RFC-0002",
			Type: "rfc", Title: "Tracing", Status: "Draft", Author: "John Ops",
			Created: "2026-02-01", Body: "A distributed tracing rollout plan.",
			UpdatedAt: 1750615452,
		},
		{
			ID: "2_ADR-0001", Repo: "beta/infra", RepoID: 2, DocID: "ADR-0001",
			Type: "adr", Title: "Use Postgres", Status: "Accepted", Author: "Jane Dev",
			Created: "2026-03-01", Body: "Adopt Postgres as the datastore, with request logging.",
			UpdatedAt: 1750615453,
		},
	}
}

// seed re-indexes the full sample corpus, restoring a known 3-document state.
// Every test seeds first so the tests are order-independent despite the shared
// index (all docs share the same primary keys, so this is an idempotent upsert).
func seed(t *testing.T) {
	t.Helper()
	if err := testClient.IndexDocuments(t.Context(), sampleDocs()); err != nil {
		t.Fatalf("seed index: %v", err)
	}
}

func TestIntegrationIndexAndSearch(t *testing.T) {
	seed(t)

	res, err := testClient.Search(t.Context(), &SearchParams{
		Query:          "structured logging",
		AllowedRepoIDs: []int64{1},
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res.Hits) == 0 {
		t.Fatalf("no hits for 'structured logging' in repo 1")
	}
	// RFC-0001 (title + body match) is the top hit; title outranks body.
	if res.Hits[0].DocID != "RFC-0001" || res.Hits[0].Repo != "acme/platform" {
		t.Errorf("top hit = %+v, want RFC-0001 in acme/platform", res.Hits[0])
	}
}

func TestIntegrationFacetCounts(t *testing.T) {
	seed(t)

	// A placeholder (empty) query matches every visible doc; facets count them.
	res, err := testClient.Search(t.Context(), &SearchParams{
		AllowedRepoIDs: []int64{1, 2},
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if res.EstimatedTotal != 3 {
		t.Errorf("estimated_total_hits = %d, want 3", res.EstimatedTotal)
	}
	if got := res.Facets["type"]; got["rfc"] != 2 || got["adr"] != 1 {
		t.Errorf("type facet = %v, want rfc:2 adr:1", got)
	}
	if got := res.Facets["status"]; got["Accepted"] != 2 || got["Draft"] != 1 {
		t.Errorf("status facet = %v, want Accepted:2 Draft:1", got)
	}
	if got := res.Facets["repo"]; got["acme/platform"] != 2 || got["beta/infra"] != 1 {
		t.Errorf("repo facet = %v, want acme/platform:2 beta/infra:1", got)
	}
	if got := res.Facets["author"]; got["Jane Dev"] != 2 || got["John Ops"] != 1 {
		t.Errorf("author facet = %v, want Jane Dev:2 John Ops:1", got)
	}
}

func TestIntegrationSnippetHighlight(t *testing.T) {
	seed(t)

	res, err := testClient.Search(t.Context(), &SearchParams{
		Query:          "logging",
		AllowedRepoIDs: []int64{1},
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res.Hits) == 0 {
		t.Fatalf("no hits for 'logging' in repo 1")
	}
	// The body snippet highlights the matched term with <em> tags.
	snippet := res.Hits[0].Snippet
	if !strings.Contains(snippet, "<em>") || !strings.Contains(snippet, "</em>") {
		t.Errorf("snippet = %q, want <em>-highlighted match", snippet)
	}
}

func TestIntegrationDeletionRemovesFromIndex(t *testing.T) {
	seed(t)

	// Before deletion, the tracing RFC is findable.
	before, err := testClient.Search(t.Context(), &SearchParams{
		Query:          "tracing",
		AllowedRepoIDs: []int64{1},
	})
	if err != nil {
		t.Fatalf("search before delete: %v", err)
	}
	if len(before.Hits) == 0 {
		t.Fatalf("expected RFC-0002 before deletion")
	}

	if err := testClient.DeleteDocuments(t.Context(), []string{"1_RFC-0002"}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	after, err := testClient.Search(t.Context(), &SearchParams{
		Query:          "tracing",
		AllowedRepoIDs: []int64{1},
	})
	if err != nil {
		t.Fatalf("search after delete: %v", err)
	}
	for _, h := range after.Hits {
		if h.DocID == "RFC-0002" {
			t.Errorf("RFC-0002 still present after deletion: %+v", after.Hits)
		}
	}
}

func TestIntegrationFilterInjectionSeam(t *testing.T) {
	seed(t)

	// Both repo 1 (RFC-0001) and repo 2 (ADR-0001) documents mention "logging".
	// Scoped to repo 1, only acme/platform docs come back.
	repo1, err := testClient.Search(t.Context(), &SearchParams{
		Query:          "logging",
		AllowedRepoIDs: []int64{1},
	})
	if err != nil {
		t.Fatalf("search repo 1: %v", err)
	}
	if len(repo1.Hits) == 0 {
		t.Fatalf("no hits for repo 1")
	}
	for _, h := range repo1.Hits {
		if h.Repo != "acme/platform" {
			t.Errorf("repo-1 scope leaked %q", h.Repo)
		}
	}

	// Scoped to repo 2, only beta/infra's ADR-0001 comes back.
	repo2, err := testClient.Search(t.Context(), &SearchParams{
		Query:          "logging",
		AllowedRepoIDs: []int64{2},
	})
	if err != nil {
		t.Fatalf("search repo 2: %v", err)
	}
	for _, h := range repo2.Hits {
		if h.Repo != "beta/infra" {
			t.Errorf("repo-2 scope leaked %q", h.Repo)
		}
	}

	// An empty allowed set authorizes nothing: no results.
	none, err := testClient.Search(t.Context(), &SearchParams{
		Query:          "logging",
		AllowedRepoIDs: []int64{},
	})
	if err != nil {
		t.Fatalf("search empty scope: %v", err)
	}
	if len(none.Hits) != 0 || none.EstimatedTotal != 0 {
		t.Errorf("empty scope returned %d hits (total %d), want none", len(none.Hits), none.EstimatedTotal)
	}
}
