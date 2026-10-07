//go:build integration

package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// confluenceRepo reconciles a repository with the given documents and pages
// and a head sha, returning its id.
func confluenceRepo(t *testing.T, name, head string, docs []DocumentInput, pages []PageInput) int64 {
	t.Helper()

	id, err := reconcileRepo(name, head, docs, pages)
	if err != nil {
		t.Fatalf("reconcile %s: %v", name, err)
	}

	return id
}

func reconcileRepo(name, head string, docs []DocumentInput, pages []PageInput) (int64, error) {
	res, err := testStore.ReconcileRepo(context.Background(), &ReconcileInput{
		Repo: RepoInput{
			InstallationID: 700, Owner: "acme", Name: name, DefaultBranch: "main",
			DocsDir: "docs", ConfigSnapshot: json.RawMessage(`{}`), LastSyncedSHA: head,
		},
		DocTypes: []DocTypeInput{{
			Name: "framework", Dir: "frameworks", IDPrefix: "FRM", PluralLabel: "Frameworks",
			Statuses: json.RawMessage(`["draft"]`), Aliases: json.RawMessage(`[]`),
		}},
		Documents: docs,
		Pages:     pages,
	})

	return res.RepoID, err
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func TestConfluenceRecordsAndTheURLJoin(t *testing.T) {
	ctx := t.Context()
	seedInstallation(t, 700)

	repoID := confluenceRepo(t, "confluence", "head-1", []DocumentInput{doc("0001", "h1"), doc("0002", "h2")}, nil)

	if _, err := testStore.GetConfluenceSync(ctx, repoID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("sync before any run: %v; want ErrNoRows", err)
	}

	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	sync := &UpsertConfluenceSyncParams{
		RepoID: repoID, Status: "succeeded", Site: "https://example.atlassian.net", Space: "DOCZ",
		FolderID: "f1", FolderTitle: "confluence", FolderUrl: "https://example.atlassian.net/wiki/folder/f1",
		HeadSha: "head-1", Counts: json.RawMessage(`{"created":2}`), StartedAt: ts(now), FinishedAt: ts(now.Add(time.Minute)),
	}
	pages := []UpsertConfluencePageParams{
		{RepoID: repoID, Key: "0001", DocID: "0001", PageID: "p1", Title: "confluence: 0001", Url: "https://x/p1",
			Version: 1, Hash: "sha256:a", Action: "created", SyncedAt: ts(now)},
		{RepoID: repoID, Key: "docz:type:framework", PageID: "p9", Title: "confluence: Frameworks", Url: "https://x/p9",
			Version: 1, Action: "created", SyncedAt: ts(now)},
	}

	if err := testStore.RecordConfluenceExport(ctx, sync, pages); err != nil {
		t.Fatalf("RecordConfluenceExport: %v", err)
	}

	got, err := testStore.GetConfluenceSync(ctx, repoID)
	if err != nil || got.Status != "succeeded" || got.FolderID != "f1" || string(got.Counts) != `{"created": 2}` {
		t.Errorf("sync %+v %v", got, err)
	}

	list, err := testStore.ListDocumentsByType(ctx, repoID, "framework")
	if err != nil || len(list) != 2 || list[0].ConfluenceUrl != "https://x/p1" || list[1].ConfluenceUrl != "" {
		t.Errorf("list %+v %v; want 0001 linked and 0002 not", list, err)
	}

	one, err := testStore.GetDocumentByID(ctx, repoID, "0001")
	if err != nil || one.ConfluenceUrl != "https://x/p1" || one.Document.RawMd == "" {
		t.Errorf("get %+v %v; want the url and the markdown", one, err)
	}

	// A failed page keeps the link that worked.
	if err := testStore.UpsertConfluencePage(ctx, &UpsertConfluencePageParams{
		RepoID: repoID, Key: "0001", DocID: "0001", Title: "confluence: 0001", Action: "failed", Reason: "HTTP 500", SyncedAt: ts(now),
	}); err != nil {
		t.Fatal(err)
	}

	rows, err := testStore.ListConfluencePages(ctx, repoID)
	if err != nil || len(rows) != 2 || rows[0].Action != "failed" || rows[0].PageID != "p1" || rows[0].Url != "https://x/p1" {
		t.Errorf("rows %+v %v; want 0001 failed with its page kept", rows, err)
	}

	in, err := testStore.ExportInputs(ctx, repoID)
	if err != nil {
		t.Fatalf("ExportInputs: %v", err)
	}

	if in.Repo.ID != repoID || len(in.Documents) != 2 || in.Documents[0].RawMd == "" || in.Sync == nil ||
		in.PageIDs["0001"] != "p1" || in.PageIDs["docz:type:framework"] != "p9" {
		t.Errorf("ExportInputs %+v", in)
	}

	if _, err := testStore.ExportInputs(ctx, 999999); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("missing repo: %v; want ErrNoRows", err)
	}
}

// TestExportInputsIsOneSnapshot reads while a writer reconciles generation
// after generation, each changing the head sha, every document, and every
// page together. Every read must see one generation throughout: a read
// committed transaction would mix them.
func TestExportInputsIsOneSnapshot(t *testing.T) {
	ctx := t.Context()
	seedInstallation(t, 700)

	gen := func(n int) (string, []DocumentInput, []PageInput) {
		tag := fmt.Sprintf("gen-%d", n)
		docs := make([]DocumentInput, 0, 20)
		pages := make([]PageInput, 0, 20)

		for i := range 20 {
			d := doc(fmt.Sprintf("%04d", i), tag)
			d.RawMD = tag
			docs = append(docs, d)

			p := page(fmt.Sprintf("p%02d", i), tag)
			p.RawMD = tag
			pages = append(pages, p)
		}

		return tag, docs, pages
	}

	head, docs, pages := gen(0)
	repoID := confluenceRepo(t, "snapshot", head, docs, pages)

	var wg sync.WaitGroup

	done := make(chan struct{})

	wg.Go(func() {
		defer close(done)

		for n := 1; n <= 40; n++ {
			head, docs, pages := gen(n)
			if _, err := reconcileRepo("snapshot", head, docs, pages); err != nil {
				t.Errorf("reconcile generation %d: %v", n, err)

				return
			}
		}
	})

	reads := 0

	for {
		select {
		case <-done:
			wg.Wait()

			if reads == 0 {
				t.Fatal("no read overlapped the writer")
			}

			return
		default:
		}

		in, err := testStore.ExportInputs(ctx, repoID)
		if err != nil {
			t.Fatalf("ExportInputs: %v", err)
		}

		reads++
		want := in.Repo.LastSyncedSha.String

		for _, d := range in.Documents {
			if d.RawMd != want {
				t.Fatalf("read %d: document %s at %s, repo at %s", reads, d.DocID, d.RawMd, want)
			}
		}

		for _, p := range in.Pages {
			if p.RawMd != want {
				t.Fatalf("read %d: page %s at %s, repo at %s", reads, p.Path, p.RawMd, want)
			}
		}
	}
}
