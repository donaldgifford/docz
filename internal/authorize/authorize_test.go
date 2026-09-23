package authorize

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/donaldgifford/docz-api/internal/store"
)

// stubLister returns fixed repos (or an error) for the authorizer.
type stubLister struct {
	repos []store.Repo
	err   error
}

func (s stubLister) ListRepos(context.Context) ([]store.Repo, error) {
	return s.repos, s.err
}

func TestAllowedReposContains(t *testing.T) {
	a := AllowedRepos{1, 2, 3}
	if !a.Contains(2) {
		t.Error("Contains(2) = false, want true")
	}
	if a.Contains(9) {
		t.Error("Contains(9) = true, want false")
	}
	if (AllowedRepos(nil)).Contains(1) {
		t.Error("nil set Contains(1) = true, want false")
	}
}

func TestAllReposAuthorizerReturnsAllIDs(t *testing.T) {
	lister := stubLister{repos: []store.Repo{{ID: 10}, {ID: 20}, {ID: 30}}}
	a := NewAllReposAuthorizer(lister)

	got, err := a.Allowed(t.Context(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody))
	if err != nil {
		t.Fatalf("Allowed: %v", err)
	}
	want := AllowedRepos{10, 20, 30}
	if len(got) != len(want) {
		t.Fatalf("Allowed = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Allowed[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestMiddlewareInjectsAllowedRepos(t *testing.T) {
	lister := stubLister{repos: []store.Repo{{ID: 7}}}
	mw := Middleware(NewAllReposAuthorizer(lister))

	var seen AllowedRepos
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = FromContext(r.Context())
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/repos", http.NoBody)
	mw(next).ServeHTTP(httptest.NewRecorder(), req)

	if !seen.Contains(7) {
		t.Errorf("handler saw allowed = %v, want it to contain 7", seen)
	}
}

func TestMiddlewareAuthorizerErrorIs500(t *testing.T) {
	lister := stubLister{err: errors.New("db down")}
	mw := Middleware(NewAllReposAuthorizer(lister))

	called := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/repos", http.NoBody)
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if called {
		t.Error("next handler was called despite an authorizer error")
	}
}

// logCounter counts records at each level so a test can assert an outage was
// actually reported.
type logCounter struct {
	errors int
}

func (*logCounter) Enabled(context.Context, slog.Level) bool { return true }

//nolint:gocritic // hugeParam: signature mandated by slog.Handler.
func (c *logCounter) Handle(_ context.Context, rec slog.Record) error {
	if rec.Level == slog.LevelError {
		c.errors++
	}
	return nil
}

func (c *logCounter) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *logCounter) WithGroup(string) slog.Handler      { return c }

// The production authorizer reads repos from Postgres, so this middleware is
// where a database outage on the hot /api/v1 path surfaces. It must leave a
// record: before this, the wrapped error chain died here and the outage showed
// only as unexplained 500s (INV-0007 F7.1).
func TestMiddlewareLogsAuthorizerError(t *testing.T) {
	counter := &logCounter{}
	prev := slog.Default()
	slog.SetDefault(slog.New(counter))
	t.Cleanup(func() { slog.SetDefault(prev) })

	mw := Middleware(NewAllReposAuthorizer(stubLister{err: errors.New("db down")}))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/repos", http.NoBody)
	rec := httptest.NewRecorder()

	mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rec, req)

	if counter.errors != 1 {
		t.Errorf("error records = %d, want 1", counter.errors)
	}
}
