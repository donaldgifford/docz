package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/v88/github"
	"github.com/jackc/pgx/v5"

	"github.com/donaldgifford/docz-api/internal/queue"
	"github.com/donaldgifford/docz-api/internal/store"
)

// testSecret is the shared HMAC secret used across the handler tests.
var testSecret = []byte("webhook-shared-secret")

// --- fakes ---------------------------------------------------------------

// fakeStore records calls so a test can assert what the handler persisted. Its
// repos map backs both GetRepo and DeleteRepo; a missing key surfaces as
// pgx.ErrNoRows, mirroring the real store.
type fakeStore struct {
	deliveries    map[string]bool
	recordErr     error
	upserts       []store.InstallationInput
	deletedInsts  []int64
	repoIDsByInst map[int64][]int64
	repos         map[string]store.Repo
	deletedRepos  []string
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		deliveries:    map[string]bool{},
		repoIDsByInst: map[int64][]int64{},
		repos:         map[string]store.Repo{},
	}
}

func (f *fakeStore) RecordDelivery(_ context.Context, deliveryID, _ string) (bool, error) {
	if f.recordErr != nil {
		return false, f.recordErr
	}
	if f.deliveries[deliveryID] {
		return false, nil
	}
	f.deliveries[deliveryID] = true
	return true, nil
}

func (f *fakeStore) UpsertInstallation(_ context.Context, in store.InstallationInput) error {
	f.upserts = append(f.upserts, in)
	return nil
}

func (f *fakeStore) DeleteInstallation(_ context.Context, id int64) error {
	f.deletedInsts = append(f.deletedInsts, id)
	return nil
}

func (f *fakeStore) ListRepoIDsByInstallation(_ context.Context, installationID int64) ([]int64, error) {
	return f.repoIDsByInst[installationID], nil
}

func (f *fakeStore) GetRepo(_ context.Context, owner, name string) (store.Repo, error) {
	if repo, ok := f.repos[owner+"/"+name]; ok {
		return repo, nil
	}
	return store.Repo{}, pgx.ErrNoRows
}

func (f *fakeStore) DeleteRepo(_ context.Context, owner, name string) (int64, error) {
	key := owner + "/" + name
	repo, ok := f.repos[key]
	if !ok {
		return 0, pgx.ErrNoRows
	}
	f.deletedRepos = append(f.deletedRepos, key)
	delete(f.repos, key)
	return repo.ID, nil
}

// fakeEnqueuer collects enqueued jobs.
type fakeEnqueuer struct {
	jobs []*queue.IngestJob
	err  error
}

func (f *fakeEnqueuer) EnqueueIngest(_ context.Context, job *queue.IngestJob) error {
	if f.err != nil {
		return f.err
	}
	f.jobs = append(f.jobs, job)
	return nil
}

// fakePurger records the repo ids purged from the search index.
type fakePurger struct{ repoIDs []int64 }

func (f *fakePurger) DeleteRepoDocuments(_ context.Context, repoID int64) error {
	f.repoIDs = append(f.repoIDs, repoID)
	return nil
}

// --- helpers -------------------------------------------------------------

// signBody returns the "sha256=<hex>" header value GitHub would send for body.
func signBody(t *testing.T, secret, body []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, secret)
	if _, err := mac.Write(body); err != nil {
		t.Fatalf("hmac write: %v", err)
	}
	return signaturePrefix + hex.EncodeToString(mac.Sum(nil))
}

// mustJSON marshals v or fails the test.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// newRequest builds a webhook POST with the given headers set (a header is
// omitted when its value is empty).
func newRequest(event, delivery string, body []byte, sig string) *http.Request {
	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodPost, "/webhooks/github", bytes.NewReader(body),
	)
	req.Header.Set("X-GitHub-Event", event)
	if delivery != "" {
		req.Header.Set("X-GitHub-Delivery", delivery)
	}
	if sig != "" {
		req.Header.Set("X-Hub-Signature-256", sig)
	}
	return req
}

