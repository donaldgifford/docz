//go:build integration

package e2e

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/internal/config"
	"github.com/donaldgifford/docz/v2/internal/export"
	"github.com/donaldgifford/docz/v2/internal/ingest"
	"github.com/donaldgifford/docz/v2/internal/queue"
	"github.com/donaldgifford/docz/v2/internal/store"
	"github.com/donaldgifford/docz/v2/pkg/export/confluence/confluencetest"
)

// confluenceConfig is a repository config exporting to the test site's
// DOCZ space in a folder named widgets; enabled toggles the block.
func confluenceConfig(enabled bool) string {
	on := "false"
	if enabled {
		on = "true"
	}

	return "---\ndocs_dir: docs\nsync:\n  confluence:\n    enabled: " + on +
		"\n    site: " + confluencetest.DefaultURL + "\n    space: DOCZ\n    folder: widgets\n"
}

// capturedExports records the export jobs an ingest enqueues.
type capturedExports struct{ jobs []queue.ExportJob }

func (c *capturedExports) EnqueueExport(_ context.Context, job *queue.ExportJob) error {
	c.jobs = append(c.jobs, *job)
	return nil
}

// confluenceSnap is the fixture repository at one commit.
func confluenceSnap(head string, enabled bool, docs map[string][]byte) *ingest.RepoSnapshot {
	snap := &ingest.RepoSnapshot{
		HeadSHA: head, DefaultBranch: "main", ConfigYAML: []byte(confluenceConfig(enabled)),
	}

	paths := make([]string, 0, len(docs))
	for p := range docs {
		paths = append(paths, p)
	}
	slices.Sort(paths)

	for _, p := range paths {
		snap.Blobs = append(snap.Blobs, ingest.BlobEntry{Path: p, GitSHA: head + p, Content: docs[p]})
	}

	return snap
}

// ingestAndExport ingests snap and returns the export jobs it enqueued.
func ingestConfluence(t *testing.T, instID int64, snap *ingest.RepoSnapshot) []queue.ExportJob {
	t.Helper()

	captured := &capturedExports{}
	svc := ingest.NewService(testStore, staticFetcher{snap: snap}, nil).WithExporter(captured)

	if _, err := svc.Run(t.Context(), instID, fixtureOwner, "widgets"); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	return captured.jobs
}

// pageRows maps each recorded key to its row.
func pageRows(t *testing.T, repoID int64) map[string]store.ConfluencePage {
	t.Helper()

	rows, err := testStore.ListConfluencePages(t.Context(), repoID)
	if err != nil {
		t.Fatal(err)
	}

	out := make(map[string]store.ConfluencePage, len(rows))
	for _, r := range rows {
		out[r.Key] = r
	}

	return out
}

