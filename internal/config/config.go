// Package config loads and validates the docz-api service configuration from
// the environment.
//
// The whole configuration is read in one Load call using spf13/viper (already
// in the module graph via the docz parsing library, so no new top-level
// dependency — DESIGN-0001 OQ 5). Load returns a fully validated, typed Config
// or an actionable error listing every missing/invalid required variable at
// once. It never logs — the logger is configured in main() from the returned
// Config.Log, so Load must run first and communicate failures via error.
//
// Secrets are held in the Secret type, which redacts itself on every logging
// and formatting path (see secret.go).
package config

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the complete service configuration loaded from the environment.
// Sub-structs group related settings so main() can hand each downstream
// package (store, githubapp, auth, ...) only the section it needs.
type Config struct {
	Store     StoreConfig
	GitHub    GitHubConfig
	Meili     MeiliConfig
	Auth      AuthConfig
	Session   SessionConfig
	Ingest    IngestConfig
	HTTP      HTTPConfig
	Log       LogConfig
	Telemetry TelemetryConfig
}

// StoreConfig holds connection strings for the durable stores.
type StoreConfig struct {
	DatabaseURL string // DATABASE_URL — required Postgres DSN.
	RedisURL    string // REDIS_URL — required Redis DSN (queue + sessions).
}

// GitHubConfig holds GitHub App credentials used for ingestion. PrivateKey
// always holds the PEM body after Load resolves the path-or-literal input.
type GitHubConfig struct {
	AppID         int64  // GITHUB_APP_ID — required.
	PrivateKey    Secret // GITHUB_APP_PRIVATE_KEY — required; PEM body after resolution.
	WebhookSecret Secret // GITHUB_WEBHOOK_SECRET — required.
	APIBase       string // GITHUB_API_BASE — default https://api.github.com.
}

// MeiliConfig holds Meilisearch connection details.
type MeiliConfig struct {
	Host   string // MEILI_HOST — required.
	APIKey Secret // MEILI_API_KEY — required.
}

// AuthConfig holds site-user authentication configuration. Providers lists the
// enabled login providers; a provider's credentials are required only when it
// is enabled. RedirectBase is the public origin used to build each provider's
// OAuth/OIDC redirect URL (<base>/auth/callback), so it must match what the
// provider app is registered with.
type AuthConfig struct {
	Providers    []string // AUTH_PROVIDERS — default ["github"].
	RedirectBase string   // AUTH_REDIRECT_BASE — required; e.g. https://docz-api.internal.
	GitHub       GitHubOAuth
	Okta         OIDCProvider
	Keycloak     OIDCProvider
}

// GitHubOAuth holds the GitHub OAuth app credentials for site login. Required
// only when "github" is an enabled auth provider.
type GitHubOAuth struct {
	ClientID     string // GITHUB_OAUTH_CLIENT_ID.
	ClientSecret Secret // GITHUB_OAUTH_CLIENT_SECRET.
}

// OIDCProvider holds OIDC credentials for Okta or Keycloak. Required only when
// the matching provider is enabled.
type OIDCProvider struct {
	Issuer       string // {OKTA,KEYCLOAK}_ISSUER.
	ClientID     string // {OKTA,KEYCLOAK}_CLIENT_ID.
	ClientSecret Secret // {OKTA,KEYCLOAK}_CLIENT_SECRET.
}

// SessionConfig holds session store settings.
type SessionConfig struct {
	Secret Secret        // SESSION_SECRET — required.
	TTL    time.Duration // SESSION_TTL — default 720h.
}

// IngestConfig holds ingestion tuning.
type IngestConfig struct {
	Debounce time.Duration // INGEST_DEBOUNCE — default 5s.
}

// HTTPConfig holds HTTP server settings.
type HTTPConfig struct {
	Addr string // HTTP_ADDR — default ":8080".
}

// LogConfig holds logging settings, read before any logger is configured.
type LogConfig struct {
	Level  string // LOG_LEVEL — default "info" (debug/info/warn/error).
	Format string // LOG_FORMAT — default "text" (text|json).
}

// TelemetryConfig holds observability settings (OQ 8). Every field has safe
// zero-value behavior: an empty OTLPEndpoint disables trace export entirely
// (the tracer becomes a no-op), so a homelab install without a collector needs
// no telemetry configuration and pays no overhead.
type TelemetryConfig struct {
	ServiceName    string  // OTEL_SERVICE_NAME — default "docz-api".
	OTLPEndpoint   string  // OTEL_EXPORTER_OTLP_ENDPOINT — default ""; empty = tracing off.
	SampleRate     float64 // OTEL_SAMPLE_RATE — default 1.0 (head sampling ratio, 0–1).
	MetricsEnabled bool    // METRICS_ENABLED — default true; exposes /metrics.
}

