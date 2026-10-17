package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/donaldgifford/docz/v2/internal/auth"
	"github.com/donaldgifford/docz/v2/internal/store"
	doczcfg "github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// Lifetimes and key prefixes of the flow's transient state (DESIGN-0023
// §5): a pending authorization waits ten minutes for the person, a code
// sixty seconds for the client, and a redeemed code is remembered long
// enough that a replay revokes what it issued.
const (
	pendingTTL     = 10 * time.Minute
	codeTTL        = time.Minute
	redeemedTTL    = 10 * time.Minute
	pendingPrefix  = "authz:"
	codePrefix     = "code:"
	redeemedPrefix = "redeemed:"
	secretBytes    = 32
)

// Principal is what the consent endpoints need of the site session: the
// session id the request is bound to, the person, and the scopes they hold.
// The package takes it through a function so it depends on no session
// package, which depends on this one for Claims.
type Principal struct {
	SessionID string
	Identity  auth.Identity
	Scopes    []string
}

// Store is the durable surface the server needs; *store.Store satisfies it.
type Store interface {
	UpsertUser(ctx context.Context, in store.UserInput) (int64, error)
	GetUser(ctx context.Context, id int64) (store.TokenUser, error)
	InsertRefreshToken(ctx context.Context, hash, familyID string, userID int64, clientID, scope string) error
	GetRefreshToken(ctx context.Context, hash string) (store.RefreshToken, error)
	RotateRefreshToken(ctx context.Context, oldHash, newHash, scope string) (store.RefreshToken, bool, error)
	RevokeRefreshFamily(ctx context.Context, familyID string) error
	UpsertConsent(ctx context.Context, userID int64, clientID, scope string) error
}

var _ Store = (*store.Store)(nil)

// ServerOptions configures a Server.
type ServerOptions struct {
	Issuer   string
	Audience string
	// SiteURL is docz-site's base, where /oauth/authorize sends the person.
	SiteURL string
	Signer  *Signer
	// ScopeMap is AUTHZ_SCOPE_MAP; its scopes and public:read are what an
	// authorization may ask for.
	ScopeMap map[string][]string
	Clients  *ClientFetcher
	// Cache holds pending authorizations and codes; nil means a MemoryCache.
	Cache      Cache
	Store      Store
	RefreshTTL time.Duration
	// Session reads the site session off a consent request's context.
	Session func(ctx context.Context) (Principal, bool)
	// Observe is called once per token request with the grant and
	// "ok" or "rejected"; nil means no observation.
	Observe func(grant, status string)
}

// Server is docz-api's authorization server (DESIGN-0023 §5): the
// authorization, token, and revocation endpoints, the consent endpoints
// docz-site calls, and the metadata documents.
type Server struct {
	issuer     string
	audience   string
	siteURL    string
	signer     *Signer
	meta       *Metadata
	clients    *ClientFetcher
	cache      Cache
	store      Store
	refreshTTL time.Duration
	session    func(ctx context.Context) (Principal, bool)
	observe    func(grant, status string)
	now        func() time.Time
}

// NewServer builds a Server.
func NewServer(opts *ServerOptions) *Server {
	cache := opts.Cache
	if cache == nil {
		cache = NewMemoryCache()
	}
	observe := opts.Observe
	if observe == nil {
		observe = func(string, string) {}
	}

	var scopeMapScopes []string
	for _, scopes := range opts.ScopeMap {
		scopeMapScopes = append(scopeMapScopes, scopes...)
	}

	return &Server{
		issuer:     opts.Issuer,
		audience:   opts.Audience,
		siteURL:    opts.SiteURL,
		signer:     opts.Signer,
		meta:       NewMetadata(opts.Issuer, opts.Audience, scopeMapScopes, opts.Signer.JWKS()),
		clients:    opts.Clients,
		cache:      cache,
		store:      opts.Store,
		refreshTTL: opts.RefreshTTL,
		session:    opts.Session,
		observe:    observe,
		now:        time.Now,
	}
}

