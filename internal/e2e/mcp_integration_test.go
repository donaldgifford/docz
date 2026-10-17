//go:build integration

package e2e

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/donaldgifford/docz/v2/internal/auth"
	"github.com/donaldgifford/docz/v2/internal/authhttp"
	"github.com/donaldgifford/docz/v2/internal/authorize"
	"github.com/donaldgifford/docz/v2/internal/config"
	"github.com/donaldgifford/docz/v2/internal/export"
	"github.com/donaldgifford/docz/v2/internal/httpapi"
	"github.com/donaldgifford/docz/v2/internal/ingest"
	"github.com/donaldgifford/docz/v2/internal/mcp"
	"github.com/donaldgifford/docz/v2/internal/oauth"
	"github.com/donaldgifford/docz/v2/internal/session"
	"github.com/donaldgifford/docz/v2/internal/telemetry"
	"github.com/donaldgifford/docz/v2/pkg/export/confluence/confluencetest"
)

// The MCP suite (IMPL-0026 Phase 6) proves DESIGN-0023 end to end over the
// shared Postgres and a Redis container: the code flow through a consent
// page that plays docz-site, a none deployment, access.scopes deciding what
// a principal sees, a trusted issuer's token, refresh reuse, and the export
// refusing a private repository until it is granted confluence:write.

// The PKCE pair from RFC 7636 §B and the agent's registered redirect.
const (
	pkceVerifier  = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	pkceChallenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	agentRedirect = "https://agent.example/cb"
	stateSecret   = "e2e-state-secret-0123456789abcdef"
	teamScope     = "team:read"
	publicScope   = "public:read"
)

// startRedis runs a Redis container for one test.
func startRedis(t *testing.T) string {
	t.Helper()
	ctx := t.Context()
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start redis: %v", err)
	}
	t.Cleanup(func() {
		if terr := ctr.Terminate(context.Background()); terr != nil {
			log.Printf("terminate redis: %v", terr)
		}
	})
	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatalf("redis host: %v", err)
	}
	port, err := ctr.MappedPort(ctx, "6379/tcp")
	if err != nil {
		t.Fatalf("redis port: %v", err)
	}

	return "redis://" + host + ":" + port.Port()
}

// fakeProvider is a login provider whose codes are the identities they
// exchange for.
type fakeProvider struct {
	identities map[string]auth.Identity
}

func (fakeProvider) Name() string { return "github" }

func (fakeProvider) AuthCodeURL(state string) string {
	return "https://github.example/login/oauth/authorize?state=" + url.QueryEscape(state)
}

func (p fakeProvider) Exchange(_ context.Context, code string) (*auth.Identity, error) {
	id, ok := p.identities[code]
	if !ok {
		return nil, errors.New("unknown code")
	}

	return &id, nil
}

// fakeIssuer is a trusted external issuer: discovery, a JWKS, and a key to
// mint with.
type fakeIssuer struct {
	srv *httptest.Server
	key *ecdsa.PrivateKey
}

func newFakeIssuer(t *testing.T) *fakeIssuer {
	t.Helper()
	f := &fakeIssuer{key: newECKey(t)}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": f.srv.URL, "jwks_uri": f.srv.URL + "/keys"})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
			{Key: &f.key.PublicKey, KeyID: "e2e", Algorithm: "ES256", Use: "sig"},
		}})
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)

	return f
}

// mint signs a token for audience carrying scopes, as a platform issuer
// would for an agent.
func (f *fakeIssuer) mint(t *testing.T, audience, subject string, scopes []string) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: f.key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "e2e"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	raw, err := jwt.Signed(signer).
		Claims(&jwt.Claims{
			Issuer: f.srv.URL, Subject: subject, Audience: jwt.Audience{audience},
			Expiry: jwt.NewNumericDate(now.Add(time.Hour)), IssuedAt: jwt.NewNumericDate(now),
		}).
		Claims(map[string]any{"scope": strings.Join(scopes, " ")}).
		Serialize()
	if err != nil {
		t.Fatal(err)
	}

	return raw
}

func newECKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	return key
}

// stackOptions says which deployment newMCPStack assembles.
type stackOptions struct {
	// none is AUTH_PROVIDERS=none: no login, every request anonymous.
	none bool
	// provider is the login; nil with none.
	provider auth.Provider
	// scopeMap is AUTHZ_SCOPE_MAP.
	scopeMap map[string][]string
}

