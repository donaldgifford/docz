//go:build integration

package store

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// testStore is a package-wide Store backed by one Postgres container, shared
// across the integration tests. Each test isolates itself by using a unique
// repo (repos are keyed by owner/name; doc types and documents by repo id).
var testStore *Store

func TestMain(m *testing.M) {
	os.Exit(runMain(m))
}

// runMain owns the container lifecycle so its defers run before os.Exit.
func runMain(m *testing.M) int {
	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("docz_api"),
		postgres.WithUsername("docz"),
		postgres.WithPassword("secret"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		log.Printf("start postgres container: %v", err)
		return 1
	}
	defer func() {
		if terr := ctr.Terminate(ctx); terr != nil {
			log.Printf("terminate container: %v", terr)
		}
	}()

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Printf("connection string: %v", err)
		return 1
	}
	if err := Migrate(ctx, dsn); err != nil {
		log.Printf("apply migrations: %v", err)
		return 1
	}
	pool, err := NewPool(ctx, dsn)
	if err != nil {
		log.Printf("open pool: %v", err)
		return 1
	}
	defer pool.Close()
	testStore = NewStore(pool)

	return m.Run()
}

// seedInstallation upserts an installation so repos referencing it satisfy the
// foreign key.
func seedInstallation(t *testing.T, id int64) {
	t.Helper()
	if err := testStore.UpsertInstallation(context.Background(), InstallationInput{
		ID:           id,
		AccountLogin: "acme",
		AccountType:  "Organization",
	}); err != nil {
		t.Fatalf("seed installation %d: %v", id, err)
	}
}

// doc builds a DocumentInput with a distinct doc id and content hash.
func doc(docID, hash string) DocumentInput {
	return DocumentInput{
		Type:        "framework",
		DocID:       docID,
		Title:       "Doc " + docID,
		Status:      "accepted",
		Author:      "alice",
		Created:     time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
		Path:        "docs/frameworks/" + docID + ".md",
		GitSHA:      "sha-" + docID,
		ContentHash: hash,
		RawMD:       "# " + docID,
	}
}