// serve signs body, builds the request, and runs it through a fresh handler,
// returning the recorder and the wired fakes for assertions.
func serve(
	t *testing.T, st *fakeStore, event, delivery string, body []byte,
) (*httptest.ResponseRecorder, *fakeEnqueuer, *fakePurger) {
	t.Helper()
	enq := &fakeEnqueuer{}
	purger := &fakePurger{}
	h := New(testSecret, st, enq, purger)
	req := newRequest(event, delivery, body, signBody(t, testSecret, body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr, enq, purger
}

func pushBody(t *testing.T, instID int64, name, defaultBranch, ref string, changed ...string) []byte {
	t.Helper()
	const owner = "acme"
	return mustJSON(t, &github.PushEvent{
		Ref: github.Ptr(ref),
		Repo: &github.PushEventRepository{
			Name:          github.Ptr(name),
			FullName:      github.Ptr(owner + "/" + name),
			Owner:         &github.User{Login: github.Ptr(owner)},
			DefaultBranch: github.Ptr(defaultBranch),
		},
		Installation: &github.Installation{ID: github.Ptr(instID)},
		Commits:      []*github.HeadCommit{{Modified: changed}},
	})
}

func installationBody(t *testing.T, action string, instID int64, account string, repoFullNames ...string) []byte {
	t.Helper()
	repos := make([]*github.Repository, 0, len(repoFullNames))
	for _, fn := range repoFullNames {
		repos = append(repos, &github.Repository{FullName: github.Ptr(fn)})
	}
	return mustJSON(t, &github.InstallationEvent{
		Action: github.Ptr(action),
		Installation: &github.Installation{
			ID:      github.Ptr(instID),
			Account: &github.User{Login: github.Ptr(account), Type: github.Ptr("Organization")},
		},
		Repositories: repos,
	})
}

// --- HMAC verification ---------------------------------------------------

func TestVerifyHMAC(t *testing.T) {
	t.Parallel()
	secret := []byte("s3cr3t")
	body := []byte(`{"zen":"keep it simple"}`)
	valid := signBody(t, secret, body)

	// nearMiss flips the final hex nibble of a valid signature: same length,
	// same prefix, wrong tail — hmac.Equal must still reject it.
	nearMiss := valid[:len(valid)-1] + "0"
	if nearMiss == valid {
		nearMiss = valid[:len(valid)-1] + "1"
	}

	tests := []struct {
		name   string
		sig    string
		secret []byte
		body   []byte
		want   bool
	}{
		{"valid", valid, secret, body, true},
		{"wrong secret", signBody(t, []byte("other"), body), secret, body, false},
		{"tampered body", valid, secret, []byte(`{"zen":"tampered"}`), false},
		{"missing header", "", secret, body, false},
		{"missing prefix", hex.EncodeToString([]byte("nope")), secret, body, false},
		{"non-hex digest", signaturePrefix + "zzzz", secret, body, false},
		{"near miss", nearMiss, secret, body, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := verifyHMAC(tc.secret, tc.body, tc.sig); got != tc.want {
				t.Errorf("verifyHMAC() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestServeHTTPRejectsBadSignature(t *testing.T) {
	body := pushBody(t, 1, "widgets", "main", "refs/heads/main", "docs/rfc/RFC-0001.md")

	tests := []struct {
		name string
		sig  string
	}{
		{"wrong secret", signBody(t, []byte("attacker"), body)},
		{"tampered body", signBody(t, testSecret, []byte(`{"ref":"tampered"}`))},
		{"missing header", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := newFakeStore()
			enq := &fakeEnqueuer{}
			h := New(testSecret, st, enq, &fakePurger{})
			req := newRequest("push", "d-1", body, tc.sig)
			rr := httptest.NewRecorder()

			h.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rr.Code)
			}
			if len(st.deliveries) != 0 {
				t.Errorf("recorded %d deliveries, want 0 (no work on bad signature)", len(st.deliveries))
			}
			if len(enq.jobs) != 0 {
				t.Errorf("enqueued %d jobs, want 0 (no work on bad signature)", len(enq.jobs))
			}
		})
	}
}

// --- idempotency ---------------------------------------------------------

func TestServeHTTPReplayedDeliveryIsNoOp(t *testing.T) {
	st := newFakeStore()
	st.repos["acme/widgets"] = store.Repo{ID: 1, DocsDir: "docs"}
	body := pushBody(t, 1, "widgets", "main", "refs/heads/main", "docs/rfc/RFC-0001.md")

	enq := &fakeEnqueuer{}
	h := New(testSecret, st, enq, &fakePurger{})
	sig := signBody(t, testSecret, body)

	first := httptest.NewRecorder()
	h.ServeHTTP(first, newRequest("push", "delivery-42", body, sig))
	if first.Code != http.StatusAccepted {
		t.Fatalf("first delivery status = %d, want 202", first.Code)
	}

	second := httptest.NewRecorder()
	h.ServeHTTP(second, newRequest("push", "delivery-42", body, sig))
	if second.Code != http.StatusOK {
		t.Errorf("replayed delivery status = %d, want 200", second.Code)
	}
	if len(enq.jobs) != 1 {
		t.Errorf("enqueued %d jobs across a delivery + its replay, want 1", len(enq.jobs))
	}
}

// --- push routing --------------------------------------------------------

func TestServeHTTPPushEnqueuesOnRelevantChange(t *testing.T) {
	st := newFakeStore()
	st.repos["acme/widgets"] = store.Repo{ID: 7, DocsDir: "docs"}
	body := pushBody(t, 99, "widgets", "main", "refs/heads/main", "docs/rfc/RFC-0002.md")

	rr, enq, _ := serve(t, st, "push", "d-push", body)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rr.Code)
	}
	if len(enq.jobs) != 1 {
		t.Fatalf("enqueued %d jobs, want 1", len(enq.jobs))
	}
	job := enq.jobs[0]
	if job.Owner != "acme" || job.Name != "widgets" || job.InstallationID != 99 || job.Reason != "push" {
		t.Errorf("job = %+v, want acme/widgets inst 99 reason push", job)
	}
}

func TestServeHTTPPushSkips(t *testing.T) {
	tests := []struct {
		name           string
		repoConfigured bool
		ref            string
		changed        string
	}{
		{"non-default branch", true, "refs/heads/feature", "docs/rfc/RFC-1.md"},
		{"irrelevant path", true, "refs/heads/main", "README.md"},
		{"unknown repo", false, "refs/heads/main", "docs/rfc/RFC-1.md"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := newFakeStore()
			if tc.repoConfigured {
				st.repos["acme/widgets"] = store.Repo{ID: 1, DocsDir: "docs"}
			}
			body := pushBody(t, 1, "widgets", "main", tc.ref, tc.changed)

			rr, enq, _ := serve(t, st, "push", "d-"+tc.name, body)

			if rr.Code != http.StatusAccepted {
				t.Errorf("status = %d, want 202", rr.Code)
			}
			if len(enq.jobs) != 0 {
				t.Errorf("enqueued %d jobs, want 0", len(enq.jobs))
			}
		})
	}
}

