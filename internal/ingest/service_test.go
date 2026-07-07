package ingest

import (
	"context"
	"errors"
	"testing"

	"github.com/donaldgifford/docz-api/internal/search"
	"github.com/donaldgifford/docz-api/internal/store"
)

// fakeFetcher returns a fixed snapshot, so the pipeline is exercised with no
// network.
type fakeFetcher struct {
	snap *RepoSnapshot
	err  error
}

func (f fakeFetcher) Fetch(context.Context, string, string) (*RepoSnapshot, error) {
	return f.snap, f.err
}

// captureReconciler records the ReconcileInput the service builds, standing in
// for the real store. It reports every input document as upserted so the
// indexing path is exercised, and answers GetDocumentsByIDs from that input.
type captureReconciler struct {
	in *store.ReconcileInput
}

func (c *captureReconciler) ReconcileRepo(
	_ context.Context, in *store.ReconcileInput,
) (store.ReconcileResult, error) {
	c.in = in
	res := store.ReconcileResult{
		RepoID:        1,
		DocsUpserted:  len(in.Documents),
		TypesUpserted: len(in.DocTypes),
	}
	for i := range in.Documents {
		res.UpsertedDocIDs = append(res.UpsertedDocIDs, in.Documents[i].DocID)
	}
	return res, nil
}

func (c *captureReconciler) GetDocumentsByIDs(
	_ context.Context, _ int64, docIDs []string,
) ([]store.Document, error) {
	out := make([]store.Document, 0, len(docIDs))
	if c.in == nil {
		return out, nil
	}
	for _, id := range docIDs {
		for i := range c.in.Documents {
			d := &c.in.Documents[i]
			if d.DocID == id {
				out = append(out, store.Document{
					RepoID: 1,
					DocID:  d.DocID,
					Type:   d.Type,
					Title:  d.Title,
					RawMd:  d.RawMD,
				})
				break
			}
		}
	}
	return out, nil
}

// captureIndexer records the documents and ids passed to the search indexer.
type captureIndexer struct {
	indexed []search.IndexDoc
	deleted []string
}

func (c *captureIndexer) IndexDocuments(_ context.Context, docs []search.IndexDoc) error {
	c.indexed = append(c.indexed, docs...)
	return nil
}

func (c *captureIndexer) DeleteDocuments(_ context.Context, ids []string) error {
	c.deleted = append(c.deleted, ids...)
	return nil
}

// failIndexer fails every write, to prove indexing errors don't fail ingest.
type failIndexer struct{}

func (failIndexer) IndexDocuments(context.Context, []search.IndexDoc) error {
	return errors.New("index boom")
}

func (failIndexer) DeleteDocuments(context.Context, []string) error {
	return errors.New("delete boom")
}

const fixtureConfig = `---
docs_dir: docs
types:
  frameworks:
    enabled: true
    dir: frameworks
    id_prefix: FW
    id_width: 4
    statuses:
      - Draft
      - Adopted
    aliases:
      - fw
`

const fixtureDoc = `---
id: FW-0001
title: Example Framework
status: Draft
author: Test Author
created: 2026-07-01
---

# FW 0001: Example Framework
`

