package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/donaldgifford/docz/v2/internal/auth"
	"github.com/donaldgifford/docz/v2/internal/authorize"
	"github.com/donaldgifford/docz/v2/internal/config"
	"github.com/donaldgifford/docz/v2/internal/oauth"
	"github.com/donaldgifford/docz/v2/internal/session"
	"github.com/donaldgifford/docz/v2/internal/store"
	"github.com/donaldgifford/docz/v2/internal/telemetry"
)

// tokenAuth is the bearer half of the gate (DESIGN-0023 §6): the verifier
// and the Redis cache its trusted issuers' key sets live in. Both are nil
// with MCP off, which is the default.
type tokenAuth struct {
	verifier *oauth.Verifier
	cache    *oauth.RedisCache
}

// buildTokenAuth builds the verifier when MCP is enabled: docz-api's own key
// when it has one, every OAUTH_TRUSTED_ISSUERS entry discovered under ctx,
// key sets cached in Redis so a restart or a second replica does not
// re-fetch them. A trusted issuer that cannot be discovered fails startup,
// as a login provider does.
func buildTokenAuth(ctx context.Context, cfg *config.Config) (tokenAuth, error) {
	if !cfg.MCP.Enabled {
		return tokenAuth{}, nil
	}

	var pub *ecdsa.PublicKey
	if cfg.OAuth.SigningKeyPEM != "" {
		key, err := cfg.OAuth.SigningKey()
		if err != nil {
			return tokenAuth{}, fmt.Errorf("parsing signing key: %w", err)
		}
		pub = &key.PublicKey
	}

	cache, err := oauth.NewRedisCache(cfg.Store.RedisURL)
	if err != nil {
		return tokenAuth{}, fmt.Errorf("building jwks cache: %w", err)
	}

	verifier, err := oauth.NewVerifier(ctx, &oauth.VerifierOptions{
		Audience:       cfg.OAuth.Audience(),
		Issuer:         cfg.OAuth.Issuer,
		Key:            pub,
		TrustedIssuers: cfg.OAuth.TrustedIssuers,
		Cache:          cache,
		Observe:        telemetry.ObserveTokenVerification,
	})
	if err != nil {
		closeJWKSCache(cache)

		return tokenAuth{}, fmt.Errorf("building token verifier: %w", err)
	}

	slog.Info("bearer tokens accepted",
		"audience", cfg.OAuth.Audience(),
		"own_issuer", pub != nil,
		"trusted_issuers", cfg.OAuth.TrustedIssuers,
	)

	return tokenAuth{verifier: verifier, cache: cache}, nil
}

// close releases the cache's Redis client.
func (t tokenAuth) close() {
	if t.cache != nil {
		closeJWKSCache(t.cache)
	}
}

func closeJWKSCache(c *oauth.RedisCache) {
	if err := c.Close(); err != nil {
		slog.Warn("close jwks cache", "err", err)
	}
}

// buildGate composes the /api/v1 gate (DESIGN-0023 §6, §7): session.Authn
// (a bearer first, then the cookie, then the anonymous principal under
// AUTH_PROVIDERS=none) over the authorize seam. With a login the authorizer
// is the scope one, so a repository is visible only to a principal holding
// one of its access.scopes; under none every onboarded repository is, as
// before. Both inject a session.Session, so everything downstream is
// identical in either mode.
func buildGate(cfg *config.Config, st *store.Store, sessionStore *session.Store, tokens tokenAuth) func(http.Handler) http.Handler {
	var authorizer authorize.Authorizer = authorize.NewScopeAuthorizer(st, cfg.Authz.CacheTTL)
	if cfg.AuthDisabled() {
		authorizer = authorize.NewAllReposAuthorizer(st)
	}

	scopeMap := cfg.Authz.ScopeMap
	opts := session.AuthnOptions{
		Sessions:  sessionStore,
		Scopes:    func(id *auth.Identity) []string { return authorize.ScopesFor(id, scopeMap) },
		Anonymous: cfg.AuthDisabled(),
	}
	if tokens.verifier != nil {
		opts.Verifier = tokens.verifier
		opts.ResourceMetadata = cfg.OAuth.Issuer + "/.well-known/oauth-protected-resource"
	}

	authn := session.Authn(opts)
	authz := authorize.Middleware(authorizer)

	return func(next http.Handler) http.Handler {
		return authn(authz(next))
	}
}

// buildAuthStack builds the bearer verifier and the authorization server
// together, since the second needs the first's cache.
func buildAuthStack(ctx context.Context, cfg *config.Config, st *store.Store) (tokenAuth, *oauth.Server, error) {
	tokens, err := buildTokenAuth(ctx, cfg)
	if err != nil {
		return tokenAuth{}, nil, fmt.Errorf("building token auth: %w", err)
	}
	authServer, err := buildAuthServer(cfg, st, tokens)
	if err != nil {
		tokens.close()

		return tokenAuth{}, nil, fmt.Errorf("building authorization server: %w", err)
	}

	return tokens, authServer, nil
}

// buildAuthServer builds docz-api's own authorization server (DESIGN-0023
// §5) when MCP is enabled and a login exists: under AUTH_PROVIDERS=none
// there is no person to consent, so nothing is mounted and nil is returned.
// Its transient state shares the token verifier's Redis cache.
func buildAuthServer(cfg *config.Config, st *store.Store, tokens tokenAuth) (*oauth.Server, error) {
	if !cfg.MCP.Enabled || cfg.AuthDisabled() {
		return nil, nil //nolint:nilnil // nil is "not mounted", which the caller tests
	}

	key, err := cfg.OAuth.SigningKey()
	if err != nil {
		return nil, fmt.Errorf("parsing signing key: %w", err)
	}
	signer, err := oauth.NewSigner(cfg.OAuth.Issuer, cfg.OAuth.Audience(), key, cfg.OAuth.AccessTTL)
	if err != nil {
		return nil, fmt.Errorf("building signer: %w", err)
	}

	var cache oauth.Cache
	if tokens.cache != nil {
		cache = tokens.cache
	}

	return oauth.NewServer(&oauth.ServerOptions{
		Issuer:     cfg.OAuth.Issuer,
		Audience:   cfg.OAuth.Audience(),
		SiteURL:    cfg.OAuth.SiteURL,
		Signer:     signer,
		ScopeMap:   cfg.Authz.ScopeMap,
		Clients:    oauth.NewClientFetcher(&oauth.ClientFetcherOptions{Cache: cache}),
		Cache:      cache,
		Store:      st,
		RefreshTTL: cfg.OAuth.RefreshTTL,
		Session:    principalFromSession,
		Observe:    telemetry.ObserveOAuthToken,
	}), nil
}

// principalFromSession is how the consent endpoints read the site session
// without the oauth package importing the session package.
func principalFromSession(ctx context.Context) (oauth.Principal, bool) {
	sess, ok := session.FromContext(ctx)
	if !ok {
		return oauth.Principal{}, false
	}

	return oauth.Principal{SessionID: sess.ID, Identity: sess.Identity, Scopes: sess.Scopes}, true
}