// --- onboarding / offboarding -------------------------------------------

func TestServeHTTPInstallationCreatedOnboards(t *testing.T) {
	st := newFakeStore()
	body := installationBody(t, "created", 55, "acme", "acme/widgets", "acme/gadgets")

	rr, enq, _ := serve(t, st, "installation", "d-install", body)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rr.Code)
	}
	if len(st.upserts) != 1 || st.upserts[0].ID != 55 || st.upserts[0].AccountLogin != "acme" {
		t.Errorf("upserts = %+v, want one for installation 55/acme", st.upserts)
	}
	if len(enq.jobs) != 2 {
		t.Fatalf("enqueued %d jobs, want 2 (one per granted repo)", len(enq.jobs))
	}
	for _, job := range enq.jobs {
		if job.Reason != "onboard" || job.InstallationID != 55 {
			t.Errorf("job = %+v, want reason onboard inst 55", job)
		}
	}
}

func TestServeHTTPInstallationDeletedOffboards(t *testing.T) {
	st := newFakeStore()
	st.repoIDsByInst[55] = []int64{3, 4}
	body := installationBody(t, "deleted", 55, "acme")

	rr, _, purger := serve(t, st, "installation", "d-uninstall", body)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rr.Code)
	}
	if len(st.deletedInsts) != 1 || st.deletedInsts[0] != 55 {
		t.Errorf("deletedInsts = %v, want [55]", st.deletedInsts)
	}
	if len(purger.repoIDs) != 2 || purger.repoIDs[0] != 3 || purger.repoIDs[1] != 4 {
		t.Errorf("purged repo ids = %v, want [3 4]", purger.repoIDs)
	}
}

