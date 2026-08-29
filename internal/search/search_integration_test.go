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

// sampleDocs is a fixed six-record corpus spanning two repos, two doc types,
// and both sources: three documents plus three api-block pages (two in repo 1,
// one in repo 2 — so the repo-scope and purge tests cover pages too). Page
// records leave the doc-only fields empty, exactly as ingest's toIndexPage
// does; their ids use the hashed-page shape but are literals here (the hash
// itself is ingest's concern).
func sampleDocs() []IndexDoc {
	return []IndexDoc{
		{
			ID: "1_RFC-0001", Source: SourceDoc, Repo: "acme/platform", RepoID: 1, DocID: "RFC-0001",
			Type: "rfc", Title: "Structured logging", Status: "Accepted", Author: "Jane Dev",
			Created: "2026-01-15", Path: "docs/rfc/0001-structured-logging.md",
			Body:      "We should adopt structured logging across services.",
			UpdatedAt: 1750615451,
		},
		{
			ID: "1_RFC-0002", Source: SourceDoc, Repo: "acme/platform", RepoID: 1, DocID: "RFC-0002",
			Type: "rfc", Title: "Tracing", Status: "Draft", Author: "John Ops",
			Created: "2026-02-01", Path: "docs/rfc/0002-tracing.md",
			Body:      "A distributed tracing rollout plan.",
			UpdatedAt: 1750615452,
		},
		{
			ID: "2_ADR-0001", Source: SourceDoc, Repo: "beta/infra", RepoID: 2, DocID: "ADR-0001",
			Type: "adr", Title: "Use Postgres", Status: "Accepted", Author: "Jane Dev",
			Created: "2026-03-01", Path: "docs/adr/0001-use-postgres.md",
			Body:      "Adopt Postgres as the datastore, with request logging.",
			UpdatedAt: 1750615453,
		},
		{
			ID: "1_p_00112233aabbccdd", Source: SourcePage, Repo: "acme/platform", RepoID: 1,
			Title: "Setup Guide", Path: "guides/setup.md",
			Body:      "Widget fleet deployment walkthrough.",
			UpdatedAt: 1750615454,
		},
		{
			ID: "1_p_1122334455667788", Source: SourcePage, Repo: "acme/platform", RepoID: 1,
			Title: "Contributing", Path: "CONTRIBUTING.md",
			Body:      "Contribution guidelines for the platform.",
			UpdatedAt: 1750615455,
		},
		{
			ID: "2_p_99aabbccddeeff00", Source: SourcePage, Repo: "beta/infra", RepoID: 2,
			Title: "Runbooks", Path: "runbooks",
			Body:      "Operational runbooks for infra logging.",
			UpdatedAt: 1750615456,
		},
	}
}

// seed re-indexes the full sample corpus, restoring a known six-record state.
// Every test seeds first so the tests are order-independent despite the shared
// index (all records share the same primary keys, so this is an idempotent
// upsert).
func seed(t *testing.T) {
	t.Helper()
	if err := testClient.IndexDocuments(t.Context(), sampleDocs()); err != nil {
		t.Fatalf("seed index: %v", err)
	}
}

