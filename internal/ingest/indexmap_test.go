package ingest

import (
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/donaldgifford/docz-api/internal/search"
	"github.com/donaldgifford/docz-api/internal/store"
)

// TestPagePrimaryKey pins the page key shape "<repo_id>_p_<16 hex>": stable
// for a given path, distinct across paths and repos, and made only of
// Meilisearch-legal id characters (the reason the path is hashed at all).
func TestPagePrimaryKey(t *testing.T) {
	t.Parallel()
	key := pagePrimaryKey(7, "guides/setup.md")
	if !strings.HasPrefix(key, "7_p_") || len(key) != len("7_p_")+pageHashLen {
		t.Fatalf("key = %q, want 7_p_<%d hex chars>", key, pageHashLen)
	}
	if key != pagePrimaryKey(7, "guides/setup.md") {
		t.Error("key not stable for the same path")
	}
	if key == pagePrimaryKey(7, "guides/other.md") {
		t.Error("distinct paths share a key")
	}
	if key == pagePrimaryKey(8, "guides/setup.md") {
		t.Error("distinct repos share a key")
	}
	for _, r := range key {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			t.Fatalf("key %q carries a Meilisearch-illegal id char %q", key, r)
		}
	}
}

// TestToIndexPage pins the page record shape: Source "page", the published
// path, empty doc-only fields.
func TestToIndexPage(t *testing.T) {
	t.Parallel()
	updated := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	page := store.RepoPage{
		RepoID:    3,
		Path:      "guides",
		RepoPath:  "docs/guides/README.md",
		Title:     "Guides",
		RawMd:     "# Guides",
		UpdatedAt: pgtype.Timestamptz{Time: updated, Valid: true},
	}

	got := toIndexPage("acme", "widgets", 3, &page)
	want := search.IndexDoc{
		ID:        pagePrimaryKey(3, "guides"),
		Source:    search.SourcePage,
		Repo:      "acme/widgets",
		RepoID:    3,
		Title:     "Guides",
		Path:      "guides",
		Body:      "# Guides",
		UpdatedAt: updated.Unix(),
	}
	if got != want {
		t.Errorf("toIndexPage = %+v, want %+v (doc-only fields empty)", got, want)
	}
}

// TestToIndexDocCarriesSourceAndPath pins the doc mapping's Phase 6 additions
// without re-proving the Phase 3 fields.
func TestToIndexDocCarriesSourceAndPath(t *testing.T) {
	t.Parallel()
	doc := store.Document{DocID: "RFC-0001", Path: "docs/rfc/0001-intro.md", RawMd: "# RFC"}
	got := toIndexDoc("acme", "widgets", 3, &doc)
	if got.Source != search.SourceDoc || got.Path != "docs/rfc/0001-intro.md" {
		t.Errorf("doc mapping = {Source:%q Path:%q}, want {doc docs/rfc/0001-intro.md}",
			got.Source, got.Path)
	}
}
