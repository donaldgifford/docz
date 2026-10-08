package export

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/donaldgifford/docz/v2/internal/store"
	doczcfg "github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/export/confluence"
	"github.com/donaldgifford/docz/v2/pkg/export/confluence/confluencetest"
)

const (
	rfcTitle = "docz: RFC-0001: First"
	adrTitle = "docz: ADR-0001: Second"
)

// fixtureConfig is a repository config with the block enabled in the
// folder layout on the test site.
func fixtureConfig() doczcfg.Config {
	cfg := doczcfg.DefaultConfig()
	cfg.Sync.Confluence = doczcfg.ConfluenceSyncConfig{
		Enabled: true, Site: confluencetest.DefaultURL, Space: "DOCZ", Folder: "docz",
		Layout:  doczcfg.LayoutFolder,
		Mermaid: doczcfg.MermaidSyncConfig{Viewer: doczcfg.MermaidViewerAuto},
	}

	return cfg
}

// fixtureInputs is a two-document repository's snapshot.
func fixtureInputs(t *testing.T, cfg *doczcfg.Config) store.ExportInputs {
	t.Helper()

	snap, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	return store.ExportInputs{
		Repo: store.Repo{
			ID: 7, Owner: "o", Name: "docz", DefaultBranch: "main", DocsDir: "docs",
			ConfigSnapshot: snap, LastSyncedSha: pgtype.Text{String: "sha-1", Valid: true},
		},
		Documents: []store.Document{
			{Type: "adr", DocID: "ADR-0001", Path: "docs/adr/0001-second.md", RawMd: adrMD},
			{Type: "rfc", DocID: "RFC-0001", Path: "docs/rfc/0001-first.md", RawMd: rfcMD},
		},
		PageIDs: map[string]string{},
	}
}

const (
	rfcMD = "---\nid: RFC-0001\ntitle: First\nstatus: Draft\ncreated: 2026-03-04\nauthor: A\n---\n\n" +
		"# RFC-0001: First\n\nSee [ADR-0001](../adr/0001-second.md) and [the Makefile](../../justfile).\n"
	adrMD = "---\nid: ADR-0001\ntitle: Second\nstatus: Proposed\ncreated: 2026-03-04\nauthor: B\n---\n\n" +
		"# ADR-0001: Second\n\nText.\n"
)

// fakeStore serves one snapshot and records what is written. advance moves
// the head on every load, as an ingest committing during each export does.
type fakeStore struct {
	mu      sync.Mutex
	in      store.ExportInputs
	err     error
	advance bool
	loads   int
	states  []string
	syncs   []store.UpsertConfluenceSyncParams
	pages   []store.UpsertConfluencePageParams
}

func (f *fakeStore) ExportInputs(_ context.Context, _ int64) (store.ExportInputs, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.loads++
	if f.advance && f.loads > 1 {
		f.in.Repo.LastSyncedSha.String += "+"
	}

	return f.in, f.err
}

func (f *fakeStore) UpsertConfluenceSync(_ context.Context, p *store.UpsertConfluenceSyncParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.states = append(f.states, p.Status)
	f.syncs = append(f.syncs, *p)

	return nil
}

func (f *fakeStore) RecordConfluenceExport(
	_ context.Context, rec *store.UpsertConfluenceSyncParams, pages []store.UpsertConfluencePageParams,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.states = append(f.states, rec.Status)
	f.syncs = append(f.syncs, *rec)
	f.pages = pages

	return nil
}

// allowList is an allow-list for the test site's DOCZ space.
type allowList struct{}

func (allowList) Allowed(site, space string) bool {
	return site == confluencetest.DefaultURL && space == "DOCZ"
}

func newService(st Store, site *confluencetest.Site) *Service {
	return NewService(st, site, allowList{}, "test")
}

func TestRun_Succeeded(t *testing.T) {
	t.Parallel()

	cfg := fixtureConfig()
	st := &fakeStore{in: fixtureInputs(t, &cfg)}
	site := confluencetest.New("DOCZ")

	res, err := newService(st, site).Run(t.Context(), 7)
	if err != nil {
		t.Fatal(err)
	}

	if res.Status != StatusSucceeded || res.Folder != "docz" || res.Runs != 1 || res.HeadSHA != "sha-1" {
		t.Errorf("result %+v", res)
	}

	if !slices.Equal(st.states, []string{"running", "succeeded"}) {
		t.Errorf("states %v", st.states)
	}

	last := st.syncs[len(st.syncs)-1]
	if last.FolderTitle != "docz" || last.FolderID == "" || last.Site != confluencetest.DefaultURL || !last.FinishedAt.Valid {
		t.Errorf("sync %+v", last)
	}

	for _, title := range []string{rfcTitle, adrTitle, "docz: RFCs", "docz: ADRs"} {
		if site.PageByTitle(title) == nil {
			t.Errorf("no page %q", title)
		}
	}

	keys := make([]string, 0, len(st.pages))
	for _, p := range st.pages {
		if p.PageID == "" || p.Url == "" || p.Action != "created" {
			t.Errorf("page row %+v", p)
		}

		keys = append(keys, p.Key)
	}

	for _, k := range []string{"RFC-0001", "ADR-0001", "docz:parent", "docz:type:rfc"} {
		if !slices.Contains(keys, k) {
			t.Errorf("no row for %s in %v", k, keys)
		}
	}
}

