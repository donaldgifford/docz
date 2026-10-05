package confluence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testSite  = "https://example.atlassian.net"
	testCloud = "11111111-2222-3333-4444-555555555555"
	testToken = "s3cr3t-t0ken-value"
)

// recorded is one request the fake server saw.
type recorded struct {
	Host   string
	Method string
	Path   string
	Query  string
	Auth   bool
	Body   string
}

// fakeAtlassian is an httptest server standing in for both the site and the
// gateway: a transport rewrites every request to it and keeps the host the
// client meant, so a test can see which of the two each request went to.
type fakeAtlassian struct {
	t       *testing.T
	srv     *httptest.Server
	mu      sync.Mutex
	reqs    []recorded
	handler func(w http.ResponseWriter, r *http.Request, body []byte)
}

func newFake(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, body []byte)) *fakeAtlassian {
	t.Helper()

	f := &fakeAtlassian{t: t, handler: handler}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)

	return f
}

func (f *fakeAtlassian) serve(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		f.t.Errorf("read request body: %v", err)
	}

	user, pass, ok := r.BasicAuth()

	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{
		Host:   r.Header.Get("X-Original-Host"),
		Method: r.Method,
		Path:   r.URL.Path,
		Query:  r.URL.RawQuery,
		Auth:   ok && user == "me@example.com" && pass == testToken,
		Body:   string(body),
	})
	f.mu.Unlock()

	if r.URL.Path == "/_edge/tenant_info" {
		fmt.Fprintf(w, `{"cloudId":%q}`, testCloud)

		return
	}

	f.handler(w, r, body)
}

func (f *fakeAtlassian) requests() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]recorded(nil), f.reqs...)
}

// RoundTrip sends every request to the fake server, recording its host.
func (f *fakeAtlassian) RoundTrip(r *http.Request) (*http.Response, error) {
	target, err := url.Parse(f.srv.URL)
	if err != nil {
		return nil, err
	}

	out := r.Clone(r.Context())
	out.Header.Set("X-Original-Host", r.URL.Host)
	out.URL.Scheme, out.URL.Host, out.Host = target.Scheme, target.Host, ""

	return http.DefaultTransport.RoundTrip(out)
}

// client builds an HTTPClient against the fake, with a recorded sleep.
func (f *fakeAtlassian) client(waits *[]time.Duration) *HTTPClient {
	h := NewHTTPClient(testSite+"/", "me@example.com", testToken, WithHTTPClient(&http.Client{Transport: f}))
	h.sleep = func(ctx context.Context, d time.Duration) error {
		if waits != nil {
			*waits = append(*waits, d)
		}

		return ctx.Err()
	}

	return h
}

// apiCalls drops the tenant_info request and checks the rest went to the
// gateway, authenticated, under the cloud id.
func apiCalls(t *testing.T, reqs []recorded) []recorded {
	t.Helper()

	var out []recorded

	for _, r := range reqs {
		if r.Path == "/_edge/tenant_info" {
			if r.Host != "example.atlassian.net" || r.Auth {
				t.Errorf("tenant_info went to %q (auth %v); want the site, unauthenticated", r.Host, r.Auth)
			}

			continue
		}

		if r.Host != "api.atlassian.com" {
			t.Errorf("%s %s went to %q; want the gateway", r.Method, r.Path, r.Host)
		}

		if !r.Auth {
			t.Errorf("%s %s carried no basic auth", r.Method, r.Path)
		}

		prefix := "/ex/confluence/" + testCloud + "/wiki/api/v2/"
		if !strings.HasPrefix(r.Path, prefix) {
			t.Errorf("path %q lacks %q", r.Path, prefix)
		}

		r.Path = "/" + strings.TrimPrefix(r.Path, prefix)
		out = append(out, r)
	}

	return out
}

const pageJSON = `{"id":"42","title":"RFC-0001: X","parentId":"7","spaceId":"9",` +
	`"version":{"number":3},"_links":{"webui":"/spaces/DOCZ/pages/42"}}`