// TestE2EConfluenceExport onboards a repository with the block enabled and
// follows it through an export, a rename, a removal, and disabling the
// block, against real Postgres and confluencetest over HTTP (IMPL-0024
// Phase 7).
func TestE2EConfluenceExport(t *testing.T) {
	const instID = 7300

	if err := testStore.UpsertInstallation(t.Context(), store.InstallationInput{
		ID: instID, AccountLogin: fixtureOwner, AccountType: "Organization",
	}); err != nil {
		t.Fatalf("seed installation: %v", err)
	}

	site := confluencetest.New("DOCZ")
	site.RequireAuth("bot@example.com", "token")

	allow := &config.ConfluenceConfig{
		Site: confluencetest.DefaultURL, Email: "bot@example.com", APIToken: "token", Spaces: []string{"DOCZ"},
	}
	svc := export.NewService(testStore, site.HTTPClient(t, "bot@example.com", "token"), allow, "e2e")

	docs := map[string][]byte{
		"docs/rfc/0001-first.md":  doc("RFC-0001", "First", "# RFC-0001: First\n\nOne."),
		"docs/rfc/0002-second.md": doc("RFC-0002", "Second", "# RFC-0002: Second\n\nTwo."),
	}

	// 1. An ingest enqueues exactly one export; running it builds the tree.
	jobs := ingestConfluence(t, instID, confluenceSnap("c1", true, docs))
	if len(jobs) != 1 || jobs[0].Owner != fixtureOwner || jobs[0].Name != "widgets" {
		t.Fatalf("enqueued %+v, want one export of acme/widgets", jobs)
	}

	repoID := jobs[0].RepoID

	res, err := svc.Run(t.Context(), repoID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	if res.Status != export.StatusSucceeded || res.Folder != "widgets" {
		t.Fatalf("result %+v", res)
	}

	folder := site.FolderByTitle("widgets")
	rfcs := site.PageByTitle("widgets: RFCs")
	first := site.PageByTitle("widgets: RFC-0001: First")

	if folder == nil || rfcs == nil || first == nil || rfcs.ParentID != folder.ID || first.ParentID != rfcs.ID {
		t.Fatalf("tree: folder %+v, RFCs %+v, RFC-0001 %+v", folder, rfcs, first)
	}

	rows := pageRows(t, repoID)
	if r := rows["RFC-0001"]; r.PageID != first.ID || r.Url == "" || r.Action != "created" || r.DocID != "RFC-0001" {
		t.Errorf("RFC-0001 row %+v", r)
	}

	sync, err := testStore.GetConfluenceSync(t.Context(), repoID)
	if err != nil {
		t.Fatal(err)
	}

	if sync.Status != "succeeded" || sync.FolderID != folder.ID || sync.HeadSha != "c1" || !sync.FinishedAt.Valid {
		t.Errorf("sync %+v", sync)
	}

	// The document read now carries the page URL.
	doc1, err := testStore.GetDocumentByID(t.Context(), repoID, "RFC-0001")
	if err != nil {
		t.Fatal(err)
	}

	if doc1.ConfluenceUrl != rows["RFC-0001"].Url {
		t.Errorf("confluence_url %q, want %q", doc1.ConfluenceUrl, rows["RFC-0001"].Url)
	}

	// 2. Renaming a document renames its page in place.
	docs["docs/rfc/0001-first.md"] = doc("RFC-0001", "Renamed", "# RFC-0001: Renamed\n\nOne.")
	ingestConfluence(t, instID, confluenceSnap("c2", true, docs))

	if _, err := svc.Run(t.Context(), repoID); err != nil {
		t.Fatalf("export after rename: %v", err)
	}

	renamed := site.PageByTitle("widgets: RFC-0001: Renamed")
	if renamed == nil || renamed.ID != first.ID || site.PageByTitle("widgets: RFC-0001: First") != nil {
		t.Errorf("rename: %+v, want page %s retitled", renamed, first.ID)
	}

	if archive := site.PageByTitle("widgets: Archive"); archive != nil {
		t.Errorf("rename archived something: %+v", archive)
	}

	// 3. Removing a document archives its page.
	second := site.PageByTitle("widgets: RFC-0002: Second")
	delete(docs, "docs/rfc/0002-second.md")
	ingestConfluence(t, instID, confluenceSnap("c3", true, docs))

	if _, err := svc.Run(t.Context(), repoID); err != nil {
		t.Fatalf("export after removal: %v", err)
	}

	archive := site.PageByTitle("widgets: Archive")
	moved := site.PageByTitle("widgets: RFC-0002: Second")

	if archive == nil || moved == nil || moved.ID != second.ID || moved.ParentID != archive.ID {
		t.Errorf("removal: archive %+v, page %+v", archive, moved)
	}

	if r := pageRows(t, repoID)["RFC-0002"]; r.Action != "archived" {
		t.Errorf("RFC-0002 row %+v, want archived", r)
	}

	// 4. Disabling the block enqueues nothing, and an export already queued
	// records disabled and writes nothing.
	site.Writes()

	if jobs := ingestConfluence(t, instID, confluenceSnap("c4", false, docs)); len(jobs) != 0 {
		t.Errorf("disabled block enqueued %+v", jobs)
	}

	res, err = svc.Run(t.Context(), repoID)
	if err != nil || res.Status != export.StatusDisabled {
		t.Errorf("disabled run: %+v, %v", res, err)
	}

	if w := site.Writes(); len(w) != 0 {
		t.Errorf("disabled run wrote %v", w)
	}

	sync, err = testStore.GetConfluenceSync(t.Context(), repoID)
	if err != nil || sync.Status != "disabled" || !strings.HasPrefix(sync.FolderTitle, "widgets") {
		t.Errorf("sync after disable %+v, %v", sync, err)
	}
}
