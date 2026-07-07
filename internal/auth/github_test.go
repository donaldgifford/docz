package auth

import (
	"net/url"
	"strings"
	"testing"
)

func TestGitHubProviderName(t *testing.T) {
	p := NewGitHubProvider("client-id", "client-secret", "https://docz.test/auth/callback")
	if p.Name() != "github" {
		t.Errorf("Name() = %q, want github", p.Name())
	}
}

func TestGitHubProviderAuthCodeURL(t *testing.T) {
	p := NewGitHubProvider("cid-123", "secret", "https://docz.test/auth/callback")
	raw := p.AuthCodeURL("state-xyz")

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("AuthCodeURL produced an unparseable URL %q: %v", raw, err)
	}
	if u.Host != "github.com" || u.Path != "/login/oauth/authorize" {
		t.Errorf("authorize URL = %s, want github.com/login/oauth/authorize", u.Host+u.Path)
	}
	q := u.Query()
	if q.Get("client_id") != "cid-123" {
		t.Errorf("client_id = %q, want cid-123", q.Get("client_id"))
	}
	if q.Get("state") != "state-xyz" {
		t.Errorf("state = %q, want state-xyz", q.Get("state"))
	}
	if q.Get("redirect_uri") != "https://docz.test/auth/callback" {
		t.Errorf("redirect_uri = %q, want the callback URL", q.Get("redirect_uri"))
	}
	// The read:user + user:email scopes back the Identity built in Exchange.
	if scopes := q.Get("scope"); !strings.Contains(scopes, "read:user") || !strings.Contains(scopes, "user:email") {
		t.Errorf("scope = %q, want read:user and user:email", scopes)
	}
}
