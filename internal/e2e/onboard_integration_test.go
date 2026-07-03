//go:build integration

// Package e2e holds the Phase 2 end-to-end proof: a fixture repo hand-onboarded
// through the real ingest pipeline into a real Postgres, then served by the real
// httpapi read endpoints. Only GitHub is faked (an in-memory RepoFetcher), so
// the test is hermetic while exercising fetch → parse → map → reconcile → serve.
package e2e

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/donaldgifford/docz-api/internal/authorize"
	"github.com/donaldgifford/docz-api/internal/httpapi"
	"github.com/donaldgifford/docz-api/internal/ingest"
	"github.com/donaldgifford/docz-api/internal/store"
)

var (
	testStore *store.Store
	testMux   http.Handler
)

func TestMain(m *testing.M) {
	os.Exit(runMain(m))
}

func runMain(m *testing.M) int {
	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("docz_api"),
		postgres.WithUsername("docz"),
		postgres.WithPassword("secret"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		log.Printf("start postgres: %v", err)
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
	if err := store.Migrate(ctx, dsn); err != nil {
		log.Printf("migrate: %v", err)
		return 1
	}
	pool, err := store.NewPool(ctx, dsn)
	if err != nil {
		log.Printf("pool: %v", err)
		return 1
	}
	defer pool.Close()

	testStore = store.NewStore(pool)

	r := chi.NewRouter()
	httpapi.NewHandler(testStore).Mount(r, authorize.Middleware(authorize.NewAllReposAuthorizer(testStore)))
	testMux = r

	return m.Run()
}

// staticFetcher is an in-memory ingest.RepoFetcher returning a fixed snapshot.
type staticFetcher struct{ snap *ingest.RepoSnapshot }

func (f staticFetcher) Fetch(context.Context, string, string) (*ingest.RepoSnapshot, error) {
	return f.snap, nil
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
      - framework
`

func doc(id, title, body string) []byte {
	return []byte("---\nid: " + id + "\ntitle: " + title + "\nstatus: Draft\nauthor: Jane\ncreated: 2026-07-01\n---\n\n" + body + "\n")
}

// fixtureOwner is the owner all fixture repos share.
const fixtureOwner = "acme"

// onboard seeds an installation and runs one ingest of snap for fixtureOwner/name.
func onboard(t *testing.T, name string, instID int64, snap *ingest.RepoSnapshot) store.ReconcileResult {
	t.Helper()
	if err := testStore.UpsertInstallation(t.Context(), store.InstallationInput{
		ID: instID, AccountLogin: fixtureOwner, AccountType: "Organization",
	}); err != nil {
		t.Fatalf("seed installation: %v", err)
	}
	res, err := ingest.NewService(testStore, staticFetcher{snap: snap}).Run(t.Context(), instID, fixtureOwner, name)
	if err != nil {
		t.Fatalf("ingest %s/%s: %v", fixtureOwner, name, err)
	}
	return res
}

// getJSON GETs path against the real mux and decodes the body into dst.
func getJSON(t *testing.T, path string, dst any) int {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody)
	rec := httptest.NewRecorder()
	testMux.ServeHTTP(rec, req)
	if dst != nil && rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), dst); err != nil {
			t.Fatalf("decode %s: %v (body %q)", path, err, rec.Body.String())
		}
	}
	return rec.Code
}

func TestE2EOnboardAndServe(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // hermetic doczcfg.Load

	snap := &ingest.RepoSnapshot{
		HeadSHA:       "head-1",
		DefaultBranch: "main",
		ConfigYAML:    []byte(fixtureConfig),
		ChangelogMD:   []byte("# Changelog\n"),
		ChangelogSHA:  "cl-1",
		Blobs: []ingest.BlobEntry{
			{Path: "docs/frameworks/0001-intro.md", GitSHA: "g1", Content: doc("FW-0001", "Intro", "# Intro")},
			{Path: "docs/frameworks/0002-next.md", GitSHA: "g2", Content: doc("FW-0002", "Next", "# Next")},
		},
	}
	res := onboard(t, "serve", 900, snap)
	if res.DocsUpserted != 2 || res.TypesUpserted != 1 {
		t.Fatalf("onboard result = %+v, want 2 docs / 1 type", res)
	}

	t.Run("repo list and detail", func(t *testing.T) {
		var list struct {
			Repos []struct{ Repo string } `json:"repos"`
		}
		if code := getJSON(t, "/api/v1/repos", &list); code != http.StatusOK {
			t.Fatalf("list repos status = %d", code)
		}
		if !containsRepo(list.Repos, "acme/serve") {
			t.Errorf("repo list %+v missing acme/serve", list.Repos)
		}

		var detail struct {
			Repo          string                  `json:"repo"`
			LastSyncedSHA string                  `json:"last_synced_sha"`
			Config        json.RawMessage         `json:"config_snapshot"`
			Types         []struct{ Name string } `json:"types"`
		}
		if code := getJSON(t, "/api/v1/repos/acme/serve", &detail); code != http.StatusOK {
			t.Fatalf("repo detail status = %d", code)
		}
		if detail.LastSyncedSHA != "head-1" || len(detail.Types) != 1 || len(detail.Config) == 0 {
			t.Errorf("detail = %+v", detail)
		}
	})

	t.Run("custom type addressable by name, prefix, and alias", func(t *testing.T) {
		for _, seg := range []string{"frameworks", "FW", "framework", "fw"} {
			var body struct {
				Docs []struct {
					DocID string `json:"doc_id"`
				} `json:"docs"`
			}
			code := getJSON(t, "/api/v1/repos/acme/serve/types/"+seg+"/docs", &body)
			if code != http.StatusOK {
				t.Fatalf("via %q: status = %d", seg, code)
			}
			if len(body.Docs) != 2 {
				t.Errorf("via %q: %d docs, want 2", seg, len(body.Docs))
			}
		}
	})

	t.Run("single doc returns raw markdown", func(t *testing.T) {
		var body struct {
			DocID   string `json:"doc_id"`
			Type    string `json:"type"`
			Created string `json:"created"`
			RawMD   string `json:"raw_md"`
		}
		code := getJSON(t, "/api/v1/repos/acme/serve/types/FW/docs/FW-0001", &body)
		if code != http.StatusOK {
			t.Fatalf("get doc status = %d", code)
		}
		if body.DocID != "FW-0001" || body.Type != "frameworks" || body.Created != "2026-07-01" {
			t.Errorf("doc = %+v", body)
		}
		if body.RawMD == "" {
			t.Error("raw_md is empty, want the document markdown")
		}
	})

	t.Run("unknown doc is 404", func(t *testing.T) {
		if code := getJSON(t, "/api/v1/repos/acme/serve/types/FW/docs/FW-9999", nil); code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", code)
		}
	})
}

func TestE2EContentHashGateAndDelete(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	d1 := ingest.BlobEntry{Path: "docs/frameworks/0001-intro.md", GitSHA: "g1", Content: doc("FW-0001", "Intro", "# Intro")}
	d2 := ingest.BlobEntry{Path: "docs/frameworks/0002-next.md", GitSHA: "g2", Content: doc("FW-0002", "Next", "# Next")}

	base := &ingest.RepoSnapshot{
		HeadSHA: "h1", DefaultBranch: "main", ConfigYAML: []byte(fixtureConfig),
		Blobs: []ingest.BlobEntry{d1, d2},
	}
	if res := onboard(t, "gate", 901, base); res.DocsUpserted != 2 {
		t.Fatalf("first onboard = %+v, want 2 upserted", res)
	}

	// Identical HEAD: the content-hash gate makes it a Postgres no-op.
	if res := onboard(t, "gate", 901, base); res.DocsUnchanged != 2 || res.DocsUpserted != 0 {
		t.Errorf("unchanged re-onboard = %+v, want 2 unchanged / 0 upserted", res)
	}

	// One doc changes, the other is removed from HEAD.
	changed := ingest.BlobEntry{Path: "docs/frameworks/0001-intro.md", GitSHA: "g1b", Content: doc("FW-0001", "Intro v2", "# Intro v2")}
	next := &ingest.RepoSnapshot{
		HeadSHA: "h2", DefaultBranch: "main", ConfigYAML: []byte(fixtureConfig),
		Blobs: []ingest.BlobEntry{changed},
	}
	res := onboard(t, "gate", 901, next)
	if res.DocsUpserted != 1 || res.DocsDeleted != 1 {
		t.Errorf("changed+deleted re-onboard = %+v, want 1 upserted / 1 deleted", res)
	}

	// The API now serves exactly one doc, the updated FW-0001.
	var body struct {
		Docs []struct {
			DocID string `json:"doc_id"`
		} `json:"docs"`
	}
	if code := getJSON(t, "/api/v1/repos/acme/gate/types/frameworks/docs", &body); code != http.StatusOK {
		t.Fatalf("list docs status = %d", code)
	}
	if len(body.Docs) != 1 || body.Docs[0].DocID != "FW-0001" {
		t.Errorf("docs after delete = %+v, want only FW-0001", body.Docs)
	}
}

func containsRepo(repos []struct{ Repo string }, want string) bool {
	for _, r := range repos {
		if r.Repo == want {
			return true
		}
	}
	return false
}
