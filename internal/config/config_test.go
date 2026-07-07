package config_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donaldgifford/docz-api/internal/config"
)

const testPEM = "-----BEGIN RSA PRIVATE KEY-----\nMIIfakekeybody\n-----END RSA PRIVATE KEY-----\n"

// validEnv returns a complete, valid environment with the default github auth
// provider enabled. Tests copy it and override or clear individual keys.
func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":               "postgres://u:p@db:5432/docz?sslmode=disable",
		"REDIS_URL":                  "redis://redis:6379/0",
		"MEILI_HOST":                 "http://meili:7700",
		"MEILI_API_KEY":              "meili-key",
		"GITHUB_APP_ID":              "123456",
		"GITHUB_APP_PRIVATE_KEY":     testPEM,
		"GITHUB_WEBHOOK_SECRET":      "whsec",
		"SESSION_SECRET":             "sess",
		"AUTH_REDIRECT_BASE":         "https://docz-api.internal",
		"GITHUB_OAUTH_CLIENT_ID":     "gh-oauth-id",
		"GITHUB_OAUTH_CLIENT_SECRET": "gh-oauth-secret",
	}
}

// load applies env via t.Setenv (auto-restored) and calls config.Load. A key
// whose value is "" is set to empty, which Load treats as unset/missing.
func load(t *testing.T, env map[string]string) (config.Config, error) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	return config.Load()
}

func TestLoadValid(t *testing.T) {
	cfg, err := load(t, validEnv())
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}

	if cfg.Store.DatabaseURL == "" || cfg.Store.RedisURL == "" {
		t.Errorf("store URLs not populated: %+v", cfg.Store)
	}
	if cfg.GitHub.AppID != 123456 {
		t.Errorf("AppID = %d, want 123456", cfg.GitHub.AppID)
	}
	if got := cfg.GitHub.PrivateKey.Reveal(); got != testPEM {
		t.Errorf("PrivateKey = %q, want the PEM body", got)
	}
	if cfg.GitHub.APIBase != "https://api.github.com" {
		t.Errorf("APIBase = %q, want the default", cfg.GitHub.APIBase)
	}
	if want := []string{"github"}; len(cfg.Auth.Providers) != 1 || cfg.Auth.Providers[0] != want[0] {
		t.Errorf("Providers = %v, want %v", cfg.Auth.Providers, want)
	}
	if cfg.Session.TTL != 720*time.Hour {
		t.Errorf("Session.TTL = %v, want 720h default", cfg.Session.TTL)
	}
	if cfg.Ingest.Debounce != 5*time.Second {
		t.Errorf("Ingest.Debounce = %v, want 5s default", cfg.Ingest.Debounce)
	}
	if cfg.HTTP.Addr != ":8080" {
		t.Errorf("HTTP.Addr = %q, want :8080 default", cfg.HTTP.Addr)
	}
	if cfg.Log.Level != "info" || cfg.Log.Format != "text" {
		t.Errorf("Log = %+v, want info/text defaults", cfg.Log)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	required := []string{
		"DATABASE_URL", "REDIS_URL", "MEILI_HOST", "MEILI_API_KEY",
		"GITHUB_APP_ID", "GITHUB_APP_PRIVATE_KEY", "GITHUB_WEBHOOK_SECRET",
		"SESSION_SECRET", "AUTH_REDIRECT_BASE", "GITHUB_OAUTH_CLIENT_ID",
		"GITHUB_OAUTH_CLIENT_SECRET",
	}
	for _, name := range required {
		t.Run(name, func(t *testing.T) {
			env := validEnv()
			env[name] = "" // clear the one under test
			_, err := load(t, env)
			if !errors.Is(err, config.ErrInvalidConfig) {
				t.Fatalf("Load error = %v, want errors.Is ErrInvalidConfig", err)
			}
			if !strings.Contains(err.Error(), name) {
				t.Errorf("error %q does not mention %q", err, name)
			}
		})
	}
}

