package oauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// Verification bounds.
const (
	// clockSkew is the leeway on exp and nbf.
	clockSkew = 30 * time.Second
	// jwksTTL is how long an issuer's key set is cached.
	jwksTTL = time.Hour
	// jwksRefreshInterval is the least time between two refreshes of one issuer's
	// keys on an unknown kid, so a flood of bad tokens cannot flood the issuer.
	jwksRefreshInterval = time.Minute
	// maxDocument caps a discovery or JWKS document read.
	maxDocument = 1 << 20
	// fetchTimeout bounds one request to an issuer.
	fetchTimeout = 10 * time.Second
)

// Allowed signature algorithms: docz-api's own ES256 and what the trusted
// issuers sign with.
var signatureAlgorithms = []jose.SignatureAlgorithm{jose.ES256, jose.RS256, jose.ES384, jose.PS256}

// Verification errors. Every one is wrapped by ErrInvalidToken, so a caller
// that only wants "this token is refused" tests one sentinel while a log
// line keeps the cause.
var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrUnknownIssuer    = errors.New("issuer is not trusted")
	ErrUnknownKey       = errors.New("no key matches the token's kid")
	ErrWrongAudience    = errors.New("audience does not name this server")
	ErrTokenExpired     = errors.New("token expired")
	ErrTokenNotYetValid = errors.New("token not yet valid")
)

// Verifier accepts a JWT from docz-api's own key or from any trusted issuer
// (DESIGN-0023 §6): signature, iss against the set, aud contains the
// server's audience, exp and nbf with clockSkew, and the scopes read from
// scope or scp as-is.
type Verifier struct {
	audience string
	own      *ownIssuer
	trusted  map[string]*trustedIssuer
	observe  func(issuer, status string)
}

// ownIssuer is docz-api's key, resolved in memory.
type ownIssuer struct {
	issuer string
	kid    string
	key    *ecdsa.PublicKey
}

// trustedIssuer is an external issuer's key set, fetched from its jwks_uri
// and cached, refreshed at most once a minute on an unknown kid.
type trustedIssuer struct {
	issuer  string
	jwksURI string
	cache   Cache
	client  *http.Client
	now     func() time.Time

	mu          sync.Mutex
	keys        *jose.JSONWebKeySet
	lastRefresh time.Time
}

// VerifierOptions configures NewVerifier. Audience is required; everything
// else is optional, and a verifier with no key and no issuer refuses every
// token.
type VerifierOptions struct {
	// Audience is the aud every token must carry, <issuer>/mcp.
	Audience string
	// Issuer and Key are docz-api's own; Key nil means docz-api mints nothing.
	Issuer string
	Key    *ecdsa.PublicKey
	// TrustedIssuers are discovered at construction; a bad one fails it.
	TrustedIssuers []string
	// Cache holds each trusted issuer's key set; nil means a MemoryCache.
	Cache Cache
	// HTTPClient fetches discovery and JWKS documents; nil means a client
	// with fetchTimeout.
	HTTPClient *http.Client
	// Observe is called once per verification with the issuer (or "unknown")
	// and "ok" or "rejected"; nil means no observation.
	Observe func(issuer, status string)
}

// NewVerifier builds a Verifier, discovering each trusted issuer's jwks_uri
// under ctx. Like a login provider, a bad issuer fails the boot rather than
// the first token.
func NewVerifier(ctx context.Context, opts *VerifierOptions) (*Verifier, error) {
	if opts.Audience == "" {
		return nil, errors.New("verifier: audience is required")
	}

	v := &Verifier{audience: opts.Audience, trusted: make(map[string]*trustedIssuer), observe: opts.Observe}
	if v.observe == nil {
		v.observe = func(string, string) {}
	}

	if opts.Key != nil {
		kid, err := KeyID(opts.Key)
		if err != nil {
			return nil, err
		}
		v.own = &ownIssuer{issuer: opts.Issuer, kid: kid, key: opts.Key}
	}

	cache := opts.Cache
	if cache == nil {
		cache = NewMemoryCache()
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: fetchTimeout}
	}

	for _, issuer := range opts.TrustedIssuers {
		issuer = strings.TrimSuffix(issuer, "/")
		jwksURI, err := discoverJWKS(ctx, client, issuer)
		if err != nil {
			return nil, fmt.Errorf("trusted issuer %s: %w", issuer, err)
		}
		v.trusted[issuer] = &trustedIssuer{issuer: issuer, jwksURI: jwksURI, cache: cache, client: client, now: time.Now}
	}

	return v, nil
}