func TestHTTPClient_Endpoints(t *testing.T) {
	t.Parallel()

	want := &Page{
		ID: "42", Title: "RFC-0001: X", ParentID: "7", SpaceID: "9", Version: 3,
		WebURL: testSite + "/wiki/spaces/DOCZ/pages/42",
	}

	tests := []struct {
		name  string
		reply string
		call  func(ctx context.Context, h *HTTPClient) (any, error)
		want  any
	}{
		{
			name:  "space_id",
			reply: `{"results":[{"id":"9","key":"DOCZ"}]}`,
			call:  func(ctx context.Context, h *HTTPClient) (any, error) { return h.SpaceID(ctx, "DOCZ") },
			want:  "9",
		},
		{
			name:  "find_page",
			reply: `{"results":[` + pageJSON + `]}`,
			call: func(ctx context.Context, h *HTTPClient) (any, error) {
				return h.FindPage(ctx, "9", "RFC-0001: X")
			},
			want: want,
		},
		{
			name:  "find_page_missing",
			reply: `{"results":[]}`,
			call: func(ctx context.Context, h *HTTPClient) (any, error) {
				return h.FindPage(ctx, "9", "Nope & Co")
			},
			want: (*Page)(nil),
		},
		{
			name:  "create_page",
			reply: pageJSON,
			call: func(ctx context.Context, h *HTTPClient) (any, error) {
				return h.CreatePage(ctx, &NewPage{SpaceID: "9", ParentID: "7", Title: "RFC-0001: X", Body: []byte("<p>x</p>")})
			},
			want: want,
		},
		{
			name:  "update_page",
			reply: pageJSON,
			call: func(ctx context.Context, h *HTTPClient) (any, error) {
				return h.UpdatePage(ctx, "42", &PageUpdate{
					Title: "RFC-0001: X", ParentID: "7", Body: []byte("<p>y</p>"), Version: 3, Message: "docz export",
				})
			},
			want: want,
		},
		{
			name:  "property",
			reply: `{"results":[{"id":"p1","key":"docz","value":{"hash":"abc"},"version":{"number":2}}]}`,
			call:  func(ctx context.Context, h *HTTPClient) (any, error) { return h.Property(ctx, "42", "docz") },
			want:  &Property{ID: "p1", Key: "docz", Value: json.RawMessage(`{"hash":"abc"}`), Version: 2},
		},
		{
			name:  "property_missing",
			reply: `{"results":[]}`,
			call:  func(ctx context.Context, h *HTTPClient) (any, error) { return h.Property(ctx, "42", "docz") },
			want:  (*Property)(nil),
		},
		{
			name:  "set_property_create",
			reply: `{}`,
			call: func(ctx context.Context, h *HTTPClient) (any, error) {
				return nil, h.SetProperty(ctx, "42", &Property{Key: "docz", Value: json.RawMessage(`{"hash":"abc"}`)})
			},
		},
		{
			name:  "set_property_update",
			reply: `{}`,
			call: func(ctx context.Context, h *HTTPClient) (any, error) {
				return nil, h.SetProperty(ctx, "42", &Property{
					ID: "p1", Key: "docz", Value: json.RawMessage(`{"hash":"def"}`), Version: 2,
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFake(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
				io.WriteString(w, tt.reply)
			})

			got, err := tt.call(t.Context(), f.client(nil))
			if err != nil {
				t.Fatalf("call: %v", err)
			}

			if tt.want != nil && fmt.Sprintf("%+v", deref(got)) != fmt.Sprintf("%+v", deref(tt.want)) {
				t.Errorf("got %+v, want %+v", deref(got), deref(tt.want))
			}

			checkGolden(t, "client/"+tt.name+".golden", requestShape(apiCalls(t, f.requests())))
		})
	}
}

// deref prints a pointer's target so two equal values compare equal.
func deref(v any) any {
	switch p := v.(type) {
	case *Page:
		if p != nil {
			return *p
		}
	case *Property:
		if p != nil {
			return fmt.Sprintf("%s %s %s v%d", p.ID, p.Key, p.Value, p.Version)
		}
	}

	return v
}

// requestShape renders requests as the golden records them.
func requestShape(reqs []recorded) []byte {
	var b bytes.Buffer

	for _, r := range reqs {
		fmt.Fprintf(&b, "%s %s", r.Method, r.Path)

		if r.Query != "" {
			fmt.Fprintf(&b, "?%s", r.Query)
		}

		b.WriteString("\n")

		if r.Body != "" {
			var pretty bytes.Buffer
			if err := json.Indent(&pretty, []byte(r.Body), "", "  "); err != nil {
				pretty.WriteString(r.Body)
			}

			b.Write(pretty.Bytes())
			b.WriteString("\n")
		}
	}

	return b.Bytes()
}

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()

	path := filepath.Join("testdata", name)

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}

		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update): %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("%s mismatch:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestHTTPClient_CloudIDResolvedOnce(t *testing.T) {
	t.Parallel()

	f := newFake(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		io.WriteString(w, `{"results":[{"id":"9","key":"DOCZ"}]}`)
	})
	h := f.client(nil)

	for range 5 {
		if _, err := h.SpaceID(t.Context(), "DOCZ"); err != nil {
			t.Fatal(err)
		}
	}

	var tenant int

	for _, r := range f.requests() {
		if r.Path == "/_edge/tenant_info" {
			tenant++
		}
	}

	if tenant != 1 {
		t.Errorf("tenant_info requested %d times, want 1", tenant)
	}

	if got := len(apiCalls(t, f.requests())); got != 5 {
		t.Errorf("%d API calls, want 5", got)
	}
}