// mcpStack is the server as runServer assembles it with MCP_ENABLED, over
// the shared store: the Authn gate with the verifier, the scope authorizer,
// the limiter, /mcp, /api/v1, the authorization server, the login
// endpoints, and beside it a consent page that plays docz-site.
type mcpStack struct {
	srv      *httptest.Server
	issuer   string
	audience string
	clientID string
	trusted  *fakeIssuer
	site     *httptest.Server
	// cookie is the session the consent page acts for, set by login.
	cookie atomic.Pointer[http.Cookie]
	// client follows no redirect, so every hop is asserted.
	client *http.Client
}

func newMCPStack(t *testing.T, redisURL string, opts stackOptions) *mcpStack {
	t.Helper()
	st := &mcpStack{
		trusted: newFakeIssuer(t),
		client: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
	}

	// The issuer is the server's own URL, known before it starts.
	st.srv = httptest.NewUnstartedServer(nil)
	st.issuer = "http://" + st.srv.Listener.Addr().String()
	st.audience = st.issuer + "/mcp"

	sessions, err := session.New(redisURL, time.Hour, false)
	if err != nil {
		t.Fatalf("session store: %v", err)
	}
	t.Cleanup(func() { _ = sessions.Close() })
	cache, err := oauth.NewRedisCache(redisURL)
	if err != nil {
		t.Fatalf("jwks cache: %v", err)
	}
	t.Cleanup(func() { _ = cache.Close() })

	key := newECKey(t)
	verifier, err := oauth.NewVerifier(t.Context(), &oauth.VerifierOptions{
		Audience: st.audience, Issuer: st.issuer, Key: &key.PublicKey,
		TrustedIssuers: []string{st.trusted.srv.URL}, Cache: cache,
		Observe: telemetry.ObserveTokenVerification,
	})
	if err != nil {
		t.Fatalf("verifier: %v", err)
	}

	var authorizer authorize.Authorizer = authorize.NewScopeAuthorizer(testStore, 0)
	if opts.none {
		authorizer = authorize.NewAllReposAuthorizer(testStore)
	}
	scopeMap := opts.scopeMap
	authn := session.Authn(session.AuthnOptions{
		Sessions:         sessions,
		Verifier:         verifier,
		Scopes:           func(id *auth.Identity) []string { return authorize.ScopesFor(id, scopeMap) },
		Anonymous:        opts.none,
		ResourceMetadata: st.issuer + oauth.ProtectedResourcePath,
	})
	authz := authorize.Middleware(authorizer)
	gate := func(next http.Handler) http.Handler { return authn(authz(next)) }
	limiter := mcp.NewLimiter(1000)

	router := chi.NewRouter()
	router.Use(middleware.RequestID, telemetry.Instrument(), middleware.Recoverer)
	router.With(gate, limiter.MCP).Handle("/mcp", mcp.Handler(mcp.New(&mcp.Deps{Store: testStore, Version: "e2e"})))
	apiGate := func(next http.Handler) http.Handler { return gate(limiter.API(next)) }

	if opts.none {
		httpapi.NewHandler(testStore).Mount(router, apiGate)
		st.srv.Config.Handler = router
		st.srv.Start()
		t.Cleanup(st.srv.Close)

		return st
	}

	signer, err := oauth.NewSigner(st.issuer, st.audience, key, 15*time.Minute)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	// The agent's client metadata document, served over TLS as the
	// fetcher requires.
	cs := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"client_id":"`+st.clientID+`","client_name":"Agent","client_uri":"https://agent.example",`+
			`"redirect_uris":["`+agentRedirect+`"]}`)
	}))
	t.Cleanup(cs.Close)
	st.clientID = cs.URL + "/oauth.json"
	st.site = httptest.NewServer(http.HandlerFunc(st.consentPage))
	t.Cleanup(st.site.Close)

	authServer := oauth.NewServer(&oauth.ServerOptions{
		Issuer: st.issuer, Audience: st.audience, SiteURL: st.site.URL,
		Signer: signer, ScopeMap: scopeMap,
		Clients:    oauth.NewClientFetcher(&oauth.ClientFetcherOptions{Transport: cs.Client().Transport, Cache: cache}),
		Cache:      cache,
		Store:      testStore,
		RefreshTTL: 720 * time.Hour,
		Session: func(ctx context.Context) (oauth.Principal, bool) {
			sess, ok := session.FromContext(ctx)
			if !ok {
				return oauth.Principal{}, false
			}

			return oauth.Principal{SessionID: sess.ID, Identity: sess.Identity, Scopes: sess.Scopes}, true
		},
		Observe: telemetry.ObserveOAuthToken,
	})
	authHandler := authhttp.New(auth.NewRegistry([]auth.Provider{opts.provider}), sessions, testStore, []byte(stateSecret))
	httpapi.NewHandler(testStore).Mount(router, apiGate, authHandler.MountAPI, authServer.MountAPI)
	authServer.MountPublic(router)
	authHandler.MountPublic(router)

	st.srv.Config.Handler = router
	st.srv.Start()
	t.Cleanup(st.srv.Close)

	return st
}

// consentPage is docz-site's /oauth/consent: read the pending authorization
// as the signed-in person, allow it, and send the browser where the server
// says.
func (st *mcpStack) consentPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/oauth/consent" {
		http.NotFound(w, r)

		return
	}
	cookie := st.cookie.Load()
	if cookie == nil {
		http.Error(w, "nobody is logged in", http.StatusUnauthorized)

		return
	}
	api := st.issuer + "/api/v1/oauth/authorizations/" + r.URL.Query().Get("authz")

	var view oauth.AuthorizationView
	if status, err := siteCall(r.Context(), http.MethodGet, api, "", cookie, &view); err != nil || status != http.StatusOK {
		http.Error(w, "read authorization: "+strconv.Itoa(status), http.StatusBadGateway)

		return
	}
	if len(view.ScopesGranted) == 0 {
		http.Error(w, "nothing to grant", http.StatusForbidden)

		return
	}
	var res oauth.DecisionResult
	if status, err := siteCall(r.Context(), http.MethodPost, api, `{"decision":"allow"}`, cookie, &res); err != nil ||
		status != http.StatusOK {
		http.Error(w, "decide: "+strconv.Itoa(status), http.StatusBadGateway)

		return
	}
	http.Redirect(w, r, res.RedirectTo, http.StatusFound)
}

// siteCall is one API request the consent page makes with the person's
// cookie.
func siteCall(ctx context.Context, method, target, body string, cookie *http.Cookie, dst any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, strings.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.AddCookie(cookie)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return res.StatusCode, nil
	}

	return res.StatusCode, json.NewDecoder(res.Body).Decode(dst)
}

// reply is one response, read and closed.
type reply struct {
	target  string
	status  int
	header  http.Header
	cookies []*http.Cookie
	body    string
}

// get sends one request without following redirects.
func (st *mcpStack) get(t *testing.T, target string, cookie *http.Cookie) reply {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res, err := st.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	return reply{target: target, status: res.StatusCode, header: res.Header, cookies: res.Cookies(), body: string(b)}
}

// api sends one bearer request to the REST API.
func (st *mcpStack) api(t *testing.T, path, token string) reply {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, st.issuer+path, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := st.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	return reply{target: path, status: res.StatusCode, header: res.Header, body: string(b)}
}

// location asserts a redirect and returns where it points.
func location(t *testing.T, res reply) *url.URL {
	t.Helper()
	if res.status != http.StatusFound {
		t.Fatalf("%s = %d, want 302: %s", res.target, res.status, res.body)
	}
	loc, err := url.Parse(res.header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}

	return loc
}

// login runs /auth/login and /auth/callback for code and returns the
// session cookie, which the consent page then acts for.
func (st *mcpStack) login(t *testing.T, code string) *http.Cookie {
	t.Helper()
	state := location(t, st.get(t, st.issuer+"/auth/login?provider=github", nil)).Query().Get("state")
	if state == "" {
		t.Fatal("login redirected without a state")
	}
	res := st.get(t, st.issuer+"/auth/callback?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(state), nil)
	location(t, res)
	for _, c := range res.cookies {
		if c.Name == "docz_session" {
			st.cookie.Store(c)

			return c
		}
	}
	t.Fatal("callback set no session cookie")

	return nil
}

// authorize runs the code flow through the consent page and returns the
// code the agent would redeem.
func (st *mcpStack) authorize(t *testing.T, scope string) string {
	t.Helper()
	q := url.Values{
		"response_type": {"code"}, "client_id": {st.clientID}, "redirect_uri": {agentRedirect},
		"code_challenge": {pkceChallenge}, "code_challenge_method": {"S256"},
		"resource": {st.audience}, "scope": {scope}, "state": {"xyz"},
	}
	consent := location(t, st.get(t, st.issuer+oauth.AuthorizePath+"?"+q.Encode(), nil))
	if !strings.HasPrefix(consent.String(), st.site.URL+"/oauth/consent?authz=") {
		t.Fatalf("authorize sent the browser to %s, want the site's consent page", consent)
	}
	cb := location(t, st.get(t, consent.String(), nil))
	if cb.Scheme+"://"+cb.Host+cb.Path != agentRedirect || cb.Query().Get("state") != "xyz" || cb.Query().Get("iss") != st.issuer {
		t.Fatalf("consent redirected to %s, want %s with state and iss", cb, agentRedirect)
	}

	return cb.Query().Get("code")
}

// tokenCall posts to /oauth/token and returns the response or the error
// code.
func (st *mcpStack) tokenCall(t *testing.T, form url.Values) (oauth.TokenResponse, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, st.issuer+oauth.TokenPath, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := st.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var out struct {
		oauth.TokenResponse
		Error string `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK && out.Error == "" {
		t.Fatalf("token = %d with no error code", res.StatusCode)
	}

	return out.TokenResponse, out.Error
}

