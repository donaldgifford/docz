// Package auth authenticates site users behind one Provider abstraction so the
// three supported login backends — GitHub (OAuth) and Okta/Keycloak (OIDC
// discovery) — are configured, not forked. It handles authentication only (who
// the user is); authorization (which repos a user may read) is a separate,
// deferred concern owned by the authorize seam and a future SpiceDB resolver.
//
// A Provider begins the authorization-code flow (AuthCodeURL) and completes it
// (Exchange), returning a normalized Identity. The Registry holds the enabled
// providers keyed by name; the login/callback HTTP handlers live in
// internal/authhttp and the session lives in internal/session.
package auth

import "context"

// Provider authenticates a site user via the OAuth/OIDC authorization-code
// flow. It establishes identity only; authorization is handled elsewhere.
type Provider interface {
	// Name is the provider's config key ("github" | "okta" | "keycloak").
	Name() string
	// AuthCodeURL is the provider URL that begins login, carrying the signed
	// state that /auth/callback verifies.
	AuthCodeURL(state string) string
	// Exchange trades the callback's authorization code for a normalized
	// Identity (verifying the OIDC id_token / calling the GitHub user API).
	Exchange(ctx context.Context, code string) (*Identity, error)
}

// Identity is the normalized result of a successful provider Exchange. Groups
// carries OIDC group/role claims, retained on the session for the future
// authorization layer even though authN does not use them today.
type Identity struct {
	Provider string
	Subject  string   // stable per-provider user id
	Email    string   // may be empty when the provider exposes none
	Login    string   // GitHub login; empty for OIDC providers
	Groups   []string // OIDC group/role claims; nil for GitHub OAuth
}
