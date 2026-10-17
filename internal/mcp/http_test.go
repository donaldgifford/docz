package mcp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/donaldgifford/docz/v2/internal/authorize"
	"github.com/donaldgifford/docz/v2/internal/httpapi"
	"github.com/donaldgifford/docz/v2/internal/oauth"
	"github.com/donaldgifford/docz/v2/internal/session"
	"github.com/donaldgifford/docz/v2/internal/telemetry"
)

// The conformance stack's constants.
const (
	stackToken    = "good-token"
	stackMetadata = "https://docz.example/.well-known/oauth-protected-resource"
	stackLimit    = 3
)

// stackVerifier accepts stackToken as a token docz-api minted for a person.
type stackVerifier struct{}

func (stackVerifier) Verify(_ context.Context, raw string) (oauth.Claims, error) {
	if raw != stackToken {
		return oauth.Claims{}, errors.New("unknown token")
	}

	return oauth.Claims{
		Issuer:   "https://docz.example",
		Subject:  "user:github:42",
		ClientID: "https://agent.example/client.json",
		Scopes:   []string{"public:read"},
		Expiry:   time.Now().Add(time.Hour),
		ID:       "jti-1",
	}, nil
}

// noSessions is the cookie path with nobody logged in.
type noSessions struct{}

func (noSessions) Lookup(context.Context, string) (session.Session, error) {
	return session.Session{}, session.ErrSessionNotFound
}

// allowPlatform is the authorizer: every principal reads acme/platform.
type allowPlatform struct{}

func (allowPlatform) Allowed(context.Context, *http.Request) (authorize.AllowedRepos, error) {
	return authorize.AllowedRepos{repoPlatform}, nil
}

// newStack assembles the router as runServer does for /mcp and /api/v1:
// the telemetry middleware, the Authn gate with a bearer verifier, the
// authorizer, the limiter, and the handler. It returns the server and a
// transport that sends the bearer.
func newStack(t *testing.T, st *fakeStore) (*httptest.Server, http.RoundTripper) {
	t.Helper()
	gate := session.Authn(session.AuthnOptions{
		Sessions:         noSessions{},
		Verifier:         stackVerifier{},
		ResourceMetadata: stackMetadata,
	})
	authz := authorize.Middleware(allowPlatform{})
	limiter := NewLimiter(stackLimit)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(telemetry.Instrument())
	r.Use(middleware.Recoverer)
	r.With(gate, authz, limiter.MCP).Handle("/mcp", Handler(New(&Deps{Store: st, Version: "test"})))
	httpapi.NewHandler(st).Mount(r, func(next http.Handler) http.Handler { return gate(authz(limiter.API(next))) })
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	return srv, bearerTransport{token: stackToken}
}

// bearerTransport adds the Authorization header to every request.
type bearerTransport struct {
	token string
}

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)

	return http.DefaultTransport.RoundTrip(r)
}

// reply is one response, read and closed.
type reply struct {
	status int
	header http.Header
	body   string
}

// do sends one request with the given method, token, and body.
func do(t *testing.T, srv *httptest.Server, method, path, token, body string) reply {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if body != "" {
		// The 2026-07-28 revision carries the version and the method beside
		// the body, and the handler checks them against it.
		req.Header.Set("Mcp-Protocol-Version", ProtocolVersion)
		req.Header.Set("Mcp-Method", "server/discover")
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	return reply{status: res.StatusCode, header: res.Header, body: string(b)}
}

// discover is a 2026-07-28 server/discover request body.
const discover = `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{` +
	`"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
	`"io.modelcontextprotocol/clientInfo":{"name":"test","version":"0"},` +
	`"io.modelcontextprotocol/clientCapabilities":{}}}}`

func TestHTTPUnauthenticatedIsChallengedBeforeTheSDK(t *testing.T) {
	t.Parallel()
	srv, _ := newStack(t, seededStore(t))

	for _, token := range []string{"", "bad-token"} {
		res := do(t, srv, http.MethodPost, "/mcp", token, discover)
		if res.status != http.StatusUnauthorized {
			t.Fatalf("token %q: status = %d, want 401", token, res.status)
		}
		want := `Bearer resource_metadata="` + stackMetadata + `", scope="public:read"`
		if got := res.header.Get("WWW-Authenticate"); got != want {
			t.Errorf("WWW-Authenticate = %q, want %q", got, want)
		}
		if strings.Contains(res.body, "jsonrpc") {
			t.Errorf("the SDK answered an unauthenticated request: %s", res.body)
		}
	}
}

func TestHTTPStatelessMethods(t *testing.T) {
	t.Parallel()
	srv, _ := newStack(t, seededStore(t))

	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		res := do(t, srv, method, "/mcp", stackToken, "")
		if res.status != http.StatusMethodNotAllowed {
			t.Errorf("%s /mcp = %d, want 405", method, res.status)
		}
	}

	res := do(t, srv, http.MethodPost, "/mcp", stackToken, discover)
	if res.status != http.StatusOK {
		t.Fatalf("server/discover = %d: %s", res.status, res.body)
	}
	if res.header.Get("Mcp-Session-Id") != "" {
		t.Error("a stateless server set Mcp-Session-Id")
	}
	if !strings.Contains(res.body, `"name":"docz-api"`) || !strings.Contains(res.body, `"2026-07-28"`) {
		t.Errorf("discover body = %s", res.body)
	}
}

func TestHTTPToolCallThroughTheStack(t *testing.T) {
	t.Parallel()
	srv, rt := newStack(t, seededStore(t))

	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(context.Background(), &sdk.StreamableClientTransport{
		Endpoint:   srv.URL + "/mcp",
		HTTPClient: &http.Client{Transport: rt},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cs.Close() }()

	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 6 {
		t.Errorf("tools = %d, want 6", len(tools.Tools))
	}
	res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: ToolListRepos})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError || !strings.Contains(text(t, res), "acme/platform") {
		t.Errorf("list_repos over HTTP: isError=%v %q", res.IsError, text(t, res))
	}
}

func TestHTTPRateLimit(t *testing.T) {
	t.Parallel()
	srv, _ := newStack(t, seededStore(t))

	var last reply
	for range stackLimit + 1 {
		last = do(t, srv, http.MethodPost, "/mcp", stackToken, discover)
	}
	if last.status != http.StatusTooManyRequests {
		t.Fatalf("request %d = %d, want 429", stackLimit+1, last.status)
	}
	if last.header.Get("Retry-After") == "" {
		t.Error("429 carries no Retry-After")
	}

	// The same principal's budget covers /api/v1 too, and the cookie path is
	// never limited (nobody is logged in here, so it is a 401, not a 429).
	if res := do(t, srv, http.MethodGet, "/api/v1/repos", stackToken, ""); res.status != http.StatusTooManyRequests {
		t.Errorf("/api/v1 with the exhausted token = %d, want 429", res.status)
	}
	if res := do(t, srv, http.MethodGet, "/api/v1/repos", "", ""); res.status != http.StatusUnauthorized {
		t.Errorf("/api/v1 without a token = %d, want 401", res.status)
	}

	rec := httptest.NewRecorder()
	telemetry.MetricsHandler().ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody))
	body := rec.Body.String()
	for _, series := range []string{
		`docz_api_rate_limited_total{surface="mcp",via="token"}`,
		`docz_api_rate_limited_total{surface="api",via="token"}`,
	} {
		if !strings.Contains(body, series) {
			t.Errorf("metrics lack %s", series)
		}
	}
}
