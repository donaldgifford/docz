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
	"slices"
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

// paths returns each hit's path in result order, the readable identity for
// an ordering assertion (the index primary key never reaches the wire).
func paths(hits []SearchHit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.Path
	}
	return out
}

// TestIntegrationHitDates proves the retrieve list. Both dates are attributes
// Meilisearch only returns when asked for by name, so a stale
// retrieveAttributes list would empty them here while every faked-searcher
// test still passed (INV-0009 F2).
func TestIntegrationHitDates(t *testing.T) {
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

	// RFC-0001 is the top hit: seeded UpdatedAt 1750615451, Created 2026-01-15.
	hit := res.Hits[0]
	if hit.DocID != "RFC-0001" {
		t.Fatalf("top hit = %q, want RFC-0001", hit.DocID)
	}
	if hit.UpdatedAt != "2025-06-22T18:04:11Z" {
		t.Errorf("updated_at = %q, want 2025-06-22T18:04:11Z (RFC3339 in UTC)", hit.UpdatedAt)
	}
	if hit.Created != "2026-01-15" {
		t.Errorf("created = %q, want 2026-01-15 passed through from the index", hit.Created)
	}

	// A page hit carries a real stamp and no authored date.
	pages, err := testClient.Search(t.Context(), &SearchParams{
		Query:          "contribution guidelines",
		AllowedRepoIDs: []int64{1},
		Source:         SourcePage,
	})
	if err != nil {
		t.Fatalf("page search: %v", err)
	}
	if len(pages.Hits) == 0 {
		t.Fatalf("no page hits for 'contribution guidelines'")
	}
	page := pages.Hits[0]
	if page.UpdatedAt != "2025-06-22T18:04:15Z" {
		t.Errorf("page updated_at = %q, want 2025-06-22T18:04:15Z — pages are stamped too", page.UpdatedAt)
	}
	if page.Created != "" {
		t.Errorf("page created = %q, want empty", page.Created)
	}
}

// TestIntegrationSortOrdersWholeResultSet sorts an unfiltered query, where
// every hit ties on relevance, so the assertion is the exact seeded order.
func TestIntegrationSortOrdersWholeResultSet(t *testing.T) {
	seed(t)

	// The corpus is seeded with strictly increasing UpdatedAt, so descending
	// is the exact reverse of the seed order.
	newestFirst := []string{
		"runbooks", "CONTRIBUTING.md", "guides/setup.md",
		"docs/adr/0001-use-postgres.md", "docs/rfc/0002-tracing.md",
		"docs/rfc/0001-structured-logging.md",
	}

	desc, err := testClient.Search(t.Context(), &SearchParams{
		AllowedRepoIDs: []int64{1, 2},
		Sort:           SortUpdatedDesc,
	})
	if err != nil {
		t.Fatalf("sorted search: %v", err)
	}
	if got := paths(desc.Hits); !slices.Equal(got, newestFirst) {
		t.Errorf("updated_at:desc order =\n  %v\nwant\n  %v", got, newestFirst)
	}

	asc, err := testClient.Search(t.Context(), &SearchParams{
		AllowedRepoIDs: []int64{1, 2},
		Sort:           SortUpdatedAsc,
	})
	if err != nil {
		t.Fatalf("sorted search: %v", err)
	}
	oldestFirst := slices.Clone(newestFirst)
	slices.Reverse(oldestFirst)
	if got := paths(asc.Hits); !slices.Equal(got, oldestFirst) {
		t.Errorf("updated_at:asc order =\n  %v\nwant\n  %v", got, oldestFirst)
	}
}

// TestIntegrationSortBeatsRelevance is the ranking-rule proof. "logging"
// matches three records, and the one whose *title* carries the term
// (RFC-0001) is the most relevant — it leads an unsorted search. Sorted by
// newest, it must come last. This fails if the sort rule is returned to
// Meilisearch's default position, where it only breaks ties within relevance.
func TestIntegrationSortBeatsRelevance(t *testing.T) {
	seed(t)

	const (
		mostRelevant = "docs/rfc/0001-structured-logging.md" // title match, oldest
		newest       = "runbooks"                            // body match, newest
	)

	unsorted, err := testClient.Search(t.Context(), &SearchParams{
		Query:          "logging",
		AllowedRepoIDs: []int64{1, 2},
	})
	if err != nil {
		t.Fatalf("unsorted search: %v", err)
	}
	if got := paths(unsorted.Hits); len(got) == 0 || got[0] != mostRelevant {
		t.Fatalf("unsorted order = %v, want the title match %q first", got, mostRelevant)
	}

	sorted, err := testClient.Search(t.Context(), &SearchParams{
		Query:          "logging",
		AllowedRepoIDs: []int64{1, 2},
		Sort:           SortUpdatedDesc,
	})
	if err != nil {
		t.Fatalf("sorted search: %v", err)
	}
	want := []string{newest, "docs/adr/0001-use-postgres.md", mostRelevant}
	if got := paths(sorted.Hits); !slices.Equal(got, want) {
		t.Errorf("updated_at:desc over a query =\n  %v\nwant\n  %v "+
			"(a total order, so the most relevant hit is last)", got, want)
	}
}

