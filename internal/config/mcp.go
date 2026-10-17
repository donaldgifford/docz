package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"

	doczcfg "github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// Defaults for the MCP server, the authorization server, and scoped access
// (DESIGN-0023 API changes).
const (
	DefaultMCPRateLimit    = 120
	DefaultOAuthAccessTTL  = 15 * time.Minute
	DefaultOAuthRefreshTTL = 720 * time.Hour
	DefaultAuthzCacheTTL   = 30 * time.Second
)

// MCPConfig turns the MCP server on and bounds it (DESIGN-0023 §2, §8).
type MCPConfig struct {
	Enabled   bool // MCP_ENABLED — default false; mounts /mcp, the token verifier, and the authorization server.
	RateLimit int  // MCP_RATE_LIMIT — tool calls per minute per principal, per replica; default 120.
}

// OAuthConfig is docz-api's own issuer and the issuers it trusts
// (DESIGN-0023 §5, §6). The signing key is an ES256 private key, held as
// its PEM body so it redacts on every logging path; SigningKey parses it.
type OAuthConfig struct {
	Issuer         string        // OAUTH_ISSUER — the public base URL; default AUTH_REDIRECT_BASE.
	SigningKeyPEM  Secret        // OAUTH_SIGNING_KEY — PEM body or a path to one; required with MCP on and a login.
	TrustedIssuers []string      // OAUTH_TRUSTED_ISSUERS — comma-separated issuer URLs whose JWTs are accepted.
	AccessTTL      time.Duration // OAUTH_ACCESS_TTL — default 15m.
	RefreshTTL     time.Duration // OAUTH_REFRESH_TTL — default 720h, from last use.
	SiteURL        string        // DOCZ_SITE_URL — where /oauth/authorize sends a person for consent.
}

// SigningKey parses the configured PEM into the ES256 private key. Load has
// already validated it, so an error here means the Config was built by hand.
func (o *OAuthConfig) SigningKey() (*ecdsa.PrivateKey, error) {
	return ParseSigningKey(o.SigningKeyPEM.Reveal())
}

// Audience is the aud claim every accepted token must carry: the MCP
// endpoint under the issuer (DESIGN-0023 §6).
func (o *OAuthConfig) Audience() string {
	return o.Issuer + "/mcp"
}

// AuthzConfig maps identities to scopes (DESIGN-0023 §7).
type AuthzConfig struct {
	// ScopeMap is AUTHZ_SCOPE_MAP parsed: a `<provider>:group:<name>` or
	// `<provider>:login:<name>` key to the scopes it grants, on top of the
	// public:read every principal holds.
	ScopeMap map[string][]string
	CacheTTL time.Duration // AUTHZ_CACHE_TTL — how long a principal's allowed set is held in process; default 30s.
}

// Scopes returns every scope the map can grant, sorted and de-duplicated,
// for the metadata document's scopes_supported.
func (a *AuthzConfig) Scopes() []string {
	set := map[string]struct{}{}
	for _, scopes := range a.ScopeMap {
		for _, s := range scopes {
			set[s] = struct{}{}
		}
	}

	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sortStrings(out)

	return out
}

// ErrInvalidScopeMap is wrapped by every AUTHZ_SCOPE_MAP parse failure.
var ErrInvalidScopeMap = errors.New("invalid AUTHZ_SCOPE_MAP")

// ParseScopeMap parses AUTHZ_SCOPE_MAP: entries separated by `;`, each
// `<provider>:group:<name>=<scope>[,<scope>]` or `<provider>:login:<name>=…`.
// Every scope must satisfy the library's grammar; a public audience is
// rejected, since every principal holds public:read already, and so is any
// write action, confluence:write included, because that scope is the
// export's alone (DESIGN-0023 §7). An empty value is an empty map.
func ParseScopeMap(raw string) (map[string][]string, error) {
	out := map[string][]string{}

	for entry := range strings.SplitSeq(raw, ";") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		key, scopesRaw, ok := strings.Cut(entry, "=")
		if !ok {
			return nil, fmt.Errorf("%w: entry %q is not <key>=<scopes>", ErrInvalidScopeMap, entry)
		}

		key = strings.TrimSpace(key)
		if err := validateScopeMapKey(key); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidScopeMap, err)
		}

		scopes := splitTrimmed(scopesRaw)
		if len(scopes) == 0 {
			return nil, fmt.Errorf("%w: entry %q grants no scope", ErrInvalidScopeMap, entry)
		}

		for _, s := range scopes {
			if err := doczcfg.ValidateScope(s); err != nil {
				return nil, fmt.Errorf("%w: entry %q: %w", ErrInvalidScopeMap, entry, err)
			}

			if strings.HasPrefix(s, doczcfg.AudiencePublic+":") {
				return nil, fmt.Errorf("%w: entry %q grants %q, which every principal holds", ErrInvalidScopeMap, entry, s)
			}

			if strings.HasSuffix(s, ":"+doczcfg.ActionWrite) {
				return nil, fmt.Errorf("%w: entry %q grants %q; a mapping cannot grant a write action", ErrInvalidScopeMap, entry, s)
			}
		}

		out[key] = append(out[key], scopes...)
	}

	return out, nil
}

