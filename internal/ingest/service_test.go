package ingest

import (
	"context"
	"testing"

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
// for the real store.
type captureReconciler struct {
	in *store.ReconcileInput
}

func (c *captureReconciler) ReconcileRepo(
	_ context.Context, in *store.ReconcileInput,
) (store.ReconcileResult, error) {
	c.in = in
	return store.ReconcileResult{
		RepoID:        1,
		DocsUpserted:  len(in.Documents),
		TypesUpserted: len(in.DocTypes),
	}, nil
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
	svc := NewService(rec, fakeFetcher{snap: snap})

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
