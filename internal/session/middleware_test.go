package session

import (
	"context"
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
