package config

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

// schemeHTTPS is the one scheme an issuer or a Confluence site may use.
const schemeHTTPS = "https"

// ErrInvalidConfig is returned by Load when the environment fails validation.
// Callers branch on it with errors.Is; the wrapped message lists every problem.
var ErrInvalidConfig = errors.New("invalid config")

// Recognized enum values for validated fields. Unexported globals carry the
// `_` prefix (Uber style) so their package scope is visible at every use.
var (
	// ProviderNone is listed for the operator-facing error message only: the
	// none case returns before the provider loop, so it never reaches the
	// unknown-provider branch.
	_validProviders  = []string{"github", "okta", "keycloak", ProviderNone}
	_validLogLevels  = []string{"debug", "info", "warn", "error"}
	_validLogFormats = []string{"text", "json"}
)

// validate collects every configuration problem and returns them as a single
// ErrInvalidConfig, so an operator can fix all of them in one pass rather than
// discovering them one restart at a time.
func validate(c *Config) error {
	var errs []string

	requireNonEmpty(&errs, "DATABASE_URL", c.Store.DatabaseURL)
	requireNonEmpty(&errs, "REDIS_URL", c.Store.RedisURL)
	requireNonEmpty(&errs, "MEILI_HOST", c.Meili.Host)
	requireNonEmpty(&errs, "MEILI_API_KEY", c.Meili.APIKey.Reveal())
	requireNonZero(&errs, "GITHUB_APP_ID", c.GitHub.AppID)
	requireNonEmpty(&errs, "GITHUB_APP_PRIVATE_KEY", c.GitHub.PrivateKey.Reveal())
	requireNonEmpty(&errs, "GITHUB_WEBHOOK_SECRET", c.GitHub.WebhookSecret.Reveal())
	// The session secret signs OAuth state and keys the session store; under
	// none-mode neither exists, so demanding it would defeat the point of a
	// first-setup shape that needs no login configuration at all.
	if !c.AuthDisabled() {
		requireNonEmpty(&errs, "SESSION_SECRET", c.Session.Secret.Reveal())
	}

	validateAuth(&errs, &c.Auth)
	validateConfluence(&errs, &c.Confluence)
	validateMCP(&errs, c)
	validateEnum(&errs, "LOG_LEVEL", c.Log.Level, _validLogLevels)
	validateEnum(&errs, "LOG_FORMAT", c.Log.Format, _validLogFormats)

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%w:\n  %s", ErrInvalidConfig, strings.Join(errs, "\n  "))
}

// validateConfluence checks the export's settings, only when the token
// turns it on: an https site with a host and no path, an email, and at
// least one space.
func validateConfluence(errs *[]string, c *ConfluenceConfig) {
	if !c.Enabled() {
		return
	}

	if u, err := url.Parse(c.Site); c.Site == "" || err != nil || u.Scheme != schemeHTTPS || u.Host == "" ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.User != nil {
		*errs = append(*errs, "CONFLUENCE_SITE: an https URL with a host and no path is required when CONFLUENCE_API_TOKEN is set")
	}

	if c.Email == "" {
		*errs = append(*errs, "CONFLUENCE_EMAIL: required when CONFLUENCE_API_TOKEN is set")
	}

	if len(c.Spaces) == 0 {
		*errs = append(*errs, "CONFLUENCE_SPACES: at least one space key is required when CONFLUENCE_API_TOKEN is set")
	}
}

// validateAuth checks the provider list and each enabled provider's credentials.
func validateAuth(errs *[]string, a *AuthConfig) {
	if len(a.Providers) == 0 {
		*errs = append(*errs, "AUTH_PROVIDERS: at least one provider is required")
		return
	}
	if slices.Contains(a.Providers, ProviderNone) {
		// "none" disables login entirely, so combining it with a real provider
		// describes two incompatible shapes at once. Rejecting the combination
		// keeps the mode unambiguous: either the read API is open, or it is not.
		if len(a.Providers) > 1 {
			*errs = append(*errs, fmt.Sprintf(
				"AUTH_PROVIDERS: %q must be the only entry when present", ProviderNone))
		}
		return
	}
	// Every enabled provider builds its redirect URL from this base, so it is
	// required whenever any provider is enabled (always, given the check above).
	requireNonEmpty(errs, "AUTH_REDIRECT_BASE", a.RedirectBase)
	for _, p := range a.Providers {
		switch p {
		case "github":
			requireNonEmpty(errs, "GITHUB_OAUTH_CLIENT_ID", a.GitHub.ClientID)
			requireNonEmpty(errs, "GITHUB_OAUTH_CLIENT_SECRET", a.GitHub.ClientSecret.Reveal())
		case "okta":
			requireOIDC(errs, "OKTA", a.Okta)
		case "keycloak":
			requireOIDC(errs, "KEYCLOAK", a.Keycloak)
		default:
			*errs = append(*errs, fmt.Sprintf(
				"AUTH_PROVIDERS: unknown provider %q (valid: %s)",
				p, strings.Join(_validProviders, ", ")))
		}
	}
}

