package ingest

import (
	"context"
	"encoding/json"
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
// for the real store. It reports every input document and page as upserted so
// the indexing path is exercised, and answers GetDocumentsByIDs and
// GetRepoPagesByPaths from that input.
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
		PagesUpserted: len(in.Pages),
	}
	for i := range in.Documents {
		res.UpsertedDocIDs = append(res.UpsertedDocIDs, in.Documents[i].DocID)
	}
	for i := range in.Pages {
		res.UpsertedPagePaths = append(res.UpsertedPagePaths, in.Pages[i].Path)
	}
	return res, nil
}

func (c *captureReconciler) GetRepoPagesByPaths(
	_ context.Context, _ int64, paths []string,
) ([]store.RepoPage, error) {
	out := make([]store.RepoPage, 0, len(paths))
	if c.in == nil {
		return out, nil
	}
	for _, path := range paths {
		for i := range c.in.Pages {
			p := &c.in.Pages[i]
			if p.Path == path {
				out = append(out, store.RepoPage{
					RepoID:   1,
					Path:     p.Path,
					RepoPath: p.RepoPath,
					Title:    p.Title,
					RawMd:    p.RawMD,
				})
				break
			}
		}
	}
	return out, nil
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
					Path:   d.Path,
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
		IndexMD:       []byte("# Home\n"),
		IndexSHA:      "idx-sha",
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
	// The fixture config has no changelog: block, so the resolved path is
	// empty and the reconcile will null the whole triple.
	if repo.ChangelogFile != "" {
		t.Errorf("ChangelogFile = %q, want empty (block not enabled)", repo.ChangelogFile)
	}
	if repo.IndexMD != "# Home\n" || repo.IndexSHA != "idx-sha" {
		t.Errorf("index = %q / %q, want cached raw", repo.IndexMD, repo.IndexSHA)
	}
	// The snapshot must carry .docz.yaml key spellings (docz v1.2.2 json
	// tags, contract clause R11), not Go field names.
	var snapshot map[string]json.RawMessage
	if err := json.Unmarshal(repo.ConfigSnapshot, &snapshot); err != nil {
		t.Fatalf("decode ConfigSnapshot: %v", err)
	}
	if _, ok := snapshot["docs_dir"]; !ok {
		t.Errorf("ConfigSnapshot missing yaml-spelled docs_dir key: %s", repo.ConfigSnapshot)
	}
	if _, ok := snapshot["DocsDir"]; ok {
		t.Error("ConfigSnapshot carries Go field name DocsDir, want yaml spellings only")
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

// TestRunMapsChangelogFile pins the opt-in rule: only an enabled changelog:
// block yields a resolved path on the repo row, and the value is the
// normalized one from the authoritative config (IMPL-0005).
func TestRunMapsChangelogFile(t *testing.T) {
	tests := []struct {
		name  string
		block string
		want  string
	}{
		{"absent block", "", ""},
		{"disabled block", "changelog:\n  enabled: false\n", ""},
		{"enabled with the default file", "changelog:\n  enabled: true\n", "CHANGELOG.md"},
		{"enabled with a chart subpath", "changelog:\n  enabled: true\n  file: charts/acme/CHANGELOG.md\n", "charts/acme/CHANGELOG.md"},
		{"enabled with a dot-slash path normalized", "changelog:\n  enabled: true\n  file: ./CHANGELOG.md\n", "CHANGELOG.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())

			snap := &RepoSnapshot{
				HeadSHA:       "head-sha",
				DefaultBranch: "main",
				ConfigYAML:    []byte(fixtureConfig + tt.block),
			}
			rec := &captureReconciler{}
			svc := NewService(rec, fakeFetcher{snap: snap}, nil)

			if _, err := svc.Run(t.Context(), 42, "acme", "platform"); err != nil {
				t.Fatalf("Run: %v", err)
			}
			if got := rec.in.Repo.ChangelogFile; got != tt.want {
				t.Errorf("ChangelogFile = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRunMapsAPIBlock pins the pages wiring (IMPL-0007): an enabled api:
// block puts the resolved landing page + additional_docs on the repo input
// and classified pages on the reconcile input; a dormant block leaves all
// three empty, and an invalid enabled block fails the whole ingest through
// the existing Validate path (no new error path).
func TestRunMapsAPIBlock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	run := func(t *testing.T, block string, blobs []BlobEntry) (*captureReconciler, error) {
		t.Helper()
		snap := &RepoSnapshot{
			HeadSHA:       "head-sha",
			DefaultBranch: "main",
			ConfigYAML:    []byte(fixtureConfig + block),
			Blobs:         blobs,
		}
		rec := &captureReconciler{}
		_, err := NewService(rec, fakeFetcher{snap: snap}, nil).Run(t.Context(), 42, "acme", "platform")
		return rec, err
	}

	t.Run("enabled block maps fields and pages", func(t *testing.T) {
		rec, err := run(t, "api:\n  enabled: true\n  additional_docs: [CONTRIBUTING.md]\n",
			[]BlobEntry{
				{Path: "docs/guides/setup.md", GitSHA: "s1", Content: []byte("# Setup")},
				{Path: "CONTRIBUTING.md", GitSHA: "s2", Content: []byte("# Contributing")},
			})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if rec.in.Repo.APILandingPage != "docs/index.md" {
			t.Errorf("APILandingPage = %q, want the backfilled docs/index.md", rec.in.Repo.APILandingPage)
		}
		if len(rec.in.Repo.APIAdditionalDocs) != 1 || rec.in.Repo.APIAdditionalDocs[0] != "CONTRIBUTING.md" {
			t.Errorf("APIAdditionalDocs = %v, want [CONTRIBUTING.md]", rec.in.Repo.APIAdditionalDocs)
		}
		if len(rec.in.Pages) != 2 {
			t.Fatalf("Pages = %d, want 2 (the guide + the additional doc)", len(rec.in.Pages))
		}
	})

	t.Run("dormant block maps nothing", func(t *testing.T) {
		rec, err := run(t, "api:\n  enabled: false\n  additional_docs: [CONTRIBUTING.md]\n",
			[]BlobEntry{{Path: "docs/guides/setup.md", GitSHA: "s1", Content: []byte("# Setup")}})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if rec.in.Repo.APILandingPage != "" || rec.in.Repo.APIAdditionalDocs != nil || rec.in.Pages != nil {
			t.Errorf("dormant block mapped (%q, %v, %d pages), want all empty",
				rec.in.Repo.APILandingPage, rec.in.Repo.APIAdditionalDocs, len(rec.in.Pages))
		}
	})

	t.Run("invalid enabled block fails the ingest", func(t *testing.T) {
		if _, err := run(t, "api:\n  enabled: true\n  landing_page: ../escape.md\n", nil); err == nil {
			t.Fatal("Run = nil error for an enabled traversal landing page, want the Validate failure")
		}
	})
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

// TestRunIndexesUpsertedPages pins Phase 6's sync path: an enabled api: block
// lands its classified pages in the search index alongside the docs, each
// record tagged with its source.
func TestRunIndexesUpsertedPages(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	snap := &RepoSnapshot{
		HeadSHA:       "head-sha",
		DefaultBranch: "main",
		ConfigYAML:    []byte(fixtureConfig + "api:\n  enabled: true\n"),
		Blobs: []BlobEntry{
			{Path: "docs/frameworks/0001-intro.md", GitSHA: "d1", Content: []byte(fixtureDoc)},
			{Path: "docs/guides/setup.md", GitSHA: "p1", Content: []byte("# Setup Guide")},
		},
	}
	rec := &captureReconciler{}
	idx := &captureIndexer{}
	svc := NewService(rec, fakeFetcher{snap: snap}, idx)

	if _, err := svc.Run(t.Context(), 42, "acme", "platform"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(idx.indexed) != 2 {
		t.Fatalf("indexed %d records, want 2 (one doc + one page)", len(idx.indexed))
	}
	bySource := make(map[string]search.IndexDoc, len(idx.indexed))
	for _, record := range idx.indexed {
		bySource[record.Source] = record
	}
	doc, ok := bySource[search.SourceDoc]
	if !ok || doc.DocID != "FW-0001" || doc.Path != "docs/frameworks/0001-intro.md" {
		t.Errorf("doc record = %+v, want FW-0001 with its repo path", doc)
	}
	page, ok := bySource[search.SourcePage]
	if !ok {
		t.Fatalf("no page record indexed; got %+v", idx.indexed)
	}
	if page.ID != pagePrimaryKey(1, "guides/setup.md") || page.Path != "guides/setup.md" {
		t.Errorf("page record = %+v, want the hashed PK + published path", page)
	}
	if page.Title != "Setup Guide" || page.Body != "# Setup Guide" || page.DocID != "" || page.Type != "" {
		t.Errorf("page record fields = %+v, want title/body set and doc-only fields empty", page)
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