func TestPingIntegration(t *testing.T) {
	if err := testStore.Ping(t.Context()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestReconcileRepoCreatesRows(t *testing.T) {
	ctx := t.Context()
	seedInstallation(t, 100)

	in := &ReconcileInput{
		Repo: RepoInput{
			InstallationID: 100,
			Owner:          "acme",
			Name:           "create",
			DefaultBranch:  "main",
			DocsDir:        "docs",
			ConfigSnapshot: json.RawMessage(`{"types":["framework"]}`),
			LastSyncedSHA:  "head-1",
			ChangelogMD:    "# Changelog\n\n- init",
			ChangelogSHA:   "chsha-1",
		},
		DocTypes: []DocTypeInput{
			{
				Name: "framework", Dir: "frameworks", IDPrefix: "FRM", PluralLabel: "Frameworks",
				Statuses: json.RawMessage(`["draft","accepted"]`), Aliases: json.RawMessage(`["fw"]`),
			},
			{
				Name: "guide", Dir: "guides", IDPrefix: "GDE", PluralLabel: "Guides",
				Statuses: json.RawMessage(`["draft"]`), Aliases: json.RawMessage(`[]`),
			},
		},
		Documents: []DocumentInput{doc("0001", "h1"), doc("0002", "h2")},
	}

	res, err := testStore.ReconcileRepo(ctx, in)
	if err != nil {
		t.Fatalf("ReconcileRepo: %v", err)
	}
	if res.RepoID == 0 {
		t.Error("RepoID = 0, want a generated id")
	}
	if res.DocsUpserted != 2 || res.TypesUpserted != 2 {
		t.Errorf("upserts = %d docs / %d types, want 2 / 2", res.DocsUpserted, res.TypesUpserted)
	}
	if res.DocsUnchanged != 0 || res.DocsDeleted != 0 || res.TypesDeleted != 0 {
		t.Errorf("unexpected non-zero unchanged/deleted: %+v", res)
	}

	// Row counts round-trip.
	hashes, err := testStore.q.ListDocumentHashes(ctx, res.RepoID)
	if err != nil {
		t.Fatalf("ListDocumentHashes: %v", err)
	}
	if len(hashes) != 2 {
		t.Errorf("document rows = %d, want 2", len(hashes))
	}
	types, err := testStore.q.ListDocTypes(ctx, res.RepoID)
	if err != nil {
		t.Fatalf("ListDocTypes: %v", err)
	}
	if len(types) != 2 {
		t.Errorf("doc_type rows = %d, want 2", len(types))
	}

	// Repo-level fields (incl. jsonb + changelog) persist.
	repo, err := testStore.q.GetRepoByOwnerName(ctx, GetRepoByOwnerNameParams{Owner: "acme", Name: "create"})
	if err != nil {
		t.Fatalf("GetRepoByOwnerName: %v", err)
	}
	if !repo.ChangelogMd.Valid || repo.ChangelogSha.String != "chsha-1" {
		t.Errorf("changelog columns not persisted: md.valid=%v sha=%q", repo.ChangelogMd.Valid, repo.ChangelogSha.String)
	}
	if string(repo.ConfigSnapshot) != `{"types": ["framework"]}` && string(repo.ConfigSnapshot) != `{"types":["framework"]}` {
		t.Errorf("config_snapshot = %s, want the seeded jsonb", repo.ConfigSnapshot)
	}
	if !repo.LastSyncedAt.Valid {
		t.Error("last_synced_at not set by now()")
	}
}

// TestMigrateUpDownRoundTrip proves every migration's Down mirrors its Up: a
// fresh database migrates up, all the way back down to zero, and up again. It
// uses its own container so the shared testStore schema is never torn down.
func TestMigrateUpDownRoundTrip(t *testing.T) {
	ctx := t.Context()

	ctr, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("docz_api_roundtrip"),
		postgres.WithUsername("docz"),
		postgres.WithPassword("secret"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if terr := ctr.Terminate(context.Background()); terr != nil {
			t.Logf("terminate container: %v", terr)
		}
	})

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	if err := Migrate(ctx, dsn); err != nil {
		t.Fatalf("initial up: %v", err)
	}
	if err := MigrateDown(ctx, dsn); err != nil {
		t.Fatalf("down to zero: %v", err)
	}
	if err := Migrate(ctx, dsn); err != nil {
		t.Fatalf("re-up after down: %v", err)
	}
}

// TestReconcileRepoIndexPair covers the index_md/index_sha lifecycle: set on
// first reconcile, kept-with-NULL-body for an empty-but-present index.md, and
// cleared when the file is absent at HEAD (DESIGN-0003).
func TestReconcileRepoIndexPair(t *testing.T) {
	ctx := t.Context()
	seedInstallation(t, 400)

	repoInput := func(indexMD, indexSHA string) *ReconcileInput {
		return &ReconcileInput{
			Repo: RepoInput{
				InstallationID: 400, Owner: "acme", Name: "index", DefaultBranch: "main",
				DocsDir: "docs", ConfigSnapshot: json.RawMessage(`{}`),
				IndexMD: indexMD, IndexSHA: indexSHA,
			},
		}
	}
	fetch := func() Repo {
		t.Helper()
		repo, err := testStore.q.GetRepoByOwnerName(ctx, GetRepoByOwnerNameParams{Owner: "acme", Name: "index"})
		if err != nil {
			t.Fatalf("GetRepoByOwnerName: %v", err)
		}
		return repo
	}

	// Present: both columns persist.
	if _, err := testStore.ReconcileRepo(ctx, repoInput("# Home\n", "idxsha-1")); err != nil {
		t.Fatalf("reconcile with index: %v", err)
	}
	repo := fetch()
	if repo.IndexMd.String != "# Home\n" || repo.IndexSha.String != "idxsha-1" {
		t.Errorf("index pair = (%q valid=%v, %q), want the seeded values",
			repo.IndexMd.String, repo.IndexMd.Valid, repo.IndexSha.String)
	}

	// Empty-but-present: body NULL (textOrNull), sha stays valid — the
	// presence signal the API keys off.
	if _, err := testStore.ReconcileRepo(ctx, repoInput("", "idxsha-2")); err != nil {
		t.Fatalf("reconcile with empty index: %v", err)
	}
	repo = fetch()
	if repo.IndexMd.Valid {
		t.Errorf("IndexMd.Valid = true for empty body, want NULL")
	}
	if repo.IndexSha.String != "idxsha-2" {
		t.Errorf("IndexSha = %q, want idxsha-2 (presence signal)", repo.IndexSha.String)
	}

	// Absent at HEAD (file deleted): both cleared.
	if _, err := testStore.ReconcileRepo(ctx, repoInput("", "")); err != nil {
		t.Fatalf("reconcile without index: %v", err)
	}
	repo = fetch()
	if repo.IndexMd.Valid || repo.IndexSha.Valid {
		t.Errorf("index pair not cleared: md.valid=%v sha.valid=%v", repo.IndexMd.Valid, repo.IndexSha.Valid)
	}
}

func TestReconcileContentHashGate(t *testing.T) {
	ctx := t.Context()
	seedInstallation(t, 200)

	base := &ReconcileInput{
		Repo: RepoInput{
			InstallationID: 200, Owner: "acme", Name: "gate", DefaultBranch: "main",
			DocsDir: "docs", ConfigSnapshot: json.RawMessage(`{}`),
		},
		Documents: []DocumentInput{doc("0001", "h1"), doc("0002", "h2")},
	}

	if _, err := testStore.ReconcileRepo(ctx, base); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}

	// Identical content: everything gated as unchanged.
	res, err := testStore.ReconcileRepo(ctx, base)
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if res.DocsUnchanged != 2 || res.DocsUpserted != 0 {
		t.Errorf("identical reconcile = %d unchanged / %d upserted, want 2 / 0", res.DocsUnchanged, res.DocsUpserted)
	}

	// One document's content changes: only that one is rewritten.
	changed := &ReconcileInput{
		Repo:      base.Repo,
		Documents: []DocumentInput{doc("0001", "h1-v2"), doc("0002", "h2")},
	}
	res, err = testStore.ReconcileRepo(ctx, changed)
	if err != nil {
		t.Fatalf("changed reconcile: %v", err)
	}
	if res.DocsUpserted != 1 || res.DocsUnchanged != 1 {
		t.Errorf("changed reconcile = %d upserted / %d unchanged, want 1 / 1", res.DocsUpserted, res.DocsUnchanged)
	}
}

