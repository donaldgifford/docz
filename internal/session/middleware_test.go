package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/donaldgifford/docz-api/internal/auth"
)

// fakeLookuper resolves one known session id, mirroring Store.Lookup.
type fakeLookuper struct {
	id      string
	session Session
	err     error
}

func (f *fakeLookuper) Lookup(_ context.Context, sessionID string) (Session, error) {
	if f.err != nil {
		return Session{}, f.err
	}
	if sessionID == f.id {
		return f.session, nil
	}
	return Session{}, ErrSessionNotFound
}

func TestMiddleware(t *testing.T) {
	t.Parallel()
	// store is read-only, so it is safe to share across parallel subtests.
	want := Session{ID: "good-id", Identity: auth.Identity{Provider: "github", Subject: "42", Login: "octocat"}}
	store := &fakeLookuper{id: "good-id", session: want}

	tests := []struct {
		name        string
		cookie      *http.Cookie
		wantStatus  int
		wantReached bool
	}{
		{"valid session", &http.Cookie{Name: cookieName, Value: "good-id"}, http.StatusOK, true},
		{"unknown session", &http.Cookie{Name: cookieName, Value: "bogus"}, http.StatusUnauthorized, false},
		{"no cookie", nil, http.StatusUnauthorized, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Per-subtest state so subtests do not race.
			var got Session
			var reached bool
			protected := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				reached = true
				got, _ = FromContext(r.Context())
			})
			handler := Middleware(store)(protected)

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/repos", nil)
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tc.wantStatus)
			}
			if reached != tc.wantReached {
				t.Errorf("reached = %v, want %v", reached, tc.wantReached)
			}
			if tc.wantReached && got.Identity.Login != "octocat" {
				t.Errorf("injected identity = %+v, want octocat", got.Identity)
			}
		})
	}
}

func TestFromContextAbsent(t *testing.T) {
	t.Parallel()
	if _, ok := FromContext(context.Background()); ok {
		t.Error("FromContext on a bare context returned ok = true, want false")
	}
}

// recordingHandler captures slog records so the tests can assert on what an
// operator would see, not just on the status code.
type recordingHandler struct {
	records []slog.Record
}

func (*recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

//nolint:gocritic // hugeParam: signature mandated by slog.Handler.
func (h *recordingHandler) Handle(_ context.Context, rec slog.Record) error {
	h.records = append(h.records, rec)
	return nil
}

func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler      { return h }

func (h *recordingHandler) countAtLevel(level slog.Level) int {
	n := 0
	for i := range h.records {
		if h.records[i].Level == level {
			n++
		}
	}
	return n
}

// captureSlog installs a recording handler as the default for the test.
func captureSlog(t *testing.T) *recordingHandler {
	t.Helper()
	h := &recordingHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return h
}

// A session backend that cannot answer is an outage, not a verdict on the
// caller: it must be logged and reported as 503, so clients do not treat a
// Redis blip as a logout (INV-0007 F7.2).
func TestMiddlewareInfraErrorLogsAnd503s(t *testing.T) {
	rec := captureSlog(t)
	store := &fakeLookuper{id: "good-id", err: errors.New("redis: connection refused")}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/repos", http.NoBody)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "any"})
	w := httptest.NewRecorder()

	Middleware(store)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler ran despite a session backend failure")
	})).ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
	if got := rec.countAtLevel(slog.LevelError); got != 1 {
		t.Errorf("error records = %d, want 1", got)
	}
}

// An absent or expired session is ordinary churn: it must stay a quiet 401 so
// normal logged-out traffic does not fill the logs with errors.
func TestMiddlewareMissingSessionStaysQuiet401(t *testing.T) {
	rec := captureSlog(t)
	store := &fakeLookuper{id: "good-id"}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/repos", http.NoBody)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "stale"})
	w := httptest.NewRecorder()

	Middleware(store)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler ran without a valid session")
	})).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if got := rec.countAtLevel(slog.LevelError); got != 0 {
		t.Errorf("error records = %d, want 0 for an ordinary missing session", got)
	}
}

// A stored session that will not decode is per-cookie poison, not an outage:
// the backend answered fine. It must stay a 401 so the holder can log in again
// — /api/v1/auth/logout sits behind this same gate, so a 503 here would be a
// dead end with no way out, and docz-site only offers login on a 401.
func TestMiddlewareCorruptSessionIs401AndLogged(t *testing.T) {
	rec := captureSlog(t)
	store := &fakeLookuper{
		id: "good-id",
		// Built exactly as Store.Lookup builds it — a double %w wrapping both
		// the sentinel and the underlying decode error — so this pins the real
		// error shape, not a simplified stand-in.
		err: fmt.Errorf("%w: %w", ErrSessionCorrupt, errors.New("unexpected end of JSON input")),
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/repos", http.NoBody)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "poisoned"})
	w := httptest.NewRecorder()

	Middleware(store)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler ran with an undecodable session")
	})).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d so the holder can log in again",
			w.Code, http.StatusUnauthorized)
	}
	if got := rec.countAtLevel(slog.LevelWarn); got != 1 {
		t.Errorf("warn records = %d, want 1", got)
	}
	if got := rec.countAtLevel(slog.LevelError); got != 0 {
		t.Errorf("error records = %d, want 0 (this is not an outage)", got)
	}
}