func TestHTTPClient_BadSiteFailsEveryCall(t *testing.T) {
	t.Parallel()

	h := NewHTTPClient("http://127.0.0.1:1", "me@example.com", testToken)

	_, first := h.SpaceID(t.Context(), "DOCZ")
	_, second := h.FindPage(t.Context(), "9", "x")

	if first == nil || second == nil || first.Error() != second.Error() {
		t.Errorf("errors %v / %v; want the same memoised failure", first, second)
	}
}

func TestHTTPClient_ChildrenPaginates(t *testing.T) {
	t.Parallel()

	f := newFake(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		cursor := r.URL.Query().Get("cursor")

		next := map[string]string{"": "c2", "c2": "c3"}[cursor]
		link := ""

		if next != "" {
			link = `"/wiki/api/v2/pages/7/children?limit=250&cursor=` + next + `"`
		} else {
			link = `""`
		}

		fmt.Fprintf(w, `{"results":[{"id":"p-%s","title":"T %s"}],"_links":{"next":%s}}`, cursor, cursor, link)
	})

	got, err := f.client(nil).Children(t.Context(), "7")
	if err != nil {
		t.Fatal(err)
	}

	ids := make([]string, 0, len(got))

	for _, p := range got {
		if p.ParentID != "7" {
			t.Errorf("child %s has parent %q", p.ID, p.ParentID)
		}

		ids = append(ids, p.ID)
	}

	if strings.Join(ids, ",") != "p-,p-c2,p-c3" {
		t.Errorf("children %v; want three pages followed", ids)
	}

	checkGolden(t, "client/children.golden", requestShape(apiCalls(t, f.requests())))
}

// status answers with code and a body that echoes the token, so a test can
// prove no error passes it on.
func status(code int, header http.Header) func(http.ResponseWriter, *http.Request, []byte) {
	return func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		for k, v := range header {
			w.Header()[k] = v
		}

		w.WriteHeader(code)
		io.WriteString(w, `{"message":"denied","detail":"`+strings.Repeat("x", 1000)+`"}`)
	}
}

