package authorize

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/donaldgifford/docz/v2/internal/auth"
	"github.com/donaldgifford/docz/v2/internal/session"
)

func TestScopesFor(t *testing.T) {
	t.Parallel()

	scopeMap := map[string][]string{
		"okta:group:platform": {"team:read", "platform:read"},
		"github:login:donald": {"docz:read"},
		"okta:login:donald":   {"okta-only:read"},
	}

	tests := []struct {
		name     string
		identity auth.Identity
		want     []string
	}{
		{"no groups, no login", auth.Identity{Provider: "github"}, []string{"public:read"}},
		{"group match", auth.Identity{Provider: "okta", Groups: []string{"platform"}}, []string{"platform:read", "public:read", "team:read"}},
		{"login match", auth.Identity{Provider: "github", Login: "donald"}, []string{"docz:read", "public:read"}},
		{"login on another provider", auth.Identity{Provider: "keycloak", Login: "donald"}, []string{"public:read"}},
		{
			"group and login, de-duplicated",
			auth.Identity{Provider: "okta", Login: "donald", Groups: []string{"platform", "platform"}},
			[]string{"okta-only:read", "platform:read", "public:read", "team:read"},
		},
		{"nil map", auth.Identity{Provider: "okta", Groups: []string{"platform"}}, []string{"public:read"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := scopeMap
			if tt.name == "nil map" {
				m = nil
			}
			if got := ScopesFor(&tt.identity, m); !slices.Equal(got, tt.want) {
				t.Errorf("ScopesFor(%+v) = %v, want %v", tt.identity, got, tt.want)
			}
		})
	}
}

type fakeLister struct {
	byScope map[string][]int64
	calls   atomic.Int32
	err     error
}

func (f *fakeLister) ListRepoIDsForScopes(_ context.Context, scopes []string) ([]int64, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, f.err
	}

	var out []int64
	for _, s := range scopes {
		for _, id := range f.byScope[s] {
			if !slices.Contains(out, id) {
				out = append(out, id)
			}
		}
	}
	slices.Sort(out)

	return out, nil
}

// requestWith carries sess the way authorize.Middleware hands it to Allowed:
// on the request's context.
func requestWith(sess *session.Session) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/repos", http.NoBody)

	return r.WithContext(session.ContextWithSession(r.Context(), sess))
}

func TestScopeAuthorizerAllowed(t *testing.T) {
	t.Parallel()

	lister := &fakeLister{byScope: map[string][]int64{
		"public:read": {1, 2},
		"team:read":   {3},
	}}
	a := NewScopeAuthorizer(lister, 0)

	tests := []struct {
		name string
		sess session.Session
		want AllowedRepos
	}{
		{"public", session.Session{Scopes: []string{"public:read"}}, AllowedRepos{1, 2}},
		{"team", session.Session{Scopes: []string{"public:read", "team:read"}}, AllowedRepos{1, 2, 3}},
		{"no scopes", session.Session{}, AllowedRepos{}},
		{"unknown scope", session.Session{Scopes: []string{"nobody:read"}}, AllowedRepos{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := requestWith(&tt.sess)
			got, err := a.Allowed(r.Context(), r)
			if err != nil {
				t.Fatalf("Allowed: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Allowed(%v) = %v, want %v", tt.sess.Scopes, got, tt.want)
			}
		})
	}

	t.Run("no session sees nothing", func(t *testing.T) {
		t.Parallel()

		r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/repos", http.NoBody)
		got, err := a.Allowed(r.Context(), r)
		if err != nil || len(got) != 0 {
			t.Errorf("Allowed(no session) = %v, %v; want empty, nil", got, err)
		}
	})

	t.Run("lister error", func(t *testing.T) {
		t.Parallel()

		broken := NewScopeAuthorizer(&fakeLister{err: errors.New("db down")}, 0)
		r := requestWith(&tests[0].sess)
		if _, err := broken.Allowed(r.Context(), r); err == nil {
			t.Error("Allowed with a failing lister = nil error, want error")
		}
	})
}

func TestScopeAuthorizerCache(t *testing.T) {
	t.Parallel()

	lister := &fakeLister{byScope: map[string][]int64{"public:read": {1}, "team:read": {2}}}
	a := NewScopeAuthorizer(lister, 30*time.Second)
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return now }

	sess := session.Session{
		Identity: auth.Identity{Provider: "github", Subject: "1"},
		Scopes:   []string{"public:read"},
	}

	allowed := func(s *session.Session) AllowedRepos {
		t.Helper()
		r := requestWith(s)
		got, err := a.Allowed(r.Context(), r)
		if err != nil {
			t.Fatal(err)
		}

		return got
	}

	for range 3 {
		allowed(&sess)
	}
	if n := lister.calls.Load(); n != 1 {
		t.Errorf("calls after three requests within the TTL = %d, want 1", n)
	}

	// A different principal is its own entry.
	other := sess
	other.Identity.Subject = "2"
	allowed(&other)
	if n := lister.calls.Load(); n != 2 {
		t.Errorf("calls after a second principal = %d, want 2", n)
	}

	// The same principal with different scopes is recomputed.
	wider := sess
	wider.Scopes = []string{"public:read", "team:read"}
	got := allowed(&wider)
	if n := lister.calls.Load(); !slices.Equal(got, AllowedRepos{1, 2}) || n != 3 {
		t.Errorf("Allowed(wider) = %v (calls %d), want [1 2] and 3 calls", got, n)
	}

	// Expiry recomputes.
	now = now.Add(31 * time.Second)
	allowed(&wider)
	if n := lister.calls.Load(); n != 4 {
		t.Errorf("calls after the TTL = %d, want 4", n)
	}

	// Forget drops the entry.
	a.Forget(wider.PrincipalKey())
	allowed(&wider)
	if n := lister.calls.Load(); n != 5 {
		t.Errorf("calls after Forget = %d, want 5", n)
	}
}