// Audience is the aud the verifier requires.
func (v *Verifier) Audience() string { return v.audience }

// Verify checks raw and returns its claims. Every refusal wraps
// ErrInvalidToken; the chain also carries the specific cause.
func (v *Verifier) Verify(ctx context.Context, raw string) (Claims, error) {
	claims, err := v.verify(ctx, raw)
	issuer := claims.Issuer
	if issuer == "" {
		issuer = "unknown"
	}
	if err != nil {
		v.observe(issuer, "rejected")

		return Claims{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	v.observe(issuer, "ok")

	return claims, nil
}

// verify is Verify without the wrapping and the observation. The returned
// Claims carry the issuer even on failure, for the metric's label.
func (v *Verifier) verify(ctx context.Context, raw string) (Claims, error) {
	tok, err := jwt.ParseSigned(raw, signatureAlgorithms)
	if err != nil {
		return Claims{}, fmt.Errorf("parse: %w", err)
	}

	// The issuer is read unverified to pick the key set; the signature is
	// checked against that set before any claim is trusted.
	var peek jwt.Claims
	if err := tok.UnsafeClaimsWithoutVerification(&peek); err != nil {
		return Claims{}, fmt.Errorf("read claims: %w", err)
	}
	issuer := strings.TrimSuffix(peek.Issuer, "/")
	partial := Claims{Issuer: issuer}

	kid := ""
	if len(tok.Headers) > 0 {
		kid = tok.Headers[0].KeyID
	}

	key, err := v.keyFor(ctx, issuer, kid)
	if err != nil {
		return partial, err
	}

	var (
		std    jwt.Claims
		custom claimSet
	)
	if err := tok.Claims(key, &std, &custom); err != nil {
		return partial, fmt.Errorf("signature: %w", err)
	}

	if err := v.checkRegistered(&std); err != nil {
		return partial, err
	}

	return Claims{
		Issuer:   issuer,
		Subject:  std.Subject,
		ClientID: custom.ClientID,
		Scopes:   custom.scopes(),
		Email:    custom.Email,
		Login:    custom.Login,
		Expiry:   std.Expiry.Time(),
		ID:       std.ID,
	}, nil
}

// keyFor resolves the verification key for issuer and kid.
func (v *Verifier) keyFor(ctx context.Context, issuer, kid string) (any, error) {
	if v.own != nil && issuer == v.own.issuer {
		if kid != "" && kid != v.own.kid {
			return nil, fmt.Errorf("%w: kid %q", ErrUnknownKey, kid)
		}

		return v.own.key, nil
	}

	t, ok := v.trusted[issuer]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownIssuer, issuer)
	}

	return t.key(ctx, kid)
}

// checkRegistered validates aud, exp, and nbf. The issuer was matched by
// keyFor, and a token with no exp is refused: every issuer docz-api trusts
// sets one, and an eternal token is not something to accept by omission.
func (v *Verifier) checkRegistered(std *jwt.Claims) error {
	if !std.Audience.Contains(v.audience) {
		return fmt.Errorf("%w: aud %v", ErrWrongAudience, []string(std.Audience))
	}

	if std.Expiry == nil {
		return fmt.Errorf("%w: no exp", ErrTokenExpired)
	}

	err := std.ValidateWithLeeway(jwt.Expected{Time: time.Now()}, clockSkew)
	switch {
	case errors.Is(err, jwt.ErrExpired):
		return ErrTokenExpired
	case errors.Is(err, jwt.ErrNotValidYet):
		return ErrTokenNotYetValid
	case err != nil:
		return fmt.Errorf("claims: %w", err)
	}

	return nil
}

