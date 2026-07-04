package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/donaldgifford/docz-api/internal/config"
)

// checkersFor builds a single-dependency readiness checker returning err,
// letting the probe be tested without real dependencies.
func checkersFor(name string, err error) []namedChecker {
	return []namedChecker{{name: name, check: func(context.Context) error { return err }}}
}

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.LogConfig
		wantErr bool
	}{
		{"text info", config.LogConfig{Level: "info", Format: "text"}, false},
		{"json debug", config.LogConfig{Level: "debug", Format: "json"}, false},
		{"text warn", config.LogConfig{Level: "warn", Format: "text"}, false},
		{"json error", config.LogConfig{Level: "error", Format: "json"}, false},
		{"bad level", config.LogConfig{Level: "verbose", Format: "text"}, true},
		{"bad format", config.LogConfig{Level: "info", Format: "xml"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := newLogger(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("newLogger(%+v) = nil error, want error", tt.cfg)
				}
				return
			}
			if err != nil {
				t.Fatalf("newLogger(%+v): unexpected error: %v", tt.cfg, err)
			}
			if logger == nil {
				t.Fatal("newLogger returned a nil logger")
			}
		})
	}
}

func TestHealthz(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", http.NoBody)
	rec := httptest.NewRecorder()

	newRouter(nil).ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if got := string(body); got != `{"status":"ok"}` {
		t.Errorf("body = %q, want the ok payload", got)
	}
}

func TestParseOnboardSpec(t *testing.T) {
	tests := []struct {
		name     string
		spec     string
		wantOwn  string
		wantName string
		wantID   int64
		wantErr  bool
	}{
		{"valid", "acme/platform@42", "acme", "platform", 42, false},
		{"missing at", "acme/platform", "", "", 0, true},
		{"missing slash", "acme@42", "", "", 0, true},
		{"empty owner", "/platform@42", "", "", 0, true},
		{"empty name", "acme/@42", "", "", 0, true},
		{"bad id", "acme/platform@notanumber", "", "", 0, true},
		{"zero id", "acme/platform@0", "", "", 0, true},
		{"negative id", "acme/platform@-1", "", "", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, name, id, err := parseOnboardSpec(tt.spec)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseOnboardSpec(%q) = nil error, want error", tt.spec)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseOnboardSpec(%q): %v", tt.spec, err)
			}
			if owner != tt.wantOwn || name != tt.wantName || id != tt.wantID {
				t.Errorf("parseOnboardSpec(%q) = (%q, %q, %d), want (%q, %q, %d)",
					tt.spec, owner, name, id, tt.wantOwn, tt.wantName, tt.wantID)
			}
		})
	}
}

func TestReadyz(t *testing.T) {
	tests := []struct {
		name     string
		checkErr error
		wantCode int
		wantVal  string
	}{
		{"reachable", nil, http.StatusOK, "ok"},
		{"unreachable", errors.New("dial tcp: refused"), http.StatusServiceUnavailable, "unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", http.NoBody)
			rec := httptest.NewRecorder()

			newRouter(checkersFor("postgres", tt.checkErr)).ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != tt.wantCode {
				t.Errorf("status = %d, want %d", res.StatusCode, tt.wantCode)
			}
			var body map[string]string
			if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["postgres"] != tt.wantVal {
				t.Errorf("postgres = %q, want %q (body %v)", body["postgres"], tt.wantVal, body)
			}
		})
	}
}

func TestReadyzReportsPerDependency(t *testing.T) {
	checkers := []namedChecker{
		{name: "postgres", check: func(context.Context) error { return nil }},
		{name: "meilisearch", check: func(context.Context) error { return errors.New("down") }},
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", http.NoBody)
	rec := httptest.NewRecorder()
	newRouter(checkers).ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 when a dependency is down", res.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["postgres"] != "ok" || body["meilisearch"] != "unavailable" {
		t.Errorf("body = %v, want postgres ok + meilisearch unavailable", body)
	}
}