// validateScopeMapKey checks `<provider>:group:<name>` or
// `<provider>:login:<name>`, each part non-empty.
func validateScopeMapKey(key string) error {
	parts := strings.SplitN(key, ":", 3)
	if len(parts) != 3 || parts[0] == "" || parts[2] == "" {
		return fmt.Errorf("key %q is not <provider>:group:<name> or <provider>:login:<name>", key)
	}

	if parts[1] != "group" && parts[1] != "login" {
		return fmt.Errorf("key %q: %q must be group or login", key, parts[1])
	}

	return nil
}

// ErrInvalidSigningKey is wrapped by every OAUTH_SIGNING_KEY parse failure.
var ErrInvalidSigningKey = errors.New("invalid OAUTH_SIGNING_KEY")

// ParseSigningKey parses a PEM-encoded EC P-256 private key, in either the
// SEC 1 ("EC PRIVATE KEY") or PKCS #8 ("PRIVATE KEY") encoding, which is
// what `openssl ecparam -name prime256v1 -genkey -noout` and
// `openssl genpkey -algorithm EC` write respectively.
func ParseSigningKey(pemBody string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemBody))
	if block == nil {
		return nil, fmt.Errorf("%w: not PEM", ErrInvalidSigningKey)
	}

	var (
		key any
		err error
	)

	switch block.Type {
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	default:
		return nil, fmt.Errorf("%w: PEM block %q is not a private key", ErrInvalidSigningKey, block.Type)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidSigningKey, err)
	}

	ec, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%w: not an EC key", ErrInvalidSigningKey)
	}

	if ec.Curve != elliptic.P256() {
		return nil, fmt.Errorf("%w: curve %s is not P-256", ErrInvalidSigningKey, ec.Curve.Params().Name)
	}

	return ec, nil
}

// validateIssuerURL requires https with a host and no user, query, or
// fragment. A path is allowed only when allowPath: Keycloak's issuers carry
// one, docz-api's own is a bare origin.
func validateIssuerURL(raw string, allowPath bool) error {
	u, err := url.Parse(raw)
	if err != nil || raw == "" {
		return errors.New("not a URL")
	}

	if u.Scheme != schemeHTTPS || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("must be https://<host> with no query or fragment")
	}

	if !allowPath && u.Path != "" && u.Path != "/" {
		return errors.New("must have no path")
	}

	return nil
}

// loadMCP reads the MCP, OAuth, and Authz sections (DESIGN-0023); Load
// defaults an empty issuer to AUTH_REDIRECT_BASE afterwards.
func loadMCP(v *viper.Viper, signingKey string, scopeMap map[string][]string) (MCPConfig, OAuthConfig, AuthzConfig) {
	mcp := MCPConfig{
		Enabled:   v.GetBool("mcp_enabled"),
		RateLimit: v.GetInt("mcp_rate_limit"),
	}
	oauth := OAuthConfig{
		Issuer:         strings.TrimSuffix(strings.TrimSpace(v.GetString("oauth_issuer")), "/"),
		SigningKeyPEM:  Secret(signingKey),
		TrustedIssuers: trimSlashes(splitTrimmed(v.GetString("oauth_trusted_issuers"))),
		AccessTTL:      v.GetDuration("oauth_access_ttl"),
		RefreshTTL:     v.GetDuration("oauth_refresh_ttl"),
		SiteURL:        strings.TrimSuffix(strings.TrimSpace(v.GetString("docz_site_url")), "/"),
	}
	authz := AuthzConfig{
		ScopeMap: scopeMap,
		CacheTTL: v.GetDuration("authz_cache_ttl"),
	}

	return mcp, oauth, authz
}
