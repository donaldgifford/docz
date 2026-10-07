package main

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/internal/config"
)

// rewrite sends every request to srv, whatever host it names.
type rewrite struct{ srv *httptest.Server }

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	target, err := url.Parse(r.srv.URL)
	if err != nil {
		return nil, err
	}

	out := req.Clone(req.Context())
	out.URL.Scheme, out.URL.Host, out.Host = target.Scheme, target.Host, ""

	return http.DefaultTransport.RoundTrip(out)
}

func TestCheckConfluenceCredentials(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
		wantLog string
	}{
		{name: "ok", status: 200, body: `{"results":[{"id":"9","key":"DOCZ"},{"id":"10","key":"ENG"}]}`, wantLog: "confluence credentials verified"},
		{name: "space missing", status: 200, body: `{"results":[{"id":"9","key":"DOCZ"}]}`, wantLog: "space=ENG"},
		{name: "rejected", status: 401, body: `{"message":"unauthorized"}`, wantErr: true},
		{name: "forbidden", status: 403, body: `{"message":"scope does not match"}`, wantLog: "could not verify"},
		{name: "unavailable", status: 503, body: `{}`, wantLog: "could not verify"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/_edge/tenant_info" {
					io.WriteString(w, `{"cloudId":"c1"}`)

					return
				}

				if !strings.HasSuffix(r.URL.Path, "/wiki/api/v2/spaces") || r.URL.Query().Get("keys") != "DOCZ,ENG" {
					t.Errorf("request %s; want the spaces lookup", r.URL)
				}

				w.WriteHeader(tt.status)
				io.WriteString(w, tt.body)
			}))
			defer srv.Close()

			var logs bytes.Buffer

			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))

			defer slog.SetDefault(prev)

			cfg := &config.ConfluenceConfig{
				Site: "https://example.atlassian.net", Email: "bot@example.com",
				APIToken: config.Secret("tok-secret"), Spaces: []string{"DOCZ", "ENG"},
			}
			client := newConfluenceClient(cfg, &http.Client{Transport: rewrite{srv}})

			err := checkConfluenceCredentials(t.Context(), client, cfg.Spaces)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err %v; want error %v", err, tt.wantErr)
			}

			if tt.wantLog != "" && !strings.Contains(logs.String(), tt.wantLog) {
				t.Errorf("logs %q; want %q", logs.String(), tt.wantLog)
			}

			if strings.Contains(logs.String()+errString(err), "tok-secret") {
				t.Error("the token leaked")
			}
		})
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}
