package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/donaldgifford/docz/v2/internal/authorize"
)

func confluenceServer(on bool) http.Handler {
	st := seededStore()
	r := chi.NewRouter()
	NewHandler(st).WithConfluenceExport(on).Mount(r, authorize.Middleware(authorize.NewAllReposAuthorizer(st)))
	return r
}

func getSync(t *testing.T, h http.Handler, path string) confluenceSyncDTO {
	t.Helper()
	rec := doGet(t, h, path)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d %s", path, rec.Code, rec.Body)
	}
	var out confluenceSyncDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestGetRepoConfluence(t *testing.T) {
	h := confluenceServer(true)

	got := getSync(t, h, "/api/v1/repos/acme/platform/confluence")
	if got.Status != "partial" || got.Folder.ID != "426780" || got.Counts.Updated != 1 || got.Counts.Failed != 1 ||
		got.StartedAt != "2026-10-07T12:00:00Z" || got.FinishedAt != "2026-10-07T12:00:41Z" {
		t.Errorf("sync %+v", got)
	}
	if len(got.Pages) != 3 {
		t.Fatalf("pages %+v", got.Pages)
	}
	if p := got.Pages[0]; p.Edited == nil || p.Edited.Version != 4 || p.Edited.Expected != 3 || p.CommentsLost != 1 ||
		p.Source != "docs/frameworks/0001-intro.md" {
		t.Errorf("FW-0001 page %+v", p)
	}
	if got.Pages[1].Edited != nil {
		t.Errorf("an unedited page carries edited: %+v", got.Pages[1])
	}

	never := getSync(t, h, "/api/v1/repos/acme/bare/confluence")
	if never.Status != "never" || never.Pages == nil || len(never.Pages) != 0 || never.Repo != "acme/bare" {
		t.Errorf("never %+v", never)
	}

	if rec := doGet(t, h, "/api/v1/repos/acme/missing/confluence"); rec.Code != http.StatusNotFound {
		t.Errorf("missing repo = %d, want 404", rec.Code)
	}
}

func TestGetRepoConfluenceServerOff(t *testing.T) {
	got := getSync(t, confluenceServer(false), "/api/v1/repos/acme/platform/confluence")
	if got.Status != "disabled" || got.Reason != "disabled on this server" || len(got.Pages) != 0 {
		t.Errorf("server off %+v", got)
	}
}

func TestDocumentConfluenceURL(t *testing.T) {
	h := confluenceServer(true)
	const want = "https://example.atlassian.net/wiki/spaces/DOCZ/pages/65861"

	var one documentDTO
	rec := doGet(t, h, "/api/v1/repos/acme/platform/types/FW/docs/FW-0001")
	if err := json.Unmarshal(rec.Body.Bytes(), &one); err != nil || one.ConfluenceURL != want {
		t.Errorf("getDoc confluence_url = %q (%v), want %q", one.ConfluenceURL, err, want)
	}

	var list struct {
		Docs []documentDTO `json:"docs"`
	}
	rec = doGet(t, h, "/api/v1/repos/acme/platform/types/FW/docs")
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Docs) != 1 || list.Docs[0].ConfluenceURL != want {
		t.Errorf("listDocs %s (%v)", rec.Body, err)
	}
}
