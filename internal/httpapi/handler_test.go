package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/donaldgifford/docz-api/internal/authorize"
	"github.com/donaldgifford/docz-api/internal/store"
)

// fakeStore is an in-memory storeReader for the handler tests.
type fakeStore struct {
	repos []store.Repo
	types map[int64][]store.DocType
	docs  map[int64][]store.Document
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
		})
	}
	return out, nil
}

func (f *fakeStore) GetDocumentByID(_ context.Context, repoID int64, docID string) (store.Document, error) {
	for i := range f.docs[repoID] {
		if f.docs[repoID][i].DocID == docID {
			return f.docs[repoID][i], nil
		}
	}
	return store.Document{}, pgx.ErrNoRows
}

func validText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

// seededStore returns a fakeStore with a full repo (custom type, doc, cached
// index.md) plus a bare repo with no index, for the 404 flavors (OQ-2a).
func seededStore() *fakeStore {
	return &fakeStore{
		repos: []store.Repo{
			{
				ID: 1, Owner: "acme", Name: "platform", DefaultBranch: "main", DocsDir: "docs",
				ConfigSnapshot: json.RawMessage(`{"docs_dir":"docs"}`), LastSyncedSha: validText("headsha"),
				IndexMd: validText("# Platform\n"), IndexSha: validText("idxsha"),
			},
			{
				ID: 2, Owner: "acme", Name: "bare", DefaultBranch: "main", DocsDir: "docs",
				ConfigSnapshot: json.RawMessage(`{"docs_dir":"docs"}`), LastSyncedSha: validText("baresha"),
			},
		},
		types: map[int64][]store.DocType{
			1: {{
				ID: 10, RepoID: 1, Name: "frameworks", Dir: "frameworks", IDPrefix: "FW",
				PluralLabel: "Frameworks", Statuses: json.RawMessage(`["Draft","Adopted"]`),
				Aliases: json.RawMessage(`["framework"]`),
			}},
		},
		docs: map[int64][]store.Document{
			1: {{
				ID: 100, RepoID: 1, Type: "frameworks", DocID: "FW-0001", Title: "Intro",
				Status: validText("Draft"), Author: validText("Jane"),
				Created: pgtype.Date{Valid: false}, Path: "docs/frameworks/0001-intro.md",
				GitSha: "abc", ContentHash: "hash1", RawMd: "# Intro\n",
			}},
		},
	}
}

// testServer wires the handler exactly as main does: authorize middleware over
// the read routes.
func testServer(st storeReader, authz authorize.Authorizer) http.Handler {
	r := chi.NewRouter()
	NewHandler(st).Mount(r, authorize.Middleware(authz))
	return r
}

