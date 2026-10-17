package config_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donaldgifford/docz/v2/internal/config"
)

// testSigningKey is a fresh P-256 key in the SEC 1 encoding, what
// `openssl ecparam -name prime256v1 -genkey -noout` writes.
func testSigningKey(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))
}

// mcpEnv is a valid, enabled MCP configuration each case breaks once.
func mcpEnv(t *testing.T) map[string]string {
	t.Helper()
	env := validEnv()
	env["MCP_ENABLED"] = "true"
	env["OAUTH_SIGNING_KEY"] = testSigningKey(t)
	env["DOCZ_SITE_URL"] = "https://docz.example/"
	env["OAUTH_TRUSTED_ISSUERS"] = "https://keycloak.example/realms/docz/, https://acme.okta.com/oauth2/default"
	env["AUTHZ_SCOPE_MAP"] = "github:login:donald=platform-team:read; keycloak:group:platform=platform-team:read,docs-team:read"
	return env
}

func TestLoadMCPDisabledDefaults(t *testing.T) {
	cfg, err := load(t, validEnv())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.MCP.Enabled {
		t.Error("MCP.Enabled = true by default, want false")
	}
	if cfg.MCP.RateLimit != config.DefaultMCPRateLimit {
		t.Errorf("MCP.RateLimit = %d, want %d", cfg.MCP.RateLimit, config.DefaultMCPRateLimit)
	}
	if cfg.OAuth.AccessTTL != 15*time.Minute || cfg.OAuth.RefreshTTL != 720*time.Hour {
		t.Errorf("OAuth TTLs = %v/%v, want 15m/720h", cfg.OAuth.AccessTTL, cfg.OAuth.RefreshTTL)
	}
	if cfg.Authz.CacheTTL != 30*time.Second {
		t.Errorf("Authz.CacheTTL = %v, want 30s", cfg.Authz.CacheTTL)
	}
	if cfg.OAuth.Issuer != cfg.Auth.RedirectBase {
		t.Errorf("OAuth.Issuer = %q, want AUTH_REDIRECT_BASE %q", cfg.OAuth.Issuer, cfg.Auth.RedirectBase)
	}
	if cfg.OAuth.Audience() != cfg.Auth.RedirectBase+"/mcp" {
		t.Errorf("Audience() = %q, want <issuer>/mcp", cfg.OAuth.Audience())
	}
	if len(cfg.Authz.ScopeMap) != 0 || len(cfg.OAuth.TrustedIssuers) != 0 {
		t.Errorf("scope map %v and trusted issuers %v, want both empty", cfg.Authz.ScopeMap, cfg.OAuth.TrustedIssuers)
	}
}

func TestLoadMCPEnabled(t *testing.T) {
	cfg, err := load(t, mcpEnv(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !cfg.MCP.Enabled {
		t.Error("MCP.Enabled = false, want true")
	}
	if cfg.OAuth.SiteURL != "https://docz.example" {
		t.Errorf("SiteURL = %q, want the trailing slash trimmed", cfg.OAuth.SiteURL)
	}
	want := []string{"https://keycloak.example/realms/docz", "https://acme.okta.com/oauth2/default"}
	if strings.Join(cfg.OAuth.TrustedIssuers, " ") != strings.Join(want, " ") {
		t.Errorf("TrustedIssuers = %q, want %q", cfg.OAuth.TrustedIssuers, want)
	}
	if _, err := cfg.OAuth.SigningKey(); err != nil {
		t.Errorf("SigningKey() = %v, want the parsed key", err)
	}
	if got := cfg.Authz.ScopeMap["keycloak:group:platform"]; strings.Join(got, " ") != "platform-team:read docs-team:read" {
		t.Errorf("ScopeMap[keycloak:group:platform] = %q", got)
	}
	if got := cfg.Authz.ScopeMap["github:login:donald"]; strings.Join(got, " ") != "platform-team:read" {
		t.Errorf("ScopeMap[github:login:donald] = %q", got)
	}
	if got := cfg.Authz.Scopes(); strings.Join(got, " ") != "docs-team:read platform-team:read" {
		t.Errorf("Authz.Scopes() = %q, want sorted and de-duplicated", got)
	}
}

func TestLoadMCPSigningKeyFromPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(path, []byte(testSigningKey(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	env := mcpEnv(t)
	env["OAUTH_SIGNING_KEY"] = path

	cfg, err := load(t, env)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := cfg.OAuth.SigningKey(); err != nil {
		t.Errorf("SigningKey() from a path = %v", err)
	}
}

func TestLoadMCPNoneModeNeedsNoKeyOrSite(t *testing.T) {
	env := validEnv()
	env["AUTH_PROVIDERS"] = "none"
	env["AUTH_REDIRECT_BASE"] = ""
	env["MCP_ENABLED"] = "true"
	env["OAUTH_ISSUER"] = "https://docz-api.example"

	if _, err := load(t, env); err != nil {
		t.Fatalf("Load under none with MCP on: %v", err)
	}
}

func TestLoadMCPMissingKeyAndSiteNamesBoth(t *testing.T) {
	env := validEnv()
	env["MCP_ENABLED"] = "true"

	_, err := load(t, env)
	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("Load = %v, want ErrInvalidConfig", err)
	}
	for _, want := range []string{"OAUTH_SIGNING_KEY: required", "DOCZ_SITE_URL: required"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

func TestLoadMCPInvalid(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rsaPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rsaKey)}))
	p384, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	p384DER, err := x509.MarshalECPrivateKey(p384)
	if err != nil {
		t.Fatal(err)
	}
	p384PEM := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: p384DER}))

	tests := []struct {
		name string
		set  map[string]string
		want string
	}{
		{name: "http issuer", set: map[string]string{"OAUTH_ISSUER": "http://docz-api.example"}, want: "OAUTH_ISSUER: must be https"},
		{name: "issuer with a path", set: map[string]string{"OAUTH_ISSUER": "https://docz-api.example/api"}, want: "OAUTH_ISSUER: must have no path"},
		{name: "issuer with a query", set: map[string]string{"OAUTH_ISSUER": "https://docz-api.example/?x=1"}, want: "OAUTH_ISSUER: must be https"},
		{
			name: "http trusted issuer",
			set:  map[string]string{"OAUTH_TRUSTED_ISSUERS": "http://keycloak.example/realms/docz"},
			want: `OAUTH_TRUSTED_ISSUERS: "http://keycloak.example/realms/docz" must be https`,
		},
		{name: "not PEM", set: map[string]string{"OAUTH_SIGNING_KEY": "-----BEGIN nothing"}, want: "OAUTH_SIGNING_KEY: not PEM"},
		{
			name: "an RSA key",
			set:  map[string]string{"OAUTH_SIGNING_KEY": rsaPEM},
			want: `OAUTH_SIGNING_KEY: PEM block "RSA PRIVATE KEY" is not a private key`,
		},
		{name: "a P-384 key", set: map[string]string{"OAUTH_SIGNING_KEY": p384PEM}, want: "OAUTH_SIGNING_KEY: curve P-384 is not P-256"},
		{name: "a bare site url", set: map[string]string{"DOCZ_SITE_URL": "docz.example"}, want: "DOCZ_SITE_URL: must be an http or https URL"},
		{name: "zero rate limit", set: map[string]string{"MCP_RATE_LIMIT": "0"}, want: "MCP_RATE_LIMIT: must be a positive"},
		{name: "zero access ttl", set: map[string]string{"OAUTH_ACCESS_TTL": "0s"}, want: "OAUTH_ACCESS_TTL: must be a positive"},
		{name: "negative refresh ttl", set: map[string]string{"OAUTH_REFRESH_TTL": "-1h"}, want: "OAUTH_REFRESH_TTL: must be a positive"},
		{name: "zero cache ttl", set: map[string]string{"AUTHZ_CACHE_TTL": "0"}, want: "AUTHZ_CACHE_TTL: must be a positive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := mcpEnv(t)
			for k, v := range tt.set {
				env[k] = v
			}

			_, err := load(t, env)
			if !errors.Is(err, config.ErrInvalidConfig) {
				t.Fatalf("Load = %v, want ErrInvalidConfig", err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not contain %q", err, tt.want)
			}
		})
	}
}