// Load reads the service configuration from the environment and validates it.
// It returns a wrapped ErrInvalidConfig listing every problem when validation
// fails, so an operator sees all misconfigurations at once.
func Load() (Config, error) {
	v := viper.New()
	v.AutomaticEnv()

	v.SetDefault("github_api_base", "https://api.github.com")
	v.SetDefault("auth_providers", "github")
	v.SetDefault("session_ttl", "720h")
	v.SetDefault("ingest_debounce", "5s")
	v.SetDefault("http_addr", ":8080")
	v.SetDefault("log_level", "info")
	v.SetDefault("log_format", "text")
	v.SetDefault("otel_service_name", "docz-api")
	v.SetDefault("otel_sample_rate", 1.0)
	v.SetDefault("metrics_enabled", true)

	privateKey, err := resolvePrivateKey(v.GetString("github_app_private_key"))
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Store: StoreConfig{
			DatabaseURL: v.GetString("database_url"),
			RedisURL:    v.GetString("redis_url"),
		},
		GitHub: GitHubConfig{
			AppID:         v.GetInt64("github_app_id"),
			PrivateKey:    Secret(privateKey),
			WebhookSecret: Secret(v.GetString("github_webhook_secret")),
			APIBase:       v.GetString("github_api_base"),
		},
		Meili: MeiliConfig{
			Host:   v.GetString("meili_host"),
			APIKey: Secret(v.GetString("meili_api_key")),
		},
		Auth: AuthConfig{
			Providers:    splitTrimmed(v.GetString("auth_providers")),
			RedirectBase: strings.TrimSuffix(v.GetString("auth_redirect_base"), "/"),
			GitHub: GitHubOAuth{
				ClientID:     v.GetString("github_oauth_client_id"),
				ClientSecret: Secret(v.GetString("github_oauth_client_secret")),
			},
			Okta: OIDCProvider{
				Issuer:       v.GetString("okta_issuer"),
				ClientID:     v.GetString("okta_client_id"),
				ClientSecret: Secret(v.GetString("okta_client_secret")),
			},
			Keycloak: OIDCProvider{
				Issuer:       v.GetString("keycloak_issuer"),
				ClientID:     v.GetString("keycloak_client_id"),
				ClientSecret: Secret(v.GetString("keycloak_client_secret")),
			},
		},
		Session: SessionConfig{
			Secret: Secret(v.GetString("session_secret")),
			TTL:    v.GetDuration("session_ttl"),
		},
		Ingest: IngestConfig{
			Debounce: v.GetDuration("ingest_debounce"),
		},
		HTTP: HTTPConfig{
			Addr: v.GetString("http_addr"),
		},
		Log: LogConfig{
			Level:  v.GetString("log_level"),
			Format: v.GetString("log_format"),
		},
		Telemetry: TelemetryConfig{
			ServiceName:    v.GetString("otel_service_name"),
			OTLPEndpoint:   v.GetString("otel_exporter_otlp_endpoint"),
			SampleRate:     v.GetFloat64("otel_sample_rate"),
			MetricsEnabled: v.GetBool("metrics_enabled"),
		},
	}

	if err := validate(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// AuthEnabled reports whether provider is one of the enabled auth providers.
//
// The receiver is a pointer to avoid copying the ~350-byte Config on every
// call (gocritic hugeParam / Uber's large-struct receiver guidance); callers
// hold an addressable Config from Load, so cfg.AuthEnabled(...) works directly.
func (c *Config) AuthEnabled(provider string) bool {
	return slices.Contains(c.Auth.Providers, provider)
}

// resolvePrivateKey returns the PEM body for the GitHub App private key. A
// value beginning with a PEM header is treated as the body itself; anything
// else is treated as a filesystem path to read. An empty value is passed
// through — required-ness is enforced by validate.
func resolvePrivateKey(raw string) (string, error) {
	if raw == "" || strings.HasPrefix(raw, "-----BEGIN") {
		return raw, nil
	}
	// The path comes from a trusted operator-supplied env var, not user input.
	data, err := os.ReadFile(raw)
	if err != nil {
		return "", fmt.Errorf("reading GITHUB_APP_PRIVATE_KEY file %q: %w", raw, err)
	}
	return string(data), nil
}