func doGet(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestReadEndpoints(t *testing.T) {
	st := seededStore()
	srv := testServer(st, authorize.NewAllReposAuthorizer(st))

	t.Run("list repos", func(t *testing.T) {
		rec := doGet(t, srv, "/api/v1/repos")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body struct {
			Repos []struct {
				Repo string `json:"repo"`
			} `json:"repos"`
		}
		mustDecode(t, rec, &body)
		if len(body.Repos) != 2 || body.Repos[0].Repo != "acme/platform" || body.Repos[1].Repo != "acme/bare" {
			t.Errorf("repos = %+v, want acme/platform + acme/bare", body.Repos)
		}
	})

	t.Run("repo detail", func(t *testing.T) {
		rec := doGet(t, srv, "/api/v1/repos/acme/platform")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body struct {
			Repo          string `json:"repo"`
			DefaultBranch string `json:"default_branch"`
			LastSyncedSHA string `json:"last_synced_sha"`
			Types         []struct {
				Name string `json:"name"`
			} `json:"types"`
		}
		mustDecode(t, rec, &body)
		if body.Repo != "acme/platform" || body.DefaultBranch != "main" || body.LastSyncedSHA != "headsha" {
			t.Errorf("detail = %+v", body)
		}
		if len(body.Types) != 1 || body.Types[0].Name != "frameworks" {
			t.Errorf("types = %+v, want frameworks", body.Types)
		}
	})

	t.Run("custom type addressable by name/prefix/alias", func(t *testing.T) {
		for _, seg := range []string{"frameworks", "FW", "framework"} {
			rec := doGet(t, srv, "/api/v1/repos/acme/platform/types/"+seg+"/docs")
			if rec.Code != http.StatusOK {
				t.Fatalf("GET .../types/%s/docs status = %d, want 200", seg, rec.Code)
			}
			var body struct {
				Docs []struct {
					DocID string `json:"doc_id"`
				} `json:"docs"`
			}
			mustDecode(t, rec, &body)
			if len(body.Docs) != 1 || body.Docs[0].DocID != "FW-0001" {
				t.Errorf("via %q: docs = %+v, want one FW-0001", seg, body.Docs)
			}
		}
	})

	t.Run("get one doc returns raw_md", func(t *testing.T) {
		rec := doGet(t, srv, "/api/v1/repos/acme/platform/types/FW/docs/FW-0001")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body documentDTO
		mustDecode(t, rec, &body)
		if body.DocID != "FW-0001" || body.Type != "frameworks" || body.RawMD != "# Intro\n" {
			t.Errorf("doc = %+v, want FW-0001/frameworks with raw_md", body)
		}
		if body.Created != "" {
			t.Errorf("Created = %q, want empty for NULL date", body.Created)
		}
	})

	t.Run("unknown type is 404", func(t *testing.T) {
		rec := doGet(t, srv, "/api/v1/repos/acme/platform/types/bogus/docs")
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("missing repo is 404", func(t *testing.T) {
		rec := doGet(t, srv, "/api/v1/repos/acme/missing")
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}

func TestGetRepoIndex(t *testing.T) {
	st := seededStore()
	// An empty-but-present index.md persists as a NULL body with a valid sha
	// (the textOrNull gotcha), which must serve as 200 with an empty string.
	st.repos = append(st.repos, store.Repo{
		ID: 3, Owner: "acme", Name: "emptyidx", DefaultBranch: "main", DocsDir: "docs",
		ConfigSnapshot: json.RawMessage(`{"docs_dir":"docs"}`), IndexSha: validText("emptysha"),
	})
	srv := testServer(st, authorize.NewAllReposAuthorizer(st))

	t.Run("present index serves body and sha", func(t *testing.T) {
		rec := doGet(t, srv, "/api/v1/repos/acme/platform/index")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body repoIndexDTO
		mustDecode(t, rec, &body)
		if body.Repo != "acme/platform" || body.IndexMD != "# Platform\n" || body.IndexSHA != "idxsha" {
			t.Errorf("index = %+v, want the cached body and sha", body)
		}
	})

	t.Run("empty index file is 200 with empty body", func(t *testing.T) {
		rec := doGet(t, srv, "/api/v1/repos/acme/emptyidx/index")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body repoIndexDTO
		mustDecode(t, rec, &body)
		if body.IndexMD != "" || body.IndexSHA != "emptysha" {
			t.Errorf("index = %+v, want empty body with a valid sha", body)
		}
	})

	t.Run("repo without an index is 404", func(t *testing.T) {
		rec := doGet(t, srv, "/api/v1/repos/acme/bare/index")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
		var body struct {
			Error string `json:"error"`
		}
		mustDecode(t, rec, &body)
		if body.Error != "index not found" {
			t.Errorf("error = %q, want 'index not found'", body.Error)
		}
	})

	t.Run("unknown repo is 404", func(t *testing.T) {
		rec := doGet(t, srv, "/api/v1/repos/acme/missing/index")
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}

func TestGetRepoIndexUnauthorizedIs404(t *testing.T) {
	srv := testServer(seededStore(), fixedAuthorizer{allowed: authorize.AllowedRepos{999}})

	rec := doGet(t, srv, "/api/v1/repos/acme/platform/index")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (existence hidden)", rec.Code)
	}
}

func TestUnauthorizedRepoHiddenAs404(t *testing.T) {
	st := seededStore()
	// Authorizer that allows a different repo id, so repo 1 is hidden.
	srv := testServer(st, fixedAuthorizer{allowed: authorize.AllowedRepos{999}})

	rec := doGet(t, srv, "/api/v1/repos/acme/platform")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (existence hidden)", rec.Code)
	}

	// And it is absent from the list.
	rec = doGet(t, srv, "/api/v1/repos")
	var body struct {
		Repos []json.RawMessage `json:"repos"`
	}
	mustDecode(t, rec, &body)
	if len(body.Repos) != 0 {
		t.Errorf("repos = %d, want 0 for an unauthorized caller", len(body.Repos))
	}
}

// fixedAuthorizer returns a fixed allowed set.
type fixedAuthorizer struct{ allowed authorize.AllowedRepos }

func (f fixedAuthorizer) Allowed(context.Context, *http.Request) (authorize.AllowedRepos, error) {
	return f.allowed, nil
}

func mustDecode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
}
