package auth

import (
	"context"
	"testing"
)

// stubProvider is a Provider that needs no network, for registry tests.
type stubProvider struct{ name string }

func (s stubProvider) Name() string { return s.name }
func (s stubProvider) AuthCodeURL(state string) string {
	return "https://example.test/" + s.name + "?state=" + state
}

func (s stubProvider) Exchange(_ context.Context, _ string) (*Identity, error) {
	return &Identity{Provider: s.name, Subject: "sub-" + s.name}, nil
}

func TestRegistry(t *testing.T) {
	t.Parallel()
	reg := NewRegistry([]Provider{stubProvider{name: "github"}, stubProvider{name: "okta"}})

	if p, ok := reg.Get("github"); !ok || p.Name() != "github" {
		t.Errorf("Get(github) = (%v, %v), want a github provider", p, ok)
	}
	if _, ok := reg.Get("keycloak"); ok {
		t.Error("Get(keycloak) ok = true, want false (not enabled)")
	}

	names := reg.Names()
	if len(names) != 2 || names[0] != "github" || names[1] != "okta" {
		t.Errorf("Names() = %v, want sorted [github okta]", names)
	}
}