func TestRunMapsCustomTypeAndSkipsMissingFrontmatter(t *testing.T) {
	// loadConfig calls doczcfg.Load, which merges $HOME/.docz.yaml; neutralize it.
	t.Setenv("HOME", t.TempDir())

	snap := &RepoSnapshot{
		HeadSHA:       "head-sha",
		DefaultBranch: "main",
		ConfigYAML:    []byte(fixtureConfig),
		ChangelogMD:   []byte("# Changelog\n"),
		ChangelogSHA:  "cl-sha",
		Blobs: []BlobEntry{
			{Path: "docs/frameworks/0001-intro.md", GitSHA: "d1", Content: []byte(fixtureDoc)},
			{Path: "docs/frameworks/0002-nofm.md", GitSHA: "d2", Content: []byte("# no frontmatter here\n")},
			{Path: "docs/notatype/0001-stray.md", GitSHA: "d3", Content: []byte(fixtureDoc)},
		},
	}
	rec := &captureReconciler{}
	svc := NewService(rec, fakeFetcher{snap: snap}, nil)

	res, err := svc.Run(t.Context(), 42, "acme", "platform")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rec.in == nil {
		t.Fatal("reconciler was not called")
	}

	// Repo-level mapping, including the cached changelog.
	repo := rec.in.Repo
	if repo.InstallationID != 42 || repo.Owner != "acme" || repo.Name != "platform" {
		t.Errorf("repo identity = %+v", repo)
	}
	if repo.DefaultBranch != "main" || repo.LastSyncedSHA != "head-sha" || repo.DocsDir != "docs" {
		t.Errorf("repo fields = %+v", repo)
	}
	if repo.ChangelogMD != "# Changelog\n" || repo.ChangelogSHA != "cl-sha" {
		t.Errorf("changelog = %q / %q, want cached raw", repo.ChangelogMD, repo.ChangelogSHA)
	}
	if len(repo.ConfigSnapshot) == 0 {
		t.Error("ConfigSnapshot is empty, want the marshaled config")
	}

	// The custom type is mapped from .docz.yaml.
	if len(rec.in.DocTypes) != 1 || rec.in.DocTypes[0].Name != "frameworks" {
		t.Fatalf("DocTypes = %+v, want one 'frameworks'", rec.in.DocTypes)
	}
	if rec.in.DocTypes[0].IDPrefix != "FW" {
		t.Errorf("id_prefix = %q, want FW", rec.in.DocTypes[0].IDPrefix)
	}

	// Only the valid doc survives: nofm is skipped, the stray dir is ignored.
	if len(rec.in.Documents) != 1 {
		t.Fatalf("Documents = %d, want 1 (nofm skipped, stray dir ignored)", len(rec.in.Documents))
	}
	doc := rec.in.Documents[0]
	if doc.DocID != "FW-0001" || doc.Type != "frameworks" || doc.ContentHash == "" {
		t.Errorf("mapped doc = %+v", doc)
	}
	if res.DocsUpserted != 1 || res.TypesUpserted != 1 {
		t.Errorf("result = %+v, want 1 doc / 1 type", res)
	}
}

func TestRunIndexesUpsertedDocuments(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	snap := &RepoSnapshot{
		HeadSHA:       "head-sha",
		DefaultBranch: "main",
		ConfigYAML:    []byte(fixtureConfig),
		Blobs: []BlobEntry{
			{Path: "docs/frameworks/0001-intro.md", GitSHA: "d1", Content: []byte(fixtureDoc)},
		},
	}
	rec := &captureReconciler{}
	idx := &captureIndexer{}
	svc := NewService(rec, fakeFetcher{snap: snap}, idx)

	if _, err := svc.Run(t.Context(), 42, "acme", "platform"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(idx.indexed) != 1 {
		t.Fatalf("indexed %d docs, want 1", len(idx.indexed))
	}
	got := idx.indexed[0]
	if got.ID != "1_FW-0001" || got.Repo != "acme/platform" || got.RepoID != 1 {
		t.Errorf("index doc identity = %+v", got)
	}
	if got.Type != "frameworks" || got.DocID != "FW-0001" || got.Title == "" || got.Body == "" {
		t.Errorf("index doc fields = %+v", got)
	}
	if len(idx.deleted) != 0 {
		t.Errorf("deleted = %v, want none", idx.deleted)
	}
}

func TestRunIndexErrorDoesNotFailIngest(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	snap := &RepoSnapshot{
		HeadSHA:       "head-sha",
		DefaultBranch: "main",
		ConfigYAML:    []byte(fixtureConfig),
		Blobs: []BlobEntry{
			{Path: "docs/frameworks/0001-intro.md", GitSHA: "d1", Content: []byte(fixtureDoc)},
		},
	}
	svc := NewService(&captureReconciler{}, fakeFetcher{snap: snap}, failIndexer{})

	// Postgres has committed; a search-index failure must not fail the ingest.
	if _, err := svc.Run(t.Context(), 42, "acme", "platform"); err != nil {
		t.Fatalf("Run should tolerate an index failure, got: %v", err)
	}
}