func TestServeHTTPInstallationReposAdded(t *testing.T) {
	st := newFakeStore()
	body := mustJSON(t, &github.InstallationRepositoriesEvent{
		Action: github.Ptr("added"),
		Installation: &github.Installation{
			ID:      github.Ptr(int64(12)),
			Account: &github.User{Login: github.Ptr("acme"), Type: github.Ptr("Organization")},
		},
		RepositoriesAdded: []*github.Repository{{FullName: github.Ptr("acme/newrepo")}},
	})

	rr, enq, _ := serve(t, st, "installation_repositories", "d-added", body)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rr.Code)
	}
	if len(st.upserts) != 1 {
		t.Errorf("upserts = %d, want 1", len(st.upserts))
	}
	if len(enq.jobs) != 1 || enq.jobs[0].Name != "newrepo" || enq.jobs[0].Reason != "repo_added" {
		t.Errorf("jobs = %+v, want one repo_added for newrepo", enq.jobs)
	}
}

func TestServeHTTPInstallationReposRemoved(t *testing.T) {
	st := newFakeStore()
	st.repos["acme/oldrepo"] = store.Repo{ID: 9, DocsDir: "docs"}
	body := mustJSON(t, &github.InstallationRepositoriesEvent{
		Action:              github.Ptr("removed"),
		Installation:        &github.Installation{ID: github.Ptr(int64(12))},
		RepositoriesRemoved: []*github.Repository{{FullName: github.Ptr("acme/oldrepo")}},
	})

	rr, _, purger := serve(t, st, "installation_repositories", "d-removed", body)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rr.Code)
	}
	if len(st.deletedRepos) != 1 || st.deletedRepos[0] != "acme/oldrepo" {
		t.Errorf("deletedRepos = %v, want [acme/oldrepo]", st.deletedRepos)
	}
	if len(purger.repoIDs) != 1 || purger.repoIDs[0] != 9 {
		t.Errorf("purged repo ids = %v, want [9]", purger.repoIDs)
	}
}

func TestServeHTTPUnhandledEventAccepted(t *testing.T) {
	st := newFakeStore()
	body := mustJSON(t, &github.PingEvent{Zen: github.Ptr("Non-blocking is better than blocking.")})

	rr, enq, _ := serve(t, st, "ping", "d-ping", body)

	if rr.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", rr.Code)
	}
	if len(enq.jobs) != 0 {
		t.Errorf("enqueued %d jobs for ping, want 0", len(enq.jobs))
	}
}

// --- pure helpers --------------------------------------------------------

func TestShouldIngest(t *testing.T) {
	t.Parallel()
	push := func(ref, defaultBranch string, changed ...string) *github.PushEvent {
		return &github.PushEvent{
			Ref:     github.Ptr(ref),
			Repo:    &github.PushEventRepository{DefaultBranch: github.Ptr(defaultBranch)},
			Commits: []*github.HeadCommit{{Modified: changed}},
		}
	}

	tests := []struct {
		name    string
		event   *github.PushEvent
		docsDir string
		want    bool
	}{
		{"docs change on default branch", push("refs/heads/main", "main", "docs/rfc/RFC-1.md"), "docs", true},
		{"config change on default branch", push("refs/heads/main", "main", ".docz.yaml"), "docs", true},
		{"non-default branch", push("refs/heads/topic", "main", "docs/rfc/RFC-1.md"), "docs", false},
		{"irrelevant path", push("refs/heads/main", "main", "src/main.go"), "docs", false},
		{"docs_dir prefix is exact", push("refs/heads/main", "main", "documentation/x.md"), "docs", false},
		{"no commits", push("refs/heads/main", "main"), "docs", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldIngest(tc.event, tc.docsDir); got != tc.want {
				t.Errorf("shouldIngest() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestOwnerName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		full      string
		wantOwner string
		wantName  string
		wantOK    bool
	}{
		{"acme/widgets", "acme", "widgets", true},
		{"acme/sub-widget.v2", "acme", "sub-widget.v2", true},
		{"noslash", "noslash", "", false}, // Cut returns (s,"",false) with no separator
		{"/widgets", "", "widgets", false},
		{"acme/", "acme", "", false},
		{"", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.full, func(t *testing.T) {
			t.Parallel()
			owner, name, ok := ownerName(&github.Repository{FullName: github.Ptr(tc.full)})
			if owner != tc.wantOwner || name != tc.wantName || ok != tc.wantOK {
				t.Errorf("ownerName(%q) = (%q,%q,%v), want (%q,%q,%v)",
					tc.full, owner, name, ok, tc.wantOwner, tc.wantName, tc.wantOK)
			}
		})
	}
}
