package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/donaldgifford/docz-api/internal/config"
)

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

	newRouter().ServeHTTP(rec, req)

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
