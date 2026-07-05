package main

import (
	"context"
	"strings"
	"time"

	"github.com/donaldgifford/docz-api/internal/auth"
	"github.com/donaldgifford/docz-api/internal/config"
)

// oidcDiscoveryTimeout bounds OIDC issuer discovery at startup so a
// misconfigured or unreachable issuer fails the boot, not the first login.
const oidcDiscoveryTimeout = 15 * time.Second

// buildAuthProviders constructs the enabled auth providers from config. GitHub
// needs no discovery; the OIDC providers (okta/keycloak) discover against their
// issuer using ctx, so a bad issuer surfaces here at startup.
func buildAuthProviders(ctx context.Context, cfg *config.Config) ([]auth.Provider, error) {
	redirectURL := cfg.Auth.RedirectBase + "/auth/callback"
	var providers []auth.Provider

	if cfg.AuthEnabled("github") {
		providers = append(providers, auth.NewGitHubProvider(
			cfg.Auth.GitHub.ClientID, cfg.Auth.GitHub.ClientSecret.Reveal(), redirectURL))
	}
	if cfg.AuthEnabled("okta") {
		p, err := auth.NewOIDCProvider(ctx, "okta",
			cfg.Auth.Okta.Issuer, cfg.Auth.Okta.ClientID,
			cfg.Auth.Okta.ClientSecret.Reveal(), redirectURL)
		if err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	if cfg.AuthEnabled("keycloak") {
		p, err := auth.NewOIDCProvider(ctx, "keycloak",
			cfg.Auth.Keycloak.Issuer, cfg.Auth.Keycloak.ClientID,
			cfg.Auth.Keycloak.ClientSecret.Reveal(), redirectURL)
		if err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	return providers, nil
}

// cookiesSecure reports whether session cookies should carry the Secure
// attribute, derived from the redirect base scheme (https → Secure).
func cookiesSecure(cfg *config.Config) bool {
	return strings.HasPrefix(cfg.Auth.RedirectBase, "https://")
}