func TestRun_RecordedIDsAndUnchanged(t *testing.T) {
	t.Parallel()

	cfg := fixtureConfig()
	st := &fakeStore{in: fixtureInputs(t, &cfg)}
	site := confluencetest.New("DOCZ")
	svc := newService(st, site)

	if _, err := svc.Run(t.Context(), 7); err != nil {
		t.Fatal(err)
	}

	for _, p := range st.pages {
		st.in.PageIDs[p.Key] = p.PageID
	}

	last := st.syncs[len(st.syncs)-1]
	st.in.Sync = &store.ConfluenceSync{FolderID: last.FolderID, FolderTitle: last.FolderTitle}
	site.Writes()

	res, err := svc.Run(t.Context(), 7)
	if err != nil {
		t.Fatal(err)
	}

	if w := site.Writes(); len(w) != 0 {
		t.Errorf("second run wrote %v", w)
	}

	if res.Counts.Unchanged == 0 || res.Counts.Created != 0 {
		t.Errorf("counts %+v", res.Counts)
	}
}

func TestRun_DisabledAndRefused(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*doczcfg.ConfluenceSyncConfig)
		want   Status
	}{
		{"disabled", func(c *doczcfg.ConfluenceSyncConfig) { c.Enabled = false }, StatusDisabled},
		{"space", func(c *doczcfg.ConfluenceSyncConfig) { c.Space = "OTHER" }, StatusRefused},
		{"site", func(c *doczcfg.ConfluenceSyncConfig) { c.Site = "https://other.atlassian.net" }, StatusRefused},
		{"page layout", func(c *doczcfg.ConfluenceSyncConfig) { c.Layout = doczcfg.LayoutPage }, StatusRefused},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := fixtureConfig()
			tt.mutate(&cfg.Sync.Confluence)

			st := &fakeStore{in: fixtureInputs(t, &cfg)}
			site := confluencetest.New("DOCZ", "OTHER")

			res, err := newService(st, site).Run(t.Context(), 7)
			if err != nil {
				t.Fatal(err)
			}

			if res.Status != tt.want || !slices.Equal(st.states, []string{string(tt.want)}) {
				t.Errorf("status %s, states %v; want %s", res.Status, st.states, tt.want)
			}

			if tt.want == StatusRefused && res.Reason == "" {
				t.Error("refused with no reason")
			}

			if w := site.Writes(); len(w) != 0 {
				t.Errorf("wrote %v", w)
			}
		})
	}
}

func TestRun_FailedPageIsRetried(t *testing.T) {
	t.Parallel()

	cfg := fixtureConfig()
	st := &fakeStore{in: fixtureInputs(t, &cfg)}
	site := confluencetest.New("DOCZ")
	site.FailOn("create", rfcTitle, 503)

	res, err := newService(st, site).Run(t.Context(), 7)
	if err == nil || errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("err %v; want a retryable error", err)
	}

	if res.Status != StatusPartial || res.Counts.Failed != 1 || res.Reason == "" {
		t.Errorf("result %+v", res)
	}

	if got := st.states[len(st.states)-1]; got != "partial" {
		t.Errorf("recorded %s, want partial", got)
	}
}

func TestRun_AuthIsNotRetried(t *testing.T) {
	t.Parallel()

	cfg := fixtureConfig()
	st := &fakeStore{in: fixtureInputs(t, &cfg)}
	site := confluencetest.New("DOCZ")
	site.RequireAuth("me@example.com", "right")

	svc := NewService(st, site.HTTPClient(t, "me@example.com", "wrong"), allowList{}, "test")

	res, err := svc.Run(t.Context(), 7)
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("err %v; want SkipRetry", err)
	}

	var ae *confluence.AuthError
	if !errors.As(err, &ae) || res.Status != StatusFailed {
		t.Errorf("err %v, status %s; want a failed AuthError", err, res.Status)
	}
}

func TestRun_GoneRepoIsNotRetried(t *testing.T) {
	t.Parallel()

	st := &fakeStore{err: pgx.ErrNoRows}

	_, err := newService(st, confluencetest.New("DOCZ")).Run(t.Context(), 7)
	if !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("err %v; want SkipRetry", err)
	}
}

// TestRun_HeadLoopStopsAtThree: an ingest committing during every export
// makes Run export again, three times in all and no more.
func TestRun_HeadLoopStopsAtThree(t *testing.T) {
	t.Parallel()

	cfg := fixtureConfig()
	st := &fakeStore{in: fixtureInputs(t, &cfg), advance: true}

	res, err := newService(st, confluencetest.New("DOCZ")).Run(t.Context(), 7)
	if err != nil {
		t.Fatal(err)
	}

	if res.Runs != maxRuns || st.loads != maxRuns {
		t.Errorf("runs %d, loads %d; want %d", res.Runs, st.loads, maxRuns)
	}
}

// TestRun_HeadUnmovedRunsOnce: with no ingest in between, the second load
// sees the head it exported and stops.
func TestRun_HeadUnmovedRunsOnce(t *testing.T) {
	t.Parallel()

	cfg := fixtureConfig()
	st := &fakeStore{in: fixtureInputs(t, &cfg)}

	res, err := newService(st, confluencetest.New("DOCZ")).Run(t.Context(), 7)
	if err != nil {
		t.Fatal(err)
	}

	if res.Runs != 1 || st.loads != 2 {
		t.Errorf("runs %d, loads %d; want 1 run over 2 loads", res.Runs, st.loads)
	}
}
