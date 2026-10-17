package oauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// clientServer serves a Client ID Metadata Document at /oauth.json and
// whatever the test registers beside it, over TLS so https: URLs resolve.
func clientServer(t *testing.T, mux *http.ServeMux) (*httptest.Server, *ClientFetcher) {
	t.Helper()
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	f := NewClientFetcher(&ClientFetcherOptions{Transport: srv.Client().Transport})

	return srv, f
}

func goodDocument(clientID string) string {
	return `{"client_id":"` + clientID + `","client_name":"Agent","client_uri":"https://agent.example","redirect_uris":["https://agent.example/cb","http://127.0.0.1:9999/cb"]}`
}

func TestClientFetcherResolves(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	mux := http.NewServeMux()
	var clientID string
	mux.HandleFunc("/oauth.json", func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write([]byte(goodDocument(clientID)))
	})
	srv, f := clientServer(t, mux)
	clientID = srv.URL + "/oauth.json"

	meta, err := f.Resolve(t.Context(), clientID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if meta.ClientID != clientID || meta.Name() != "Agent" || len(meta.RedirectURIs) != 2 {
		t.Errorf("Resolve = %+v", meta)
	}

	// Cached for max-age capped at an hour: no second fetch.
	if _, err := f.Resolve(t.Context(), clientID); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Errorf("fetches = %d, want 1 (cached)", hits.Load())
	}
}

func TestClientFetcherRefusals(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	var base string
	mux.HandleFunc("/mismatch.json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(goodDocument("https://elsewhere.example/oauth.json")))
	})
	mux.HandleFunc("/no-redirects.json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"client_id":"` + base + `/no-redirects.json"}`))
	})
	mux.HandleFunc("/bad-redirect.json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"client_id":"` + base + `/bad-redirect.json","redirect_uris":["http://agent.example/cb"]}`))
	})
	mux.HandleFunc("/redirect.json", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, base+"/oauth.json", http.StatusFound)
	})
	mux.HandleFunc("/big.json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(
			[]byte(
				`{"client_id":"` + base + `/big.json","redirect_uris":["https://a.example/cb"],"pad":"` + strings.Repeat(
					"x",
					clientMaxDocument,
				) + `"}`,
			),
		)
	})
	mux.HandleFunc("/not-json.json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html>`))
	})
	mux.HandleFunc("/missing.json", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv, f := clientServer(t, mux)
	base = srv.URL

	tests := []struct {
		name     string
		clientID string
	}{
		{"http scheme", strings.Replace(base, "https://", "http://", 1) + "/oauth.json"},
		{"not a URL", "::nope"},
		{"fragment", base + "/oauth.json#frag"},
		{"userinfo", strings.Replace(base, "https://", "https://u:p@", 1) + "/oauth.json"},
		{"client_id mismatch", base + "/mismatch.json"},
		{"no redirect_uris", base + "/no-redirects.json"},
		{"non-loopback http redirect_uri", base + "/bad-redirect.json"},
		{"behind a redirect", base + "/redirect.json"},
		{"over the size cap", base + "/big.json"},
		{"not JSON", base + "/not-json.json"},
		{"404", base + "/missing.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := f.Resolve(t.Context(), tt.clientID)
			if !errors.Is(err, ErrInvalidClient) {
				t.Errorf("Resolve(%q) = %v, want ErrInvalidClient", tt.clientID, err)
			}
		})
	}
}

// fakeResolver answers every host with one address.
type fakeResolver struct{ addr netip.Addr }

func (r fakeResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{r.addr}, nil
}

func TestClientFetcherRefusesPrivateAddresses(t *testing.T) {
	t.Parallel()

	for _, addr := range []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254", "::1", "fd00::1", "fe80::1", "0.0.0.0", "::ffff:10.0.0.1"} {
		t.Run(addr, func(t *testing.T) {
			t.Parallel()

			f := NewClientFetcher(&ClientFetcherOptions{Resolver: fakeResolver{addr: netip.MustParseAddr(addr)}})
			_, err := f.Resolve(t.Context(), "https://client.example/oauth.json")
			if !errors.Is(err, ErrInvalidClient) || !errors.Is(err, ErrPrivateAddress) {
				t.Errorf("Resolve via %s = %v, want ErrInvalidClient wrapping ErrPrivateAddress", addr, err)
			}
		})
	}
}

func TestIsPrivatePublic(t *testing.T) {
	t.Parallel()

	for _, addr := range []string{"93.184.216.34", "2606:2800:220:1:248:1893:25c8:1946"} {
		if isPrivate(netip.MustParseAddr(addr)) {
			t.Errorf("isPrivate(%s) = true, want false", addr)
		}
	}
}

func TestMaxAge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		header string
		want   time.Duration
	}{
		{"", 0},
		{"no-store", 0},
		{"max-age=60", time.Minute},
		{"public, max-age=3600, immutable", time.Hour},
		{`max-age="120"`, 2 * time.Minute},
		{"max-age=-1", 0},
		{"max-age=abc", 0},
	}
	for _, tt := range tests {
		if got := maxAge(tt.header); got != tt.want {
			t.Errorf("maxAge(%q) = %v, want %v", tt.header, got, tt.want)
		}
	}
}