// Metadata returns the issuer's documents.
func (s *Server) Metadata() *Metadata { return s.meta }

// MountPublic serves the metadata documents and the three OAuth endpoints
// on r, outside every gate.
func (s *Server) MountPublic(r chi.Router) {
	s.meta.Mount(r)
	r.Get(AuthorizePath, s.authorize)
	r.Post(TokenPath, s.token)
	r.Post(RevokePath, s.revoke)
}

// MountAPI serves the consent endpoints on r, which the caller has placed
// inside the gated /api/v1 group.
func (s *Server) MountAPI(r chi.Router) {
	r.Get("/oauth/authorizations/{id}", s.getAuthorization)
	r.Post("/oauth/authorizations/{id}", s.decideAuthorization)
}

// pending is a validated authorization request waiting for consent.
type pending struct {
	ClientID      string    `json:"client_id"`
	ClientName    string    `json:"client_name"`
	ClientURI     string    `json:"client_uri"`
	RedirectURI   string    `json:"redirect_uri"`
	CodeChallenge string    `json:"code_challenge"`
	Scopes        []string  `json:"scopes"`
	State         string    `json:"state"`
	ExpiresAt     time.Time `json:"expires_at"`
	// SessionID binds the request to the session that first read it, so
	// only that session can decide it.
	SessionID string `json:"session_id,omitempty"`
}

// code is a redeemable authorization code.
type code struct {
	Pending  pending       `json:"pending"`
	UserID   int64         `json:"user_id"`
	Identity auth.Identity `json:"identity"`
	Granted  []string      `json:"granted"`
}

// grantable is every scope an authorization may ask for.
func (s *Server) grantable() []string { return s.meta.ScopesSupported() }

// intersect returns the requested scopes the principal holds, in the
// requested order. public:read is every principal's.
func intersect(requested, held []string) []string {
	out := make([]string, 0, len(requested))
	for _, sc := range requested {
		if sc == doczcfg.ScopePublicRead || slices.Contains(held, sc) {
			out = append(out, sc)
		}
	}

	return out
}

// randomSecret is 32 random bytes, base64url: a pending id, a code, a
// refresh token, a family id.
func randomSecret() (string, error) {
	b := make([]byte, secretBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random secret: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken is the stored form of a refresh token or a code: the hex
// SHA-256, so a database read never yields a usable credential.
func hashToken(raw string) string { return hashHex(raw) }

// put stores v under key for ttl.
func (s *Server) put(ctx context.Context, key string, v any, ttl time.Duration) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return s.cache.Set(ctx, key, raw, ttl)
}

// redirectWith appends params to redirectURI's query and adds iss, as RFC
// 9207 asks on every authorization response.
func (s *Server) redirectWith(redirectURI string, params url.Values) string {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return redirectURI
	}
	q := u.Query()
	for k, vs := range params {
		for _, v := range vs {
			q.Set(k, v)
		}
	}
	q.Set("iss", s.issuer)
	u.RawQuery = q.Encode()

	return u.String()
}

// audit is the one log line per token request: the grant, the client, the
// principal, and the outcome, and never a token.
func audit(ctx context.Context, grant, clientID, principal, status string) {
	slog.InfoContext(ctx, "oauth token", "grant", grant, "client_id", clientID, "principal", principal, "status", status)
}

// s256 is the PKCE transform: base64url(sha256(verifier)).
func s256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))

	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// validPKCE is RFC 7636's grammar for a verifier or a challenge: 43 to 128
// unreserved characters.
func validPKCE(s string) bool {
	if len(s) < 43 || len(s) > 128 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '.', r == '_', r == '~':
		default:
			return false
		}
	}

	return true
}

// noStore marks a response as not cacheable, as RFC 6749 §5.1 requires of
// anything carrying a token or a code.
func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}

// itoa formats a user id for an audit line.
func itoa(n int64) string { return strconv.FormatInt(n, 10) }