func TestHTTPClient_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		code  int
		check func(t *testing.T, err error)
	}{
		{"unauthorized", http.StatusUnauthorized, func(t *testing.T, err error) {
			t.Helper()

			var ae *AuthError
			if !errors.As(err, &ae) || ae.Status != http.StatusUnauthorized {
				t.Errorf("got %v; want AuthError 401", err)
			}
		}},
		{"forbidden", http.StatusForbidden, func(t *testing.T, err error) {
			t.Helper()

			var ae *AuthError
			if !errors.As(err, &ae) || ae.Status != http.StatusForbidden {
				t.Errorf("got %v; want AuthError 403", err)
			}
		}},
		{"conflict", http.StatusConflict, func(t *testing.T, err error) {
			t.Helper()

			var ce *ConflictError
			if !errors.As(err, &ce) || ce.Title != "RFC-0001: X" || ce.Want != 3 {
				t.Errorf("got %#v; want ConflictError for v3", err)
			}
		}},
		{"server", http.StatusInternalServerError, func(t *testing.T, err error) {
			t.Helper()

			var re *RequestError
			if !errors.As(err, &re) || re.Status != http.StatusInternalServerError || len(re.Body) > maxErrorBody {
				t.Errorf("got %v; want RequestError 500 with a capped body", err)
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFake(t, status(tt.code, nil))

			_, err := f.client(nil).UpdatePage(t.Context(), "42", &PageUpdate{Title: "RFC-0001: X", Version: 4})
			tt.check(t, err)

			if err != nil && strings.Contains(fmt.Sprintf("%v %+v %#v", err, err, err), testToken) {
				t.Errorf("error carries the token: %v", err)
			}
		})
	}
}

func TestHTTPClient_RateLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		limited   int
		header    string
		wantWaits []time.Duration
		wantErr   bool
	}{
		{"retry_after_seconds", 2, "5", []time.Duration{5 * time.Second, 5 * time.Second}, false},
		{"no_header_doubles", 2, "", []time.Duration{2 * time.Second, 4 * time.Second}, false},
		{"exhausted", 4, "1", []time.Duration{time.Second, time.Second, time.Second}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				mu    sync.Mutex
				calls int
			)

			f := newFake(t, func(w http.ResponseWriter, r *http.Request, b []byte) {
				mu.Lock()
				calls++
				n := calls
				mu.Unlock()

				if n <= tt.limited {
					h := http.Header{}
					if tt.header != "" {
						h.Set("Retry-After", tt.header)
					}

					status(http.StatusTooManyRequests, h)(w, r, b)

					return
				}

				io.WriteString(w, `{"results":[{"id":"9","key":"DOCZ"}]}`)
			})

			var waits []time.Duration

			id, err := f.client(&waits).SpaceID(t.Context(), "DOCZ")

			var re *RequestError
			if tt.wantErr != (err != nil) || (tt.wantErr && (!errors.As(err, &re) || re.Status != http.StatusTooManyRequests)) {
				t.Fatalf("err %v; want error %v", err, tt.wantErr)
			}

			if !tt.wantErr && id != "9" {
				t.Errorf("id %q, want 9", id)
			}

			if fmt.Sprint(waits) != fmt.Sprint(tt.wantWaits) {
				t.Errorf("waits %v, want %v", waits, tt.wantWaits)
			}
		})
	}
}

func TestRetryAfter(t *testing.T) {
	t.Parallel()

	future := time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)

	if got := retryAfter(future, time.Second); got < 59*time.Minute || got > time.Hour {
		t.Errorf("HTTP date gave %v; want about an hour", got)
	}

	if got := retryAfter("garbage", 3*time.Second); got != 3*time.Second {
		t.Errorf("garbage gave %v; want the fallback", got)
	}
}

func TestHTTPClient_CancelDuringWait(t *testing.T) {
	t.Parallel()

	f := newFake(t, status(http.StatusTooManyRequests, http.Header{"Retry-After": {"60"}}))
	h := NewHTTPClient(testSite, "me@example.com", testToken, WithHTTPClient(&http.Client{Transport: f}))

	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(50*time.Millisecond, cancel)

	start := time.Now()
	_, err := h.SpaceID(ctx, "DOCZ")

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err %v; want context.Canceled", err)
	}

	if time.Since(start) > 10*time.Second {
		t.Error("cancellation slept through the Retry-After wait")
	}
}
