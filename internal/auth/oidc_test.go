package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// discoveryServer stands up a minimal OIDC issuer that serves just the
// discovery document, enough for oidc.NewProvider to succeed offline.
func discoveryServer(t *testing.T) string {
	t.Helper()
	var issuer string
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{`+
			`"issuer":"`+issuer+`",`+
			`"authorization_endpoint":"`+issuer+`/authorize",`+
			`"token_endpoint":"`+issuer+`/token",`+
			`"jwks_uri":"`+issuer+`/keys",`+
			`"id_token_signing_alg_values_supported":["RS256"]}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	issuer = srv.URL
	return issuer
}

func TestNewOIDCProviderDiscovers(t *testing.T) {
	issuer := discoveryServer(t)
	p, err := NewOIDCProvider(
		context.Background(), "okta", issuer, "cid", "secret", "https://docz.test/auth/callback",
		[]string{"profile", "email"})
	if err != nil {
		t.Fatalf("NewOIDCProvider against a live discovery doc: %v", err)
	}
	if p.Name() != "okta" {
		t.Errorf("Name() = %q, want okta", p.Name())
	}

	raw := p.AuthCodeURL("state-1")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("AuthCodeURL unparseable %q: %v", raw, err)
	}
	if u.Path != "/authorize" {
		t.Errorf("authorize path = %q, want /authorize (from discovery)", u.Path)
	}
	q := u.Query()
	if q.Get("client_id") != "cid" || q.Get("state") != "state-1" {
		t.Errorf("client_id/state = %q/%q, want cid/state-1", q.Get("client_id"), q.Get("state"))
	}
	// OIDC requires the openid scope.
	if scope := q.Get("scope"); !strings.Contains(scope, "openid") {
		t.Errorf("scope = %q, want it to include openid", scope)
	}
	// "groups" is opt-in: an issuer rejects the whole authorize request with
	// invalid_scope when its client is not assigned a requested scope, and
	// nothing in docz-api reads the claim for an access decision.
	if scope := q.Get("scope"); strings.Contains(scope, "groups") {
		t.Errorf("scope = %q, want no groups unless configured", scope)
	}
}

// TestNewOIDCProviderScopes pins the requested scope set: openid is always
// sent (OIDC mandates it) and never duplicated, configured scopes are passed
// through in order, and an empty configuration still yields a valid request.
func TestNewOIDCProviderScopes(t *testing.T) {
	issuer := discoveryServer(t)
	tests := []struct {
		name   string
		scopes []string
		want   string
	}{
		{"default set", []string{"profile", "email"}, "openid profile email"},
		{"groups opted in", []string{"profile", "email", "groups"}, "openid profile email groups"},
		{"none configured", nil, "openid"},
		{"openid named explicitly is not doubled", []string{"openid", "profile"}, "openid profile"},
		{"empty entries dropped", []string{"", "profile", ""}, "openid profile"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewOIDCProvider(
				context.Background(), "okta", issuer, "cid", "secret",
				"https://docz.test/auth/callback", tt.scopes)
			if err != nil {
				t.Fatalf("NewOIDCProvider: %v", err)
			}
			u, err := url.Parse(p.AuthCodeURL("s"))
			if err != nil {
				t.Fatalf("AuthCodeURL unparseable: %v", err)
			}
			if got := u.Query().Get("scope"); got != tt.want {
				t.Errorf("scope = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewOIDCProviderErrorsOnBadIssuer(t *testing.T) {
	// A discovery endpoint that 404s must fail construction, not defer the error
	// to the first login.
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	if _, err := NewOIDCProvider(
		context.Background(), "keycloak", srv.URL, "cid", "secret", "https://docz.test/cb",
		nil); err == nil {
		t.Error("NewOIDCProvider with an unreachable discovery doc returned nil error")
	}
}