// key returns the issuer's key for kid, refreshing the set on an unknown
// kid at most once per jwksRefreshInterval.
func (t *trustedIssuer) key(ctx context.Context, kid string) (any, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.keys == nil {
		if err := t.load(ctx); err != nil {
			return nil, err
		}
	}

	if k, ok := t.find(kid); ok {
		return k, nil
	}

	if t.now().Sub(t.lastRefresh) >= jwksRefreshInterval {
		t.lastRefresh = t.now()
		if err := t.refresh(ctx); err != nil {
			return nil, err
		}
		if k, ok := t.find(kid); ok {
			return k, nil
		}
	}

	return nil, fmt.Errorf("%w: kid %q at %s", ErrUnknownKey, kid, t.issuer)
}

// find returns the key for kid, or the only key when the token names none.
func (t *trustedIssuer) find(kid string) (any, bool) {
	if kid == "" {
		if len(t.keys.Keys) == 1 {
			return t.keys.Keys[0].Key, true
		}

		return nil, false
	}

	for i := range t.keys.Keys {
		if t.keys.Keys[i].KeyID == kid {
			return t.keys.Keys[i].Key, true
		}
	}

	return nil, false
}

// load fills keys from the cache, or from the issuer when the cache has
// nothing.
func (t *trustedIssuer) load(ctx context.Context) error {
	if raw, err := t.cache.Get(ctx, t.cacheKey()); err == nil {
		var set jose.JSONWebKeySet
		if jerr := json.Unmarshal(raw, &set); jerr == nil && len(set.Keys) > 0 {
			t.keys = &set

			return nil
		}
	}

	return t.refresh(ctx)
}

// refresh fetches the key set from the issuer and caches it. Only the
// unknown-kid path in key stamps lastRefresh: the first load is not a
// refresh, so a rotation right after startup is still picked up.
func (t *trustedIssuer) refresh(ctx context.Context) error {
	raw, err := fetchDocument(ctx, t.client, t.jwksURI)
	if err != nil {
		return fmt.Errorf("fetch jwks for %s: %w", t.issuer, err)
	}

	var set jose.JSONWebKeySet
	if err := json.Unmarshal(raw, &set); err != nil {
		return fmt.Errorf("jwks for %s: %w", t.issuer, err)
	}
	if len(set.Keys) == 0 {
		return fmt.Errorf("jwks for %s has no keys", t.issuer)
	}
	t.keys = &set

	// A cache failure costs one fetch per replica per hour, not the request.
	if err := t.cache.Set(ctx, t.cacheKey(), raw, jwksTTL); err != nil {
		slog.DebugContext(ctx, "jwks cache write failed", "issuer", t.issuer, "err", err)
	}

	return nil
}

func (t *trustedIssuer) cacheKey() string {
	sum := sha256.Sum256([]byte(t.issuer))

	return "jwks:" + hex.EncodeToString(sum[:])
}

// discoverJWKS reads the issuer's OpenID discovery document for its jwks_uri.
func discoverJWKS(ctx context.Context, client *http.Client, issuer string) (string, error) {
	raw, err := fetchDocument(ctx, client, issuer+"/.well-known/openid-configuration")
	if err != nil {
		return "", fmt.Errorf("discovery: %w", err)
	}

	var doc struct {
		Issuer  string `json:"issuer"`
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", fmt.Errorf("discovery document: %w", err)
	}
	if strings.TrimSuffix(doc.Issuer, "/") != issuer {
		return "", fmt.Errorf("discovery document names issuer %q, not %q", doc.Issuer, issuer)
	}
	if doc.JWKSURI == "" {
		return "", errors.New("discovery document has no jwks_uri")
	}

	return doc.JWKSURI, nil
}

// fetchDocument GETs a JSON document, capped at maxDocument.
func fetchDocument(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDocument))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}

	return body, nil
}

// joinScopes is the space-separated scope claim.
func joinScopes(scopes []string) string { return strings.Join(scopes, " ") }

// randomID is 16 random bytes, base64url, for a jti.
func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random id: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}
