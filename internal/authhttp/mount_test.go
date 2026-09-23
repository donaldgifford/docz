package authhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/donaldgifford/docz-api/internal/auth"
	"github.com/donaldgifford/docz-api/internal/store"
)

func TestMountPublicRegistersRoutes(t *testing.T) {
	h, _, _ := newHandler(githubStub())
	r := chi.NewRouter()
	h.MountPublic(r)

	// /auth/login is now routable (it redirects to the provider).
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/auth/login?provider=github", nil))
	if rec.Code != http.StatusFound {
		t.Errorf("mounted /auth/login status = %d, want 302", rec.Code)
	}
}

func TestMountAPIRegistersRoutes(t *testing.T) {
	h, _, _ := newHandler(githubStub())
	r := chi.NewRouter()
	h.MountAPI(r)

	// /auth/session is routable; without a session in context it is a 401
	// (not a 404), proving the route exists.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/auth/session", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("mounted /auth/session status = %d, want 401 (route exists, no session)", rec.Code)
	}
}

// erroringUsers fails UpsertUser, exercising callback's serverError (500) path.
type erroringUsers struct{}

func (erroringUsers) UpsertUser(context.Context, store.UserInput) (int64, error) {
	return 0, errors.New("db down")
}

func TestCallbackUpsertFailureIs500(t *testing.T) {
	h := New(auth.NewRegistry([]auth.Provider{githubStub()}),
		&fakeSessions{}, erroringUsers{}, []byte(testStateSecret))

	state, err := auth.EncodeState([]byte(testStateSecret), "github")
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}
	rec := httptest.NewRecorder()
	h.callback(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet,
		"/auth/callback?code=abc&state="+url.QueryEscape(state), nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 when the user upsert fails", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"error"`) {
		t.Errorf("body = %q, want an opaque error envelope", body)
	}
}