func (st *mcpStack) redeem(t *testing.T, code string) oauth.TokenResponse {
	t.Helper()
	tok, oauthErr := st.tokenCall(t, url.Values{
		"grant_type": {oauth.GrantAuthorizationCode}, "code": {code}, "code_verifier": {pkceVerifier},
		"client_id": {st.clientID}, "redirect_uri": {agentRedirect}, "resource": {st.audience},
	})
	if oauthErr != "" {
		t.Fatalf("redeem = %s", oauthErr)
	}

	return tok
}

func (st *mcpStack) refresh(t *testing.T, refreshToken string) (oauth.TokenResponse, string) {
	t.Helper()

	return st.tokenCall(t, url.Values{"grant_type": {oauth.GrantRefreshToken}, "refresh_token": {refreshToken}})
}

// bearerTransport sends every request with the token.
type bearerTransport struct {
	token string
}

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)

	return http.DefaultTransport.RoundTrip(r)
}

// mcpClient connects an SDK client to /mcp, with the token or without one.
func (st *mcpStack) mcpClient(t *testing.T, token string) *sdk.ClientSession {
	t.Helper()
	rt := http.DefaultTransport
	if token != "" {
		rt = bearerTransport{token: token}
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "e2e", Version: "0"}, nil)
	cs, err := client.Connect(t.Context(), &sdk.StreamableClientTransport{
		Endpoint: st.srv.URL + "/mcp", HTTPClient: &http.Client{Transport: rt},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	return cs
}

// call runs one tool and decodes its structured content into out when the
// call succeeded; it returns the text and whether the call was an error.
func call(t *testing.T, cs *sdk.ClientSession, tool string, args map[string]any, out any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(t.Context(), &sdk.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	var text strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	if res.IsError || out == nil {
		return text.String(), res.IsError
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatalf("%s structured content %s: %v", tool, b, err)
	}

	return text.String(), false
}

// sees asserts which of the suite's repositories list_repos shows the
// caller. The store is shared with the other suites, so it asserts
// membership and never the whole list.
func sees(t *testing.T, cs *sdk.ClientSession, want map[string]bool) {
	t.Helper()
	got := repoLabels(t, cs)
	for label, visible := range want {
		if slices.Contains(got, label) != visible {
			t.Errorf("list_repos = %v: %s visible = %v, want %v", got, label, !visible, visible)
		}
	}
}

// repoLabels lists what list_repos shows the caller.
func repoLabels(t *testing.T, cs *sdk.ClientSession) []string {
	t.Helper()
	var out struct {
		Repos []struct {
			Repo string `json:"repo"`
		} `json:"repos"`
	}
	if text, isErr := call(t, cs, mcp.ToolListRepos, nil, &out); isErr {
		t.Fatalf("list_repos: %s", text)
	}
	labels := make([]string, 0, len(out.Repos))
	for _, r := range out.Repos {
		labels = append(labels, r.Repo)
	}
	slices.Sort(labels)

	return labels
}

// accessConfig is a manifest with the access block and, when sync is set,
// the Confluence block.
func accessConfig(scopes []string, sync bool) []byte {
	var b strings.Builder
	b.WriteString("---\ndocs_dir: docs\naccess:\n  scopes:\n")
	for _, s := range scopes {
		b.WriteString("    - " + s + "\n")
	}
	if sync {
		b.WriteString("sync:\n  confluence:\n    enabled: true\n    site: " + confluencetest.DefaultURL +
			"\n    space: DOCZ\n    folder: private\n")
	}

	return []byte(b.String())
}

// snapshot is one commit of a repository with the given manifest and one RFC.
func snapshot(head string, manifest []byte, docs map[string][]byte) *ingest.RepoSnapshot {
	snap := &ingest.RepoSnapshot{HeadSHA: head, DefaultBranch: "main", ConfigYAML: manifest}
	paths := make([]string, 0, len(docs))
	for p := range docs {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	for _, p := range paths {
		snap.Blobs = append(snap.Blobs, ingest.BlobEntry{Path: p, GitSHA: head + p, Content: docs[p]})
	}

	return snap
}

// TestE2EMCP is the suite. Its repositories are onboarded once; the
// scenarios share them and run in order, since the export scenario
// re-ingests one.
func TestE2EMCP(t *testing.T) {
	const instID = 9100
	redisURL := startRedis(t)

	// acme/open is readable by everyone; acme/team only by team:read.
	onboard(t, "open", instID, snapshot("o1", accessConfig([]string{publicScope}, false), map[string][]byte{
		"docs/rfc/0001-open.md": doc("RFC-0001", "Open", "# RFC-0001: Open\n\n## Summary\n\nPublic.\n"),
	}))
	onboard(t, "team", instID, snapshot("t1", accessConfig([]string{teamScope}, false), map[string][]byte{
		"docs/rfc/0001-team.md": doc("RFC-0001", "Team", "# RFC-0001: Team\n\n## Summary\n\nTeam only.\n"),
	}))

	provider := fakeProvider{identities: map[string]auth.Identity{
		"eng-code":   {Provider: "github", Subject: "42", Email: "o@example.com", Login: "octocat", Groups: []string{"eng"}},
		"guest-code": {Provider: "github", Subject: "7", Email: "g@example.com", Login: "guest"},
	}}
	scopeMap := map[string][]string{"github:group:eng": {teamScope}}

	t.Run("code flow through the consent page", func(t *testing.T) {
		st := newMCPStack(t, redisURL, stackOptions{provider: provider, scopeMap: scopeMap})
		st.login(t, "eng-code")

		tok := st.redeem(t, st.authorize(t, publicScope+" "+teamScope))
		if tok.TokenType != "Bearer" || tok.Scope != publicScope+" "+teamScope || tok.RefreshToken == "" {
			t.Fatalf("token response = %+v", tok)
		}

		cs := st.mcpClient(t, tok.AccessToken)
		tools, err := cs.ListTools(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(tools.Tools) != 6 {
			t.Errorf("tools = %d, want 6", len(tools.Tools))
		}
		sees(t, cs, map[string]bool{"acme/open": true, "acme/team": true})
		var docOut struct {
			DocID  string `json:"doc_id"`
			Fields struct {
				Kind string `json:"kind"`
			} `json:"fields"`
		}
		text, isErr := call(t, cs, mcp.ToolGetDoc, map[string]any{"repo": "acme/team", "id": "RFC-0001"}, &docOut)
		if isErr || docOut.DocID != "RFC-0001" || docOut.Fields.Kind != mcp.KindRFC || !strings.Contains(text, "Team only.") {
			t.Errorf("get_doc = isError %v %+v %q", isErr, docOut, text)
		}

		// The same token reads the REST API with the limiter in front.
		if res := st.get(t, st.issuer+"/api/v1/repos/acme/team/docs/rfc/RFC-0001", nil); res.status != http.StatusUnauthorized {
			t.Errorf("/api/v1 without a credential = %d, want 401: %s", res.status, res.body)
		}
		if res := st.api(t, "/api/v1/repos", tok.AccessToken); res.status != http.StatusOK {
			t.Errorf("/api/v1/repos with the token = %d, want 200: %s", res.status, res.body)
		}

		// Refresh rotates; reuse of the rotated-out token revokes the family,
		// the newest token included.
		next, oauthErr := st.refresh(t, tok.RefreshToken)
		if oauthErr != "" || next.RefreshToken == tok.RefreshToken || next.AccessToken == "" {
			t.Fatalf("refresh = %q %+v", oauthErr, next)
		}
		if _, oauthErr = st.refresh(t, tok.RefreshToken); oauthErr != oauth.ErrorInvalidGrant {
			t.Errorf("reuse of a rotated-out refresh token = %q, want invalid_grant", oauthErr)
		}
		if _, oauthErr = st.refresh(t, next.RefreshToken); oauthErr != oauth.ErrorInvalidGrant {
			t.Errorf("newest token after the family's revocation = %q, want invalid_grant", oauthErr)
		}
	})

	t.Run("a person without the group is granted public:read only", func(t *testing.T) {
		st := newMCPStack(t, redisURL, stackOptions{provider: provider, scopeMap: scopeMap})
		st.login(t, "guest-code")

		tok := st.redeem(t, st.authorize(t, publicScope+" "+teamScope))
		if tok.Scope != publicScope {
			t.Fatalf("a guest's token carries %q, want %q", tok.Scope, publicScope)
		}
		cs := st.mcpClient(t, tok.AccessToken)
		sees(t, cs, map[string]bool{"acme/open": true, "acme/team": false})
		if text, isErr := call(t, cs, mcp.ToolGetDoc, map[string]any{"repo": "acme/team", "id": "RFC-0001"}, nil); !isErr ||
			!strings.Contains(text, "repo not found") {
			t.Errorf("get_doc on the team repository = isError %v %q, want repo not found", isErr, text)
		}
	})

	t.Run("a trusted issuer's token reads with the scopes it carries", func(t *testing.T) {
		st := newMCPStack(t, redisURL, stackOptions{provider: provider, scopeMap: scopeMap})

		public := st.mcpClient(t, st.trusted.mint(t, st.audience, "agent:public", []string{publicScope}))
		sees(t, public, map[string]bool{"acme/open": true, "acme/team": false})
		team := st.mcpClient(t, st.trusted.mint(t, st.audience, "agent:team", []string{teamScope}))
		sees(t, team, map[string]bool{"acme/open": false, "acme/team": true})

		// A wrong audience and an unknown issuer are refused before the SDK.
		wrong := st.trusted.mint(t, "https://other.example/mcp", "agent:x", []string{publicScope})
		res := st.api(t, "/api/v1/repos", wrong)
		if res.status != http.StatusUnauthorized || !strings.Contains(res.header.Get("WWW-Authenticate"), "resource_metadata=") {
			t.Errorf("wrong audience = %d %q, want 401 with the challenge", res.status, res.header.Get("WWW-Authenticate"))
		}
	})

	t.Run("a none deployment needs no token", func(t *testing.T) {
		st := newMCPStack(t, redisURL, stackOptions{none: true})
		cs := st.mcpClient(t, "")
		sees(t, cs, map[string]bool{"acme/open": true, "acme/team": true})
	})

	t.Run("the export refuses a private repository until it is granted confluence:write", func(t *testing.T) {
		site := confluencetest.New("DOCZ")
		site.RequireAuth("bot@example.com", "token")
		allow := &config.ConfluenceConfig{
			Site: confluencetest.DefaultURL, Email: "bot@example.com", APIToken: "token", Spaces: []string{"DOCZ"},
		}
		svc := export.NewService(testStore, site.HTTPClient(t, "bot@example.com", "token"), allow, "e2e")
		docs := map[string][]byte{
			"docs/rfc/0001-private.md": doc("RFC-0001", "Private", "# RFC-0001: Private\n\nOurs.\n"),
		}

		onboard(t, "private", instID, snapshot("p1", accessConfig([]string{teamScope}, true), docs))
		repo, err := testStore.GetRepo(t.Context(), fixtureOwner, "private")
		if err != nil {
			t.Fatal(err)
		}
		res, err := svc.Run(t.Context(), repo.ID)
		if err != nil {
			t.Fatalf("export: %v", err)
		}
		if res.Status != export.StatusRefused || !strings.Contains(res.Reason, "confluence:write") {
			t.Fatalf("export of a team:read repository = %+v, want refused for its scopes", res)
		}

		onboard(t, "private", instID, snapshot("p2", accessConfig([]string{teamScope, "confluence:write"}, true), docs))
		res, err = svc.Run(t.Context(), repo.ID)
		if err != nil {
			t.Fatalf("export: %v", err)
		}
		if res.Status != export.StatusSucceeded || res.Folder != "private" {
			t.Fatalf("export after the grant = %+v, want succeeded", res)
		}
	})
}