func TestLoadMCPSigningKeyIsRedacted(t *testing.T) {
	cfg, err := load(t, mcpEnv(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	var logged strings.Builder
	slog.New(slog.NewTextHandler(&logged, nil)).Info("config", "oauth", cfg.OAuth)
	if strings.Contains(logged.String(), "BEGIN EC PRIVATE KEY") {
		t.Errorf("the signing key leaked into a log line: %s", logged.String())
	}
}

func TestParseScopeMap(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "empty", raw: ""},
		{name: "blank entries are skipped", raw: " ; github:login:a=t:read ;; "},
		{name: "no equals", raw: "github:login:a", wantErr: `entry "github:login:a" is not <key>=<scopes>`},
		{name: "two-part key", raw: "github:a=t:read", wantErr: "is not <provider>:group:<name> or <provider>:login:<name>"},
		{name: "unknown kind", raw: "github:team:a=t:read", wantErr: `"team" must be group or login`},
		{name: "empty provider", raw: ":group:a=t:read", wantErr: "is not <provider>:group:<name>"},
		{name: "empty name", raw: "github:group:=t:read", wantErr: "is not <provider>:group:<name>"},
		{name: "no scopes", raw: "github:group:a=", wantErr: "grants no scope"},
		{name: "a public scope", raw: "github:group:a=public:read", wantErr: `grants "public:read", which every principal holds`},
		{name: "a write scope", raw: "github:group:a=impl:write", wantErr: `action "write" is reserved`},
		{name: "the export's scope", raw: "github:group:a=confluence:write", wantErr: "a mapping cannot grant a write action"},
		{name: "a bad audience", raw: "github:group:a=My_Team:read", wantErr: "must match"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := config.ParseScopeMap(tt.raw)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ParseScopeMap(%q) = %v, want nil", tt.raw, err)
				}
				return
			}
			if !errors.Is(err, config.ErrInvalidScopeMap) || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ParseScopeMap(%q) = %v, want ErrInvalidScopeMap containing %q", tt.raw, err, tt.wantErr)
			}
		})
	}
}

func TestParseScopeMapMergesRepeatedKeys(t *testing.T) {
	got, err := config.ParseScopeMap("github:group:a=x:read;github:group:a=y:read")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got["github:group:a"], " ") != "x:read y:read" {
		t.Errorf("repeated key = %q, want both scopes", got["github:group:a"])
	}
}

func TestLoadScopeMapFailureIsInvalidConfig(t *testing.T) {
	env := validEnv()
	env["AUTHZ_SCOPE_MAP"] = "nonsense"

	_, err := load(t, env)
	if !errors.Is(err, config.ErrInvalidConfig) || !errors.Is(err, config.ErrInvalidScopeMap) {
		t.Fatalf("Load = %v, want ErrInvalidConfig wrapping ErrInvalidScopeMap", err)
	}
}

func TestParseSigningKeyPKCS8(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pemBody := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))

	parsed, err := config.ParseSigningKey(pemBody)
	if err != nil {
		t.Fatalf("ParseSigningKey(PKCS8) = %v", err)
	}
	if !parsed.Equal(key) {
		t.Error("ParseSigningKey(PKCS8) returned a different key")
	}
}
