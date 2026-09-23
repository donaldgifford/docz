package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCProvider authenticates site users via standard OIDC discovery. It backs
// both Okta and Keycloak — they differ only by issuer and credentials.
type OIDCProvider struct {
	name     string
	verifier *oidc.IDTokenVerifier
	oauth    *oauth2.Config
}

var _ Provider = (*OIDCProvider)(nil)

// NewOIDCProvider performs OIDC discovery against issuer and builds a provider.
// It does network I/O, so it is called at startup with a bounded context: a
// misconfigured issuer fails the boot rather than the first login. redirectURL
// is the absolute /auth/callback URL registered with the OIDC client.
//
// scopes are the requested scopes beyond openid, which is always sent because
// OIDC requires it. They are configurable because an issuer rejects the whole
// authorize request with invalid_scope when the client is not assigned a
// requested scope — so a scope no deployment universally has (notably
// "groups") must not be hardcoded. A nil or empty slice requests openid alone.
func NewOIDCProvider(
	ctx context.Context, name, issuer, clientID, clientSecret, redirectURL string, scopes []string,
) (*OIDCProvider, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery for %s: %w", name, err)
	}
	return &OIDCProvider{
		name:     name,
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
		oauth: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       withOpenID(scopes),
			Endpoint:     provider.Endpoint(),
		},
	}, nil
}

// withOpenID returns scopes with openid guaranteed first and no duplicates, so
// a configured list can neither drop the scope OIDC mandates nor request it
// twice by naming it explicitly.
func withOpenID(scopes []string) []string {
	out := make([]string, 0, len(scopes)+1)
	out = append(out, oidc.ScopeOpenID)
	seen := map[string]bool{oidc.ScopeOpenID: true}
	for _, s := range scopes {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// Name returns the provider key ("okta" | "keycloak").
func (o *OIDCProvider) Name() string { return o.name }

// AuthCodeURL returns the issuer's authorize URL carrying state.
func (o *OIDCProvider) AuthCodeURL(state string) string {
	return o.oauth.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// Exchange trades the code for a token, verifies the returned id_token, and
// maps its standard + group claims to an Identity.
func (o *OIDCProvider) Exchange(ctx context.Context, code string) (*Identity, error) {
	tok, err := o.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("%s token exchange: %w", o.name, err)
	}

	rawIDToken, ok := tok.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("%s: id_token missing from token response", o.name)
	}
	idToken, err := o.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("%s id_token verify: %w", o.name, err)
	}

	// Groups is read unconditionally but never required: it arrives only when
	// the issuer puts the claim in the id_token, which needs either the
	// "groups" scope (opt-in via {OKTA,KEYCLOAK}_SCOPES) or a claim mapper.
	// Absent, it stays nil and drops out of the session response.
	var claims struct {
		Sub           string   `json:"sub"`
		Email         string   `json:"email"`
		EmailVerified *bool    `json:"email_verified"`
		Groups        []string `json:"groups"`
	}
	if cerr := idToken.Claims(&claims); cerr != nil {
		return nil, fmt.Errorf("%s id_token claims: %w", o.name, cerr)
	}

	// Only trust an email the issuer asserts is verified, mirroring the GitHub
	// provider (which requires a primary + verified address). A pointer lets us
	// distinguish "asserted false" (drop the email) from "claim absent" (the
	// issuer makes no claim, so we keep the value rather than surprise providers
	// that omit it).
	email := claims.Email
	if claims.EmailVerified != nil && !*claims.EmailVerified {
		email = ""
	}

	return &Identity{
		Provider: o.name,
		Subject:  claims.Sub,
		Email:    email,
		Groups:   claims.Groups,
	}, nil
}