// TestIntegrationCreatedSortEdges pins what happens to records with no
// authored date — the api-block pages, whose created is "".
//
// Meilisearch places them LAST in both directions, not at the low end of a
// lexicographic string order. It treats an empty value as absent for sorting
// purposes and parks such documents at the end of the results whichever way
// the sort runs. DESIGN-0005 predicted "first ascending, last descending";
// this test found otherwise and the contract description was corrected to
// match. The behavior is the better one for a "newest first" listing —
// undated records never crowd the top — but it is Meilisearch's to define,
// so it is pinned here.
func TestIntegrationCreatedSortEdges(t *testing.T) {
	seed(t)

	// Dated documents, oldest to newest authored date.
	docsAsc := []string{
		"docs/rfc/0001-structured-logging.md", // 2026-01-15
		"docs/rfc/0002-tracing.md",            // 2026-02-01
		"docs/adr/0001-use-postgres.md",       // 2026-03-01
	}
	docsDesc := slices.Clone(docsAsc)
	slices.Reverse(docsDesc)

	for _, tc := range []struct {
		name string
		sort string
		want []string
	}{
		{"descending", SortCreatedDesc, docsDesc},
		{"ascending", SortCreatedAsc, docsAsc},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := testClient.Search(t.Context(), &SearchParams{
				AllowedRepoIDs: []int64{1, 2},
				Sort:           tc.sort,
			})
			if err != nil {
				t.Fatalf("created sort search: %v", err)
			}
			if len(res.Hits) != 6 {
				t.Fatalf("got %d hits, want the whole corpus", len(res.Hits))
			}
			if got := paths(res.Hits)[:3]; !slices.Equal(got, tc.want) {
				t.Errorf("%s leading hits = %v, want the dated documents %v", tc.name, got, tc.want)
			}
			// The undated pages trail in both directions.
			for _, h := range res.Hits[3:] {
				if h.Source != SourcePage {
					t.Errorf("%s trailing hit %q is a %s, want the undated pages last",
						tc.name, h.Path, h.Source)
				}
			}
		})
	}
}

// TestIntegrationSecondarySortKey proves the tie-break. The shared corpus has
// distinct stamps, so it can never exercise one; this indexes two records
// that share an updated_at to the second — the shape every repository's first
// ingest produces, since one reconcile is one transaction — and asserts the
// requested sort falls through to created rather than to Meilisearch's
// internal order.
func TestIntegrationSecondarySortKey(t *testing.T) {
	seed(t)

	const sharedStamp = 1750619999
	tied := []IndexDoc{
		{
			ID: "9_TIE-0001", Source: SourceDoc, Repo: "tie/repo", RepoID: 9, DocID: "TIE-0001",
			Type: "rfc", Title: "Tied older", Created: "2026-05-01",
			Path: "docs/rfc/0001-tied-older.md", Body: "tiebreaker corpus",
			UpdatedAt: sharedStamp,
		},
		{
			ID: "9_TIE-0002", Source: SourceDoc, Repo: "tie/repo", RepoID: 9, DocID: "TIE-0002",
			Type: "rfc", Title: "Tied newer", Created: "2026-06-01",
			Path: "docs/rfc/0002-tied-newer.md", Body: "tiebreaker corpus",
			UpdatedAt: sharedStamp,
		},
	}
	if err := testClient.IndexDocuments(t.Context(), tied); err != nil {
		t.Fatalf("index tied docs: %v", err)
	}
	t.Cleanup(func() {
		// The index is shared across this package's tests; leaving these in
		// would break every corpus-count assertion.
		if err := testClient.DeleteDocuments(context.Background(), []string{"9_TIE-0001", "9_TIE-0002"}); err != nil {
			t.Errorf("clean up tied docs: %v", err)
		}
	})

	for _, tc := range []struct {
		name string
		sort string
		want []string
	}{
		{
			name: "newest updated breaks ties toward the newer created",
			sort: SortUpdatedDesc,
			want: []string{"docs/rfc/0002-tied-newer.md", "docs/rfc/0001-tied-older.md"},
		},
		{
			name: "oldest updated breaks ties toward the older created",
			sort: SortUpdatedAsc,
			want: []string{"docs/rfc/0001-tied-older.md", "docs/rfc/0002-tied-newer.md"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := testClient.Search(t.Context(), &SearchParams{
				Query:          "tiebreaker corpus",
				AllowedRepoIDs: []int64{9},
				Sort:           tc.sort,
			})
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			if got := paths(res.Hits); !slices.Equal(got, tc.want) {
				t.Errorf("order = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestIntegrationSourceFilter narrows to one record kind, with the counts
// coming from the server rather than from client-side filtering.
func TestIntegrationSourceFilter(t *testing.T) {
	seed(t)

	for _, tc := range []struct {
		source string
		want   int
	}{
		{SourceDoc, 3},
		{SourcePage, 3},
	} {
		t.Run(tc.source, func(t *testing.T) {
			res, err := testClient.Search(t.Context(), &SearchParams{
				AllowedRepoIDs: []int64{1, 2},
				Source:         tc.source,
			})
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			if int(res.EstimatedTotal) != tc.want {
				t.Errorf("estimated_total_hits = %d, want %d", res.EstimatedTotal, tc.want)
			}
			for _, h := range res.Hits {
				if h.Source != tc.source {
					t.Errorf("hit %q has source %q, want only %q", h.Path, h.Source, tc.source)
				}
			}
		})
	}
}

// TestIntegrationEnsureIndexAppliesRankingRules proves the deploy path: the
// ranking-rule order is applied to an index that already exists and holds
// documents, not only to a freshly created one.
func TestIntegrationEnsureIndexAppliesRankingRules(t *testing.T) {
	seed(t)

	if err := testClient.EnsureIndex(t.Context()); err != nil {
		t.Fatalf("EnsureIndex on a populated index: %v", err)
	}
	got, err := testClient.svc.Index(indexUID).GetRankingRulesWithContext(t.Context())
	if err != nil {
		t.Fatalf("read ranking rules: %v", err)
	}
	if !slices.Equal(*got, rankingRules) {
		t.Errorf("ranking rules = %v, want %v", *got, rankingRules)
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
