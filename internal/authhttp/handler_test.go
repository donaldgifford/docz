package authhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/donaldgifford/docz-api/internal/auth"
	"github.com/donaldgifford/docz-api/internal/session"
	"github.com/donaldgifford/docz-api/internal/store"
)

const testStateSecret = "test-state-secret"

// --- fakes ---------------------------------------------------------------

type stubProvider struct {
	name        string
	authURL     string
	identity    *auth.Identity
	exchangeErr error
}

func (s *stubProvider) Name() string                    { return s.name }
func (s *stubProvider) AuthCodeURL(state string) string { return s.authURL + "?state=" + state }

func (s *stubProvider) Exchange(_ context.Context, _ string) (*auth.Identity, error) {
	if s.exchangeErr != nil {
		return nil, s.exchangeErr
	}
	return s.identity, nil
}

type fakeUsers struct{ upserts []store.UserInput }

func (f *fakeUsers) UpsertUser(_ context.Context, in store.UserInput) (int64, error) {
	f.upserts = append(f.upserts, in)
	return int64(len(f.upserts)), nil
}

type fakeSessions struct {
	issued        []auth.Identity
	revoked       []string
	cookieSet     bool
	cookieCleared bool
}

func (f *fakeSessions) Issue(_ context.Context, identity *auth.Identity) (string, error) {
	f.issued = append(f.issued, *identity)
	return "issued-session-id", nil
}

func (f *fakeSessions) Revoke(_ context.Context, sessionID string) error {
	f.revoked = append(f.revoked, sessionID)
	return nil
}

func (f *fakeSessions) SetCookie(w http.ResponseWriter, sessionID string) {
	f.cookieSet = true
	http.SetCookie(w, &http.Cookie{Name: "docz_session", Value: sessionID, Path: "/"})
}

func (f *fakeSessions) ClearCookie(w http.ResponseWriter) {
	f.cookieCleared = true
	http.SetCookie(w, &http.Cookie{Name: "docz_session", Value: "", Path: "/", MaxAge: -1})
}

// fakeLookuper feeds session.Middleware a fixed session for the protected-route
// tests.
type fakeLookuper struct{ sess session.Session }

func (f *fakeLookuper) Lookup(_ context.Context, _ string) (session.Session, error) {
	return f.sess, nil
}

func newHandler(providers ...auth.Provider) (*Handler, *fakeUsers, *fakeSessions) {
	users := &fakeUsers{}
	sessions := &fakeSessions{}
	h := New(auth.NewRegistry(providers), sessions, users, []byte(testStateSecret))
	return h, users, sessions
}

func githubStub() *stubProvider {
	return &stubProvider{
		name:    "github",
		authURL: "https://github.com/login/oauth/authorize",
		identity: &auth.Identity{
			Provider: "github", Subject: "1001", Email: "octo@github.test", Login: "octocat",
		},
	}
}

// --- login ---------------------------------------------------------------

func TestLoginRedirectsToProvider(t *testing.T) {
	h, _, _ := newHandler(githubStub())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/login?provider=github", nil)
	rr := httptest.NewRecorder()
	h.login(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://github.com/login/oauth/authorize") || !strings.Contains(loc, "state=") {
		t.Errorf("Location = %q, want the provider authorize URL with state", loc)
	}
}

func TestLoginUnknownProvider(t *testing.T) {
	h, _, _ := newHandler(githubStub())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/login?provider=okta", nil)
	rr := httptest.NewRecorder()
	h.login(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for a disabled provider", rr.Code)
	}
}

// --- callback ------------------------------------------------------------

func TestCallbackIssuesSession(t *testing.T) {
	h, users, sessions := newHandler(githubStub())

	state, err := auth.EncodeState([]byte(testStateSecret), "github")
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}
	target := "/auth/callback?code=abc123&state=" + url.QueryEscape(state)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
	rr := httptest.NewRecorder()
	h.callback(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302 after login", rr.Code)
	}
	if len(users.upserts) != 1 || users.upserts[0].Subject != "1001" || users.upserts[0].Login != "octocat" {
		t.Errorf("upserts = %+v, want one for github/1001/octocat", users.upserts)
	}
	if len(sessions.issued) != 1 || sessions.issued[0].Provider != "github" {
		t.Errorf("issued = %+v, want one github session", sessions.issued)
	}
	if !sessions.cookieSet {
		t.Error("session cookie was not set")
	}
}

func TestCallbackRejects(t *testing.T) {
	validState, err := auth.EncodeState([]byte(testStateSecret), "github")
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}

	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"invalid state", "code=abc&state=forged.sig", http.StatusBadRequest},
		{"missing code", "state=" + url.QueryEscape(validState), http.StatusBadRequest},
		{"provider error", "error=access_denied&state=" + url.QueryEscape(validState), http.StatusUnauthorized},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, users, sessions := newHandler(githubStub())
			req := httptest.NewRequestWithContext(
				context.Background(), http.MethodGet, "/auth/callback?"+tc.query, nil)
			rr := httptest.NewRecorder()
			h.callback(rr, req)

			if rr.Code != tc.want {
				t.Errorf("status = %d, want %d", rr.Code, tc.want)
			}
			if len(users.upserts) != 0 || len(sessions.issued) != 0 {
				t.Errorf("a rejected callback wrote state: upserts=%d issued=%d",
					len(users.upserts), len(sessions.issued))
			}
		})
	}
}

func TestCallbackExchangeFailure(t *testing.T) {
	stub := githubStub()
	stub.exchangeErr = errors.New("token exchange boom")
	h, users, _ := newHandler(stub)

	state, err := auth.EncodeState([]byte(testStateSecret), "github")
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet,
		"/auth/callback?code=abc&state="+url.QueryEscape(state), nil)
	rr := httptest.NewRecorder()
	h.callback(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 on exchange failure", rr.Code)
	}
	if len(users.upserts) != 0 {
		t.Error("user upserted despite a failed exchange")
	}
}

// --- session / logout (behind the session middleware) --------------------

func TestGetSessionReturnsUser(t *testing.T) {
	h, _, _ := newHandler(githubStub())
	sess := session.Session{ID: "sid", Identity: auth.Identity{
		Provider: "okta", Subject: "u1", Login: "", Email: "u@ok.test", Groups: []string{"eng"},
	}}
	handler := session.Middleware(&fakeLookuper{sess: sess})(http.HandlerFunc(h.getSession))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/session", nil)
	req.AddCookie(&http.Cookie{Name: "docz_session", Value: "sid"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"provider":"okta"`) || !strings.Contains(body, `"eng"`) {
		t.Errorf("body = %s, want okta identity with groups", body)
	}
}

func TestGetSessionUnauthenticated(t *testing.T) {
	h, _, _ := newHandler(githubStub())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/session", nil)
	rr := httptest.NewRecorder()
	h.getSession(rr, req) // no session middleware → no session in context

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 without a session", rr.Code)
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	h, _, sessions := newHandler(githubStub())
	sess := session.Session{ID: "revoke-me", Identity: auth.Identity{Provider: "github", Subject: "9"}}
	handler := session.Middleware(&fakeLookuper{sess: sess})(http.HandlerFunc(h.logout))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "docz_session", Value: "revoke-me"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(sessions.revoked) != 1 || sessions.revoked[0] != "revoke-me" {
		t.Errorf("revoked = %v, want [revoke-me]", sessions.revoked)
	}
	if !sessions.cookieCleared {
		t.Error("logout did not clear the cookie")
	}
}
