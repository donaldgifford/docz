package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrInvalidConfig is returned by Load when the environment fails validation.
// Callers branch on it with errors.Is; the wrapped message lists every problem.
var ErrInvalidConfig = errors.New("invalid config")

// Recognized enum values for validated fields. Unexported globals carry the
// `_` prefix (Uber style) so their package scope is visible at every use.
var (
	_validProviders  = []string{"github", "okta", "keycloak"}
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
	requireNonEmpty(&errs, "SESSION_SECRET", c.Session.Secret.Reveal())

	validateAuth(&errs, &c.Auth)
	validateEnum(&errs, "LOG_LEVEL", c.Log.Level, _validLogLevels)
	validateEnum(&errs, "LOG_FORMAT", c.Log.Format, _validLogFormats)

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%w:\n  %s", ErrInvalidConfig, strings.Join(errs, "\n  "))
}

// validateAuth checks the provider list and each enabled provider's credentials.
func validateAuth(errs *[]string, a *AuthConfig) {
	if len(a.Providers) == 0 {
		*errs = append(*errs, "AUTH_PROVIDERS: at least one provider is required")
		return
	}
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