func TestReconcileDeletesAbsent(t *testing.T) {
	ctx := t.Context()
	seedInstallation(t, 300)

	full := &ReconcileInput{
		Repo: RepoInput{
			InstallationID: 300, Owner: "acme", Name: "del", DefaultBranch: "main",
			DocsDir: "docs", ConfigSnapshot: json.RawMessage(`{}`),
		},
		DocTypes: []DocTypeInput{
			{
				Name: "framework", Dir: "frameworks", IDPrefix: "FRM", PluralLabel: "Frameworks",
				Statuses: json.RawMessage(`[]`), Aliases: json.RawMessage(`[]`),
			},
			{
				Name: "guide", Dir: "guides", IDPrefix: "GDE", PluralLabel: "Guides",
				Statuses: json.RawMessage(`[]`), Aliases: json.RawMessage(`[]`),
			},
		},
		Documents: []DocumentInput{doc("0001", "h1"), doc("0002", "h2"), doc("0003", "h3")},
	}
	first, err := testStore.ReconcileRepo(ctx, full)
	if err != nil {
		t.Fatalf("full reconcile: %v", err)
	}

	// HEAD now drops one document and one doc type.
	trimmed := &ReconcileInput{
		Repo: full.Repo,
		DocTypes: []DocTypeInput{
			{
				Name: "framework", Dir: "frameworks", IDPrefix: "FRM", PluralLabel: "Frameworks",
				Statuses: json.RawMessage(`[]`), Aliases: json.RawMessage(`[]`),
			},
		},
		Documents: []DocumentInput{doc("0001", "h1"), doc("0002", "h2")},
	}
	res, err := testStore.ReconcileRepo(ctx, trimmed)
	if err != nil {
		t.Fatalf("trimmed reconcile: %v", err)
	}
	if res.DocsDeleted != 1 {
		t.Errorf("DocsDeleted = %d, want 1", res.DocsDeleted)
	}
	if res.TypesDeleted != 1 {
		t.Errorf("TypesDeleted = %d, want 1", res.TypesDeleted)
	}

	hashes, err := testStore.q.ListDocumentHashes(ctx, first.RepoID)
	if err != nil {
		t.Fatalf("ListDocumentHashes: %v", err)
	}
	if len(hashes) != 2 {
		t.Errorf("document rows after delete = %d, want 2", len(hashes))
	}
	types, err := testStore.q.ListDocTypes(ctx, first.RepoID)
	if err != nil {
		t.Fatalf("ListDocTypes: %v", err)
	}
	if len(types) != 1 {
		t.Errorf("doc_type rows after delete = %d, want 1", len(types))
	}
}