func TestIntegrationEnsureIndexIdempotent(t *testing.T) {
	// TestMain already called EnsureIndex once (cold). A second call must
	// succeed on the existing index: the create task fails harmlessly (never
	// waited on) and only the settings update is applied.
	if err := testClient.EnsureIndex(t.Context()); err != nil {
		t.Fatalf("EnsureIndex on an existing index: %v", err)
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
	if res.EstimatedTotal != 6 {
		t.Errorf("estimated_total_hits = %d, want 6", res.EstimatedTotal)
	}
	if got := res.Facets["type"]; got["rfc"] != 2 || got["adr"] != 1 {
		t.Errorf("type facet = %v, want rfc:2 adr:1", got)
	}
	if got := res.Facets["status"]; got["Accepted"] != 2 || got["Draft"] != 1 {
		t.Errorf("status facet = %v, want Accepted:2 Draft:1", got)
	}
	if got := res.Facets["repo"]; got["acme/platform"] != 4 || got["beta/infra"] != 2 {
		t.Errorf("repo facet = %v, want acme/platform:4 beta/infra:2", got)
	}
	if got := res.Facets["author"]; got["Jane Dev"] != 2 || got["John Ops"] != 1 {
		t.Errorf("author facet = %v, want Jane Dev:2 John Ops:1", got)
	}
	if got := res.Facets["source"]; got["doc"] != 3 || got["page"] != 3 {
		t.Errorf("source facet = %v, want doc:3 page:3", got)
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

// TestIntegrationPageHitShape pins a page hit's wire shape: source "page",
// the published path, and "" for every doc-only field. The repo scope applies
// to pages exactly as it does to docs.
func TestIntegrationPageHitShape(t *testing.T) {
	seed(t)

	res, err := testClient.Search(t.Context(), &SearchParams{
		Query:          "contribution guidelines",
		AllowedRepoIDs: []int64{1},
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res.Hits) == 0 {
		t.Fatalf("no hits for the CONTRIBUTING.md page in repo 1")
	}
	hit := res.Hits[0]
	if hit.Source != SourcePage || hit.Path != "CONTRIBUTING.md" || hit.Title != "Contributing" {
		t.Errorf("page hit = %+v, want source page at CONTRIBUTING.md", hit)
	}
	if hit.DocID != "" || hit.Type != "" || hit.Status != "" || hit.Author != "" {
		t.Errorf("page hit doc-only fields = %+v, want all empty", hit)
	}
	if !strings.Contains(hit.Snippet, "<em>") {
		t.Errorf("page snippet = %q, want <em>-highlighted match", hit.Snippet)
	}

	// Repo 2's runbooks page is invisible under a repo-1 scope...
	scoped, err := testClient.Search(t.Context(), &SearchParams{
		Query:          "runbooks",
		AllowedRepoIDs: []int64{1},
	})
	if err != nil {
		t.Fatalf("scoped search: %v", err)
	}
	for _, h := range scoped.Hits {
		if h.Source == SourcePage && h.Repo != "acme/platform" {
			t.Errorf("repo-1 scope leaked page %+v", h)
		}
	}
	// ...and visible under its own.
	own, err := testClient.Search(t.Context(), &SearchParams{
		Query:          "runbooks",
		AllowedRepoIDs: []int64{2},
	})
	if err != nil {
		t.Fatalf("repo-2 search: %v", err)
	}
	if len(own.Hits) == 0 || own.Hits[0].Path != "runbooks" {
		t.Fatalf("repo-2 page not findable in its own scope: %+v", own.Hits)
	}
}

// TestIntegrationPageDeletionRemovesFromIndex mirrors the doc-deletion test
// for the page namespace: deleting a page's hashed primary key removes it.
func TestIntegrationPageDeletionRemovesFromIndex(t *testing.T) {
	seed(t)

	before, err := testClient.Search(t.Context(), &SearchParams{
		Query:          "deployment walkthrough",
		AllowedRepoIDs: []int64{1},
	})
	if err != nil {
		t.Fatalf("search before delete: %v", err)
	}
	if len(before.Hits) == 0 {
		t.Fatalf("expected the setup guide page before deletion")
	}

	if err := testClient.DeleteDocuments(t.Context(), []string{"1_p_00112233aabbccdd"}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	after, err := testClient.Search(t.Context(), &SearchParams{
		Query:          "deployment walkthrough",
		AllowedRepoIDs: []int64{1},
	})
	if err != nil {
		t.Fatalf("search after delete: %v", err)
	}
	for _, h := range after.Hits {
		if h.Path == "guides/setup.md" {
			t.Errorf("setup guide page still present after deletion: %+v", after.Hits)
		}
	}
	seed(t) // restore the shared corpus
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

func TestIntegrationDeleteRepoDocuments(t *testing.T) {
	seed(t)

	// Repo 2 has one document and one page before the purge; the repo_id
	// filter behind DeleteRepoDocuments covers both sources, which is what
	// makes the offboard purge complete (IMPL-0007 Phase 6).
	before, err := testClient.Search(t.Context(), &SearchParams{AllowedRepoIDs: []int64{2}})
	if err != nil {
		t.Fatalf("search repo 2 before purge: %v", err)
	}
	if before.EstimatedTotal != 2 {
		t.Fatalf("expected repo 2's doc + page before purge, got %d", before.EstimatedTotal)
	}

	if derr := testClient.DeleteRepoDocuments(t.Context(), 2); derr != nil {
		t.Fatalf("DeleteRepoDocuments: %v", derr)
	}

	// Repo 2 is now empty...
	after, err := testClient.Search(t.Context(), &SearchParams{AllowedRepoIDs: []int64{2}})
	if err != nil {
		t.Fatalf("search repo 2 after purge: %v", err)
	}
	if after.EstimatedTotal != 0 || len(after.Hits) != 0 {
		t.Errorf("repo 2 has %d hits after purge, want 0", len(after.Hits))
	}

	// ...while repo 1's documents are untouched (the purge is scoped by repo_id).
	repo1, err := testClient.Search(t.Context(), &SearchParams{AllowedRepoIDs: []int64{1}})
	if err != nil {
		t.Fatalf("search repo 1 after purge: %v", err)
	}
	if repo1.EstimatedTotal == 0 {
		t.Errorf("repo 1 documents were removed by a repo-2 purge")
	}

	// Restore the corpus so the shared index stays order-independent.
	seed(t)
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