// requireOIDC checks the issuer/client-id/client-secret trio for an OIDC
// provider, prefixing each env-var name with the provider (OKTA_/KEYCLOAK_).
func requireOIDC(errs *[]string, prefix string, p OIDCProvider) {
	requireNonEmpty(errs, prefix+"_ISSUER", p.Issuer)
	requireNonEmpty(errs, prefix+"_CLIENT_ID", p.ClientID)
	requireNonEmpty(errs, prefix+"_CLIENT_SECRET", p.ClientSecret.Reveal())
}

// requireNonEmpty records a missing-required error for an empty string.
func requireNonEmpty(errs *[]string, name, value string) {
	if value == "" {
		*errs = append(*errs, name+" is required")
	}
}

// requireNonZero records a missing-required error for a zero int64.
func requireNonZero(errs *[]string, name string, value int64) {
	if value == 0 {
		*errs = append(*errs, name+" is required")
	}
}

// validateEnum records an error when value is not one of allowed.
func validateEnum(errs *[]string, name, value string, allowed []string) {
	if !slices.Contains(allowed, value) {
		*errs = append(*errs, fmt.Sprintf(
			"%s: invalid value %q (valid: %s)", name, value, strings.Join(allowed, ", ")))
	}
}

// splitTrimmed splits a comma-separated list, trims surrounding whitespace from
// each element, and drops empties. It returns nil for an empty or all-blank
// input.
func splitTrimmed(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// validateMCP checks the MCP, OAuth, and authorization settings. The scope
// map, the rate limit, the TTLs, and a signing key that is set are judged
// whenever present; the issuer, the key, and the site URL are required only
// while MCP is on, and the key and the site only with a login to consent
// through (DESIGN-0023 question 4).
func validateMCP(errs *[]string, c *Config) {
	validateMCPBounds(errs, c)
	validateOAuth(errs, &c.OAuth)

	if !c.MCP.Enabled {
		return
	}

	if err := validateIssuerURL(c.OAuth.Issuer, false); err != nil {
		*errs = append(*errs, fmt.Sprintf("OAUTH_ISSUER: %v when MCP_ENABLED is set (defaults to AUTH_REDIRECT_BASE)", err))
	}

	if c.AuthDisabled() {
		return
	}

	if c.OAuth.SigningKeyPEM.Reveal() == "" {
		*errs = append(*errs, "OAUTH_SIGNING_KEY: required when MCP_ENABLED is set with a login provider")
	}

	if c.OAuth.SiteURL == "" {
		*errs = append(*errs, "DOCZ_SITE_URL: required when MCP_ENABLED is set with a login provider")
	}
}

// validateMCPBounds checks the numbers: the rate limit and the three TTLs.
func validateMCPBounds(errs *[]string, c *Config) {
	if c.MCP.RateLimit <= 0 {
		*errs = append(*errs, "MCP_RATE_LIMIT: must be a positive number of calls per minute")
	}

	if c.OAuth.AccessTTL <= 0 {
		*errs = append(*errs, "OAUTH_ACCESS_TTL: must be a positive duration")
	}

	if c.OAuth.RefreshTTL <= 0 {
		*errs = append(*errs, "OAUTH_REFRESH_TTL: must be a positive duration")
	}

	if c.Authz.CacheTTL <= 0 {
		*errs = append(*errs, "AUTHZ_CACHE_TTL: must be a positive duration")
	}
}

// validateOAuth checks the values that are judged whenever set: each trusted
// issuer, a signing key, and the site URL.
func validateOAuth(errs *[]string, c *OAuthConfig) {
	for _, issuer := range c.TrustedIssuers {
		if err := validateIssuerURL(issuer, true); err != nil {
			*errs = append(*errs, fmt.Sprintf("OAUTH_TRUSTED_ISSUERS: %q %v", issuer, err))
		}
	}

	if pemBody := c.SigningKeyPEM.Reveal(); pemBody != "" {
		if _, err := ParseSigningKey(pemBody); err != nil {
			*errs = append(*errs, "OAUTH_SIGNING_KEY: "+strings.TrimPrefix(err.Error(), ErrInvalidSigningKey.Error()+": "))
		}
	}

	if c.SiteURL != "" {
		if u, err := url.Parse(c.SiteURL); err != nil || (u.Scheme != schemeHTTPS && u.Scheme != "http") || u.Host == "" {
			*errs = append(*errs, "DOCZ_SITE_URL: must be an http or https URL with a host")
		}
	}
}

// sortStrings sorts in place; a local name so mcp.go needs no extra import.
func sortStrings(s []string) { slices.Sort(s) }
