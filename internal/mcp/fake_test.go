package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/donaldgifford/docz/v2/internal/search"
	"github.com/donaldgifford/docz/v2/internal/store"
)

// fakeStore is the in-memory httpapi.StoreReader the tests read through,
// the shape of internal/httpapi's own test fake.
type fakeStore struct {
	repos []store.Repo
	types map[int64][]store.DocType
	docs  map[int64][]store.Document
	pages map[int64][]store.RepoPage
	urls  map[string]string
}

func (f *fakeStore) ListRepos(context.Context) ([]store.Repo, error) { return f.repos, nil }

func (f *fakeStore) GetRepo(_ context.Context, owner, name string) (store.Repo, error) {
	for i := range f.repos {
		if f.repos[i].Owner == owner && f.repos[i].Name == name {
			return f.repos[i], nil
		}
	}

	return store.Repo{}, pgx.ErrNoRows
}

func (f *fakeStore) GetDocTypesForRepo(_ context.Context, repoID int64) ([]store.DocType, error) {
	return f.types[repoID], nil
}

func (f *fakeStore) ListDocumentsByType(
	_ context.Context, repoID int64, typeName string,
) ([]store.ListDocumentsByTypeRow, error) {
	var out []store.ListDocumentsByTypeRow
	for i := range f.docs[repoID] {
		d := f.docs[repoID][i]
		if d.Type != typeName {
			continue
		}
		out = append(out, store.ListDocumentsByTypeRow{
			ID: d.ID, RepoID: d.RepoID, Type: d.Type, DocID: d.DocID, Title: d.Title,
			Status: d.Status, Author: d.Author, Created: d.Created, Path: d.Path,
			GitSha: d.GitSha, ContentHash: d.ContentHash, UpdatedAt: d.UpdatedAt,
			ConfluenceUrl: f.urls[d.DocID],
		})
	}

	return out, nil
}

func (f *fakeStore) GetDocumentByID(_ context.Context, repoID int64, docID string) (store.GetDocumentByIDRow, error) {
	for i := range f.docs[repoID] {
		if f.docs[repoID][i].DocID == docID {
			return store.GetDocumentByIDRow{Document: f.docs[repoID][i], ConfluenceUrl: f.urls[docID]}, nil
		}
	}

	return store.GetDocumentByIDRow{}, pgx.ErrNoRows
}

func (f *fakeStore) ListRepoPages(_ context.Context, repoID int64) ([]store.ListRepoPagesRow, error) {
	out := make([]store.ListRepoPagesRow, 0, len(f.pages[repoID]))
	for i := range f.pages[repoID] {
		p := &f.pages[repoID][i]
		out = append(out, store.ListRepoPagesRow{ID: p.ID, RepoID: p.RepoID, Path: p.Path, Title: p.Title, GitSha: p.GitSha})
	}

	return out, nil
}

func (f *fakeStore) GetRepoPageByPath(_ context.Context, repoID int64, path string) (store.RepoPage, error) {
	for i := range f.pages[repoID] {
		if f.pages[repoID][i].Path == path {
			return f.pages[repoID][i], nil
		}
	}

	return store.RepoPage{}, pgx.ErrNoRows
}

func (*fakeStore) GetConfluenceSync(context.Context, int64) (store.ConfluenceSync, error) {
	return store.ConfluenceSync{}, pgx.ErrNoRows
}

func (*fakeStore) ListConfluencePages(context.Context, int64) ([]store.ConfluencePage, error) {
	return nil, nil
}

// fakeSearcher records the params it was given and answers a fixed result.
type fakeSearcher struct {
	got    search.SearchParams
	calls  int
	result search.SearchResult
	err    error
}

func (f *fakeSearcher) Search(_ context.Context, p *search.SearchParams) (search.SearchResult, error) {
	f.got = *p
	f.calls++

	return f.result, f.err
}

func validText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

// corpus reads one of the type packages' fixtures.
func corpus(t *testing.T, typeName, file string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "pkg", typeName, "testdata", file))
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}

// Repository ids: platform is visible to the test principal, hidden is
// onboarded and never allowed.
const (
	repoPlatform int64 = 1
	repoHidden   int64 = 2
)