func TestLoadOverridesAndDurations(t *testing.T) {
	env := validEnv()
	env["GITHUB_API_BASE"] = "https://ghe.example.com/api/v3"
	env["SESSION_TTL"] = "48h"
	env["INGEST_DEBOUNCE"] = "500ms"
	env["HTTP_ADDR"] = "127.0.0.1:9000"
	env["LOG_LEVEL"] = "debug"
	env["LOG_FORMAT"] = "json"

	cfg, err := load(t, env)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if cfg.GitHub.APIBase != "https://ghe.example.com/api/v3" {
		t.Errorf("APIBase = %q", cfg.GitHub.APIBase)
	}
	if cfg.Session.TTL != 48*time.Hour {
		t.Errorf("Session.TTL = %v, want 48h", cfg.Session.TTL)
	}
	if cfg.Ingest.Debounce != 500*time.Millisecond {
		t.Errorf("Ingest.Debounce = %v, want 500ms", cfg.Ingest.Debounce)
	}
	if cfg.HTTP.Addr != "127.0.0.1:9000" || cfg.Log.Level != "debug" || cfg.Log.Format != "json" {
		t.Errorf("overrides not applied: %+v %+v", cfg.HTTP, cfg.Log)
	}
}

func TestLoadAuthProviderConditional(t *testing.T) {
	env := validEnv()
	env["AUTH_PROVIDERS"] = "github, okta" // okta enabled but creds absent

	_, err := load(t, env)
	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("Load error = %v, want ErrInvalidConfig", err)
	}
	for _, want := range []string{"OKTA_ISSUER", "OKTA_CLIENT_ID", "OKTA_CLIENT_SECRET"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestLoadAuthProviderEnabled(t *testing.T) {
	env := validEnv()
	env["AUTH_PROVIDERS"] = "github,keycloak"
	env["KEYCLOAK_ISSUER"] = "https://kc.example.com/realms/acme"
	env["KEYCLOAK_CLIENT_ID"] = "kc-id"
	env["KEYCLOAK_CLIENT_SECRET"] = "kc-secret"

	cfg, err := load(t, env)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if !cfg.AuthEnabled("keycloak") || !cfg.AuthEnabled("github") {
		t.Errorf("AuthEnabled: providers = %v", cfg.Auth.Providers)
	}
	if cfg.AuthEnabled("okta") {
		t.Error("AuthEnabled(okta) = true, want false")
	}
}

func TestLoadInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantSub string
	}{
		{"unknown provider", "AUTH_PROVIDERS", "github,bogus", "bogus"},
		{"empty providers", "AUTH_PROVIDERS", " , ", "at least one provider"},
		{"bad log level", "LOG_LEVEL", "verbose", "LOG_LEVEL"},
		{"bad log format", "LOG_FORMAT", "xml", "LOG_FORMAT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := validEnv()
			env[tt.key] = tt.value
			_, err := load(t, env)
			if !errors.Is(err, config.ErrInvalidConfig) {
				t.Fatalf("Load error = %v, want ErrInvalidConfig", err)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error %q does not contain %q", err, tt.wantSub)
			}
		})
	}
}

func TestLoadPrivateKeyFromFile(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "app.pem")
	if err := os.WriteFile(keyPath, []byte(testPEM), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}

	env := validEnv()
	env["GITHUB_APP_PRIVATE_KEY"] = keyPath // a path, not a PEM body

	cfg, err := load(t, env)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if got := cfg.GitHub.PrivateKey.Reveal(); got != testPEM {
		t.Errorf("PrivateKey = %q, want the file contents", got)
	}
}

func TestLoadPrivateKeyMissingFile(t *testing.T) {
	env := validEnv()
	env["GITHUB_APP_PRIVATE_KEY"] = filepath.Join(t.TempDir(), "does-not-exist.pem")

	_, err := load(t, env)
	if err == nil {
		t.Fatal("Load: expected an error for a missing key file")
	}
	// This is a read error, not a validation error.
	if errors.Is(err, config.ErrInvalidConfig) {
		t.Errorf("error = %v, want a file-read error, not ErrInvalidConfig", err)
	}
}

func TestSecretRedaction(t *testing.T) {
	s := config.Secret("hunter2")

	if got := s.String(); got != "REDACTED" {
		t.Errorf("String() = %q, want REDACTED", got)
	}
	if got := s.GoString(); got != "REDACTED" {
		t.Errorf("GoString() = %q, want REDACTED", got)
	}
	if got := s.LogValue().String(); got != "REDACTED" {
		t.Errorf("LogValue() = %q, want REDACTED", got)
	}
	if got := s.Reveal(); got != "hunter2" {
		t.Errorf("Reveal() = %q, want hunter2", got)
	}
	// No formatting verb may leak the value, even when the secret is a struct
	// field printed with %+v / %#v / %v.
	formatted := fmt.Sprintf("%s %v %+v %#v", s, s, struct{ S config.Secret }{S: s}, s)
	if strings.Contains(formatted, "hunter2") {
		t.Errorf("formatted output leaked the secret value: %s", formatted)
	}
}
