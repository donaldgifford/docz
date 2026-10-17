package session

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/donaldgifford/docz/v2/internal/auth"
	"github.com/donaldgifford/docz/v2/internal/oauth"
)

// TokenVerifier is the bearer half of Authn; *oauth.Verifier satisfies it.
type TokenVerifier interface {
	Verify(ctx context.Context, raw string) (oauth.Claims, error)
}

var _ TokenVerifier = (*oauth.Verifier)(nil)

// ScopeFunc computes a cookie session's scopes from its identity, per
// request (DESIGN-0023 §7, IMPL-0026 question 5). authorize.ScopesFor bound
// to the scope map is the production one.
type ScopeFunc func(identity *auth.Identity) []string

// AuthnOptions configures Authn.
type AuthnOptions struct {
	// Sessions resolves the cookie; nil only with Anonymous.
	Sessions lookuper
	// Verifier accepts a bearer token; nil means there is no bearer path
	// (MCP off) and an Authorization header is ignored.
	Verifier TokenVerifier
	// Scopes computes a cookie session's scopes; nil means public:read.
	Scopes ScopeFunc
	// Anonymous is AUTH_PROVIDERS=none: no cookie, every request is the
	// anonymous principal. A bearer token is still verified when a Verifier
	// is set, so a token caller is named in the audit line.
	Anonymous bool
	// ResourceMetadata is the URL the 401 challenge points at
	// (<issuer>/.well-known/oauth-protected-resource); empty means no
	// challenge header.
	ResourceMetadata string
}

// Authn is the one authentication middleware for /api/v1 and /mcp
// (DESIGN-0023 §6): a bearer token first, which on any failure is a 401
// with the challenge and never a fall-through; then the cookie, with
// Middleware's 401/503 split; then the anonymous principal under
// AUTH_PROVIDERS=none. Every 401 it writes carries the WWW-Authenticate
// challenge when ResourceMetadata is set.
func Authn(opts AuthnOptions) func(http.Handler) http.Handler {
	scopes := opts.Scopes
	if scopes == nil {
		scopes = func(*auth.Identity) []string { return []string{"public:read"} }
	}
	cookie := Middleware(opts.Sessions)
	if opts.Anonymous {
		cookie = AnonymousMiddleware()
	}

	return func(next http.Handler) http.Handler {
		withCookie := cookie(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, ok := FromContext(r.Context())
			if !ok {
				unauthorized(w, opts.ResourceMetadata)

				return
			}
			if sess.Via == ViaCookie {
				sess.Scopes = scopes(&sess.Identity)
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, sess)))
		}))

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, present := bearer(r)
			if !present || opts.Verifier == nil {
				withChallenge(withCookie, opts.ResourceMetadata).ServeHTTP(w, r)

				return
			}

			claims, err := opts.Verifier.Verify(r.Context(), raw)
			if err != nil {
				slog.DebugContext(r.Context(), "bearer token rejected", "err", err, "path", r.URL.Path)
				unauthorized(w, opts.ResourceMetadata)

				return
			}

			sess := FromClaims(&claims)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, sess)))
		})
	}
}

// FromClaims turns verified token claims into a Session: a subject docz-api
// minted (`user:<provider>:<subject>`) resolves to the person, so their
// cookie and token are one principal; any other subject is the issuer's.
func FromClaims(c *oauth.Claims) Session {
	identity := auth.Identity{Provider: c.Issuer, Subject: c.Subject, Email: c.Email, Login: c.Login}
	if provider, subject, ok := oauth.UserSubject(c.Subject); ok {
		identity.Provider, identity.Subject = provider, subject
	}

	id := c.ID
	if id == "" {
		id = c.Issuer + ":" + c.Subject
	}

	return Session{
		ID:       id,
		Identity: identity,
		Via:      ViaToken,
		Scopes:   c.Scopes,
		ClientID: c.ClientID,
		Issuer:   c.Issuer,
	}
}

// bearer returns the Authorization header's bearer token, when there is one.
func bearer(r *http.Request) (string, bool) {
	scheme, raw, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	raw = strings.TrimSpace(raw)

	return raw, raw != ""
}

// withChallenge adds the WWW-Authenticate header to a 401 the wrapped
// cookie path writes, so a token client learns where the metadata is even
// when it sent no token.
func withChallenge(next http.Handler, resourceMetadata string) http.Handler {
	if resourceMetadata == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&challengeWriter{ResponseWriter: w, challenge: challenge(resourceMetadata)}, r)
	})
}

// challengeWriter sets the challenge header the moment a 401 status is
// written and leaves every other status alone.
type challengeWriter struct {
	http.ResponseWriter
	challenge string
}

func (c *challengeWriter) WriteHeader(status int) {
	if status == http.StatusUnauthorized {
		c.Header().Set("WWW-Authenticate", c.challenge)
	}
	c.ResponseWriter.WriteHeader(status)
}

// challenge is the RFC 9728 §5.1 challenge: where the protected resource
// metadata is and the scope a first request needs.
func challenge(resourceMetadata string) string {
	return fmt.Sprintf(`Bearer resource_metadata=%q, scope="public:read"`, resourceMetadata)
}

// unauthorized writes the 401 envelope with the challenge.
func unauthorized(w http.ResponseWriter, resourceMetadata string) {
	if resourceMetadata != "" {
		w.Header().Set("WWW-Authenticate", challenge(resourceMetadata))
	}
	writeUnauthorized(w)
}