// seededStore is a repository with the rfc and impl built-ins, a custom
// frameworks type, one document of each, a landing page, a changelog, and
// a published page, beside a repository the principal may not read.
func seededStore(t *testing.T) *fakeStore {
	t.Helper()

	return &fakeStore{
		repos: []store.Repo{
			{
				ID: repoPlatform, Owner: "acme", Name: "platform", DefaultBranch: "main", DocsDir: "docs",
				ConfigSnapshot: json.RawMessage(`{"docs_dir":"docs"}`), LastSyncedSha: validText("headsha"),
				IndexMd: validText("# Platform docs\n\nWelcome.\n"), IndexSha: validText("idxsha"),
				ChangelogMd:  validText("# Changelog\n\n## [1.0.0] - 2026-01-01\n"),
				ChangelogSha: validText("clsha"),
			},
			{
				ID: repoHidden, Owner: "acme", Name: "hidden", DefaultBranch: "main", DocsDir: "docs",
				ConfigSnapshot: json.RawMessage(`{"docs_dir":"docs"}`), LastSyncedSha: validText("hiddensha"),
			},
		},
		types: map[int64][]store.DocType{
			repoPlatform: {
				{
					ID: 10, RepoID: repoPlatform, Name: "rfc", Dir: "rfc", IDPrefix: "RFC", PluralLabel: "RFCs",
					Statuses: json.RawMessage(`["Draft","Accepted"]`), Aliases: json.RawMessage(`[]`),
				},
				{
					ID: 11, RepoID: repoPlatform, Name: "impl", Dir: "impl", IDPrefix: "IMPL", PluralLabel: "Implementation Plans",
					Statuses: json.RawMessage(`["Draft","In Progress","Completed"]`), Aliases: json.RawMessage(`["implementation"]`),
				},
				{
					ID: 12, RepoID: repoPlatform, Name: "frameworks", Dir: "frameworks", IDPrefix: "FW", PluralLabel: "Frameworks",
					Statuses: json.RawMessage(`["Draft","Adopted"]`), Aliases: json.RawMessage(`["framework"]`),
				},
			},
			repoHidden: {
				{ID: 20, RepoID: repoHidden, Name: "rfc", Dir: "rfc", IDPrefix: "RFC", PluralLabel: "RFCs"},
			},
		},
		docs: map[int64][]store.Document{
			repoPlatform: {
				{
					ID: 100, RepoID: repoPlatform, Type: "rfc", DocID: "RFC-0001", Title: "Booty RFC",
					Status: validText("Accepted"), Author: validText("Jane"),
					Path: "docs/rfc/0001-booty.md", GitSha: "rfcsha", ContentHash: "h1",
					RawMd: corpus(t, "rfc", "booty-rfc-0001.md"),
				},
				{
					ID: 101, RepoID: repoPlatform, Type: "impl", DocID: "IMPL-0007", Title: "Eliminate Redundant File Reads",
					Status: validText("In Progress"), Author: validText("Donald"),
					Path: "docs/impl/0007-eliminate.md", GitSha: "implsha", ContentHash: "h2",
					RawMd: corpus(t, "impl", "docz-impl-0007.md"),
				},
				{
					ID: 102, RepoID: repoPlatform, Type: "frameworks", DocID: "FW-0001", Title: "Intro",
					Status: validText("Draft"), Author: validText("Jane"),
					Path: "docs/frameworks/0001-intro.md", GitSha: "fwsha", ContentHash: "h3",
					RawMd: "---\nid: FW-0001\ntitle: Intro\n---\n\n# Intro\n\n## Why\n\nBecause.\n",
				},
			},
			repoHidden: {
				{
					ID: 200, RepoID: repoHidden, Type: "rfc", DocID: "RFC-0001", Title: "Secret",
					Path: "docs/rfc/0001-secret.md", GitSha: "s", ContentHash: "s", RawMd: "# Secret\n",
				},
			},
		},
		pages: map[int64][]store.RepoPage{
			repoPlatform: {{
				ID: 300, RepoID: repoPlatform, Path: "guides/setup", RepoPath: "docs/guides/setup.md",
				Title: "Setup", RawMd: "# Setup\n\nRun it.\n", GitSha: "pagesha", ContentHash: "p1",
			}},
		},
		urls: map[string]string{"RFC-0001": "https://example.atlassian.net/wiki/pages/1"},
	}
}
