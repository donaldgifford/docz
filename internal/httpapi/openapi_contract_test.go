package httpapi

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/go-chi/chi/v5"

	"github.com/donaldgifford/docz/v2/api"
	"github.com/donaldgifford/docz/v2/internal/auth"
	"github.com/donaldgifford/docz/v2/internal/authhttp"
	"github.com/donaldgifford/docz/v2/internal/authorize"
	"github.com/donaldgifford/docz/v2/internal/oauth"
	"github.com/donaldgifford/docz/v2/internal/queue"
	"github.com/donaldgifford/docz/v2/internal/search"
	"github.com/donaldgifford/docz/v2/internal/session"
	"github.com/donaldgifford/docz/v2/internal/store"
	"github.com/donaldgifford/docz/v2/internal/webhook"
)

// This is the OpenAPI contract test: it loads the hand-authored spec
// (api/openapi.yaml, embedded as api.Spec), drives the real chi handler stack
// with in-memory fakes, and validates every request/response against the spec
// via kin-openapi. A green run means the server's actual wire behavior matches
// the published contract (DESIGN-0002); drift in a DTO field, envelope key,
// status code, content-type, or a required header fails here. It is the single
// wire-contract owner (the golden fixtures were retired at parity in Phase 2).

// Fixed secrets/ids the fakes and request builders share. Keeping them constant
// makes the callback (signed state) and webhook (HMAC) cases deterministic.
const (
	contractStateSecret   = "contract-state-secret"
	contractWebhookSecret = "contract-webhook-secret"
	contractSessionID     = "contract-session"
	// sessionCookieName mirrors the unexported session.cookieName; the gated
	// requests carry it so the real session middleware resolves an identity.
	sessionCookieName = "docz_session"
)

// contractSearcher returns one fixed result so the /search contract is
// deterministic (the real Meilisearch response is exercised by the search
// integration test).
type contractSearcher struct{}

func (contractSearcher) Search(context.Context, *search.SearchParams) (search.SearchResult, error) {
	return search.SearchResult{
		Query:          "intro",
		EstimatedTotal: 1,
		Hits: []search.SearchHit{{
			Source: search.SourceDoc, Repo: "acme/platform", DocID: "FW-0001",
			Type: "frameworks", Title: "Intro", Path: "docs/frameworks/0001-intro.md",
			Status: "Draft", Author: "Jane",
			// Real dates, not zero values: the schema types both as strings,
			// so "" would satisfy it without ever exercising the spellings
			// consumers generate against.
			Created: "2026-01-15", UpdatedAt: "2025-06-22T18:04:11Z",
			Snippet: "an <em>intro</em> to frameworks",
		}},
		Facets: map[string]search.FacetMap{
			"type":   {"frameworks": 1},
			"status": {"Draft": 1},
			"source": {"doc": 1},
		},
	}, nil
}

// fakeSessions stands in for the Redis session store. It satisfies both the
// session middleware's lookuper (Lookup) and authhttp's sessionStore (Issue/
// Revoke/SetCookie/ClearCookie), so the gated auth routes run their real
// middleware and the callback flow completes without a live Redis.
type fakeSessions struct{}

func (fakeSessions) Lookup(context.Context, string) (session.Session, error) {
	return session.Session{
		ID:  contractSessionID,
		Via: session.ViaCookie,
		Identity: auth.Identity{
			Provider: "github", Subject: "42",
			Email: "jane@example.com", Login: "jane",
		},
	}, nil
}

func (fakeSessions) Issue(context.Context, *auth.Identity) (string, error) {
	return contractSessionID, nil
}
func (fakeSessions) Revoke(context.Context, string) error  { return nil }
func (fakeSessions) SetCookie(http.ResponseWriter, string) {}
func (fakeSessions) ClearCookie(http.ResponseWriter)       {}

// fakeUsers stands in for the durable users table; UpsertUser is a no-op that
// returns a fixed id so the callback flow proceeds.
type fakeUsers struct{}

func (fakeUsers) UpsertUser(context.Context, store.UserInput) (int64, error) { return 1, nil }

// stubProvider is a stand-in auth provider: AuthCodeURL yields a redirect target
// so login asserts a 302, and Exchange returns a fixed identity so callback
// completes without a live OAuth/OIDC server.
type stubProvider struct{}

func (stubProvider) Name() string { return "github" }
func (stubProvider) AuthCodeURL(state string) string {
	return "https://provider.example/authorize?state=" + url.QueryEscape(state)
}

func (stubProvider) Exchange(context.Context, string) (*auth.Identity, error) {
	return &auth.Identity{
		Provider: "github", Subject: "42", Email: "jane@example.com", Login: "jane",
	}, nil
}

// fakeWebhookStore satisfies the webhook handler's store surface. The contract
// case sends a ping event, which routes to a no-op, so only RecordDelivery is
// exercised (returning isNew=true so the delivery is processed to 202).
type fakeWebhookStore struct{}

func (fakeWebhookStore) RecordDelivery(context.Context, string, string) (bool, error) {
	return true, nil
}

func (fakeWebhookStore) UpsertInstallation(context.Context, store.InstallationInput) error {
	return nil
}
func (fakeWebhookStore) DeleteInstallation(context.Context, int64) error { return nil }

func (fakeWebhookStore) ListRepoIDsByInstallation(context.Context, int64) ([]int64, error) {
	return nil, nil
}

func (fakeWebhookStore) GetRepo(context.Context, string, string) (store.Repo, error) {
	return store.Repo{}, nil
}
func (fakeWebhookStore) DeleteRepo(context.Context, string, string) (int64, error) { return 0, nil }

// fakeEnqueuer satisfies the webhook handler's enqueuer; ingest jobs are dropped.
type fakeEnqueuer struct{}

func (fakeEnqueuer) EnqueueIngest(context.Context, *queue.IngestJob) error { return nil }

// loadContractSpec loads and validates the embedded spec, then builds a
// spec-derived router. Validation enforces OAS 3.1 strictness, so a malformed
// spec (e.g. an info.summary or a bare const) fails before any request runs.
func loadContractSpec(t *testing.T) routers.Router {
	t.Helper()
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(api.Spec)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("spec validation: %v", err)
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("build router: %v", err)
	}
	return router
}

// buildContractHandler wires the whole HTTP surface with in-memory fakes exactly
// as main mounts it: the read/search routes and the two gated auth endpoints
// behind the session+authorize gate, the public auth redirects on the root
// router, and the HMAC webhook receiver outside the gate. Validation therefore
// runs against the production handlers and their real middleware.
func buildContractHandler() http.Handler { return contractHandler(false) }

// The bearer path of the gate (DESIGN-0023 §6): docz-api's own issuer with a
// test key, so the contract test mints tokens the verifier accepts.
const (
	contractIssuer   = "https://docz.example"
	contractAudience = contractIssuer + "/mcp"
	contractMetadata = contractIssuer + "/.well-known/oauth-protected-resource"
)

// contractSigningKey is one ES256 key per process; the verifier trusts its
// public half and the tests sign with its private half.
var contractSigningKey = func() *ecdsa.PrivateKey {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return key
}()

// contractVerifier is the verifier the gate runs: docz-api's own key only, no
// trusted issuers, so no discovery request is made.
func contractVerifier() *oauth.Verifier {
	v, err := oauth.NewVerifier(context.Background(), &oauth.VerifierOptions{
		Audience: contractAudience,
		Issuer:   contractIssuer,
		Key:      &contractSigningKey.PublicKey,
	})
	if err != nil {
		panic(err)
	}
	return v
}

// contractToken mints a token for the test subject with the given audience
// and lifetime, so a case can ask for a wrong audience or an expired token.
func contractToken(t *testing.T, audience string, ttl time.Duration) string {
	t.Helper()
	signer, err := oauth.NewSigner(contractIssuer, audience, contractSigningKey, ttl)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signer.Issue(&oauth.Claims{
		Subject: "user:github:42", ClientID: "contract-client",
		Scopes: []string{"public:read"}, Email: "jane@example.com", Login: "jane",
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// contractClientID is the one Client ID Metadata Document the contract's
// fetcher serves, from memory: the flow never fetches over the network.
const (
	contractClientID     = "https://agent.example/oauth.json"
	contractRedirectURI  = "https://agent.example/cb"
	contractPKCEVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	contractChallenge    = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
)

// contractScopeMap grants the fixed session's login one scope beyond
// public:read, so a consent has something to intersect.
var contractScopeMap = map[string][]string{"github:login:jane": {"team:read"}}

// clientDocument serves contractClientID's document without a server.
type clientDocument struct{}

func (clientDocument) RoundTrip(r *http.Request) (*http.Response, error) {
	body := `{"client_id":"` + contractClientID + `","client_name":"Agent","client_uri":"https://agent.example","redirect_uris":["` + contractRedirectURI + `"]}`
	status := http.StatusOK
	if r.URL.String() != contractClientID {
		status, body = http.StatusNotFound, ""
	}
	return &http.Response{
		StatusCode: status, Body: io.NopCloser(strings.NewReader(body)),
		Header: http.Header{"Content-Type": {"application/json"}}, Request: r,
	}, nil
}

// fakeOAuthStore is the authorization server's durable surface in memory;
// the real one is a sqlc query set over Postgres.
type fakeOAuthStore struct {
	fakeUsers
	mu     sync.Mutex
	tokens map[string]*store.RefreshToken
}

func (*fakeOAuthStore) GetUser(context.Context, int64) (store.TokenUser, error) {
	return store.TokenUser{ID: 1, Provider: "github", Subject: "42", Email: "jane@example.com", Login: "jane"}, nil
}

func (f *fakeOAuthStore) InsertRefreshToken(_ context.Context, hash, familyID string, userID int64, clientID, scope string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens[hash] = &store.RefreshToken{TokenHash: hash, FamilyID: familyID, UserID: userID, ClientID: clientID, Scope: scope, IssuedAt: time.Now()}
	return nil
}

func (f *fakeOAuthStore) GetRefreshToken(_ context.Context, hash string) (store.RefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.tokens[hash]; ok {
		return *t, nil
	}
	return store.RefreshToken{}, store.ErrNotFound
}

func (f *fakeOAuthStore) RotateRefreshToken(_ context.Context, oldHash, newHash, scope string) (store.RefreshToken, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	old, ok := f.tokens[oldHash]
	if !ok || old.LastUsedAt != nil || old.RevokedAt != nil {
		return store.RefreshToken{}, false, nil
	}
	now := time.Now()
	old.LastUsedAt = &now
	next := &store.RefreshToken{TokenHash: newHash, FamilyID: old.FamilyID, UserID: old.UserID, ClientID: old.ClientID, Scope: scope, IssuedAt: now}
	f.tokens[newHash] = next
	return *next, true, nil
}

func (f *fakeOAuthStore) RevokeRefreshFamily(_ context.Context, familyID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	for _, t := range f.tokens {
		if t.FamilyID == familyID && t.RevokedAt == nil {
			t.RevokedAt = &now
		}
	}
	return nil
}

func (*fakeOAuthStore) UpsertConsent(context.Context, int64, string, string) error { return nil }

// contractAuthServer builds the authorization server over the contract key,
// an in-memory cache standing in for Redis, and the fakes above.
func contractAuthServer() *oauth.Server {
	signer, err := oauth.NewSigner(contractIssuer, contractAudience, contractSigningKey, 15*time.Minute)
	if err != nil {
		panic(err)
	}
	cache := oauth.NewMemoryCache()
	return oauth.NewServer(&oauth.ServerOptions{
		Issuer: contractIssuer, Audience: contractAudience, SiteURL: "https://site.example",
		Signer:     signer,
		ScopeMap:   map[string][]string{"github:login:jane": {"team:read"}},
		Clients:    oauth.NewClientFetcher(&oauth.ClientFetcherOptions{Cache: cache, Transport: clientDocument{}}),
		Cache:      cache,
		Store:      &fakeOAuthStore{tokens: map[string]*store.RefreshToken{}},
		RefreshTTL: 720 * time.Hour,
		Session: func(ctx context.Context) (oauth.Principal, bool) {
			sess, ok := session.FromContext(ctx)
			if !ok {
				return oauth.Principal{}, false
			}
			return oauth.Principal{SessionID: sess.ID, Identity: sess.Identity, Scopes: sess.Scopes}, true
		},
	})
}

// contractHandler builds the surface in either auth mode. authDisabled mirrors
// runServer under AUTH_PROVIDERS=none: the anonymous injector replaces the
// session middleware, and the public login routes are left unmounted.
func contractHandler(authDisabled bool) http.Handler {
	st := seededStore()
	// A document the export never wrote, so getDoc covers confluence_url "".
	st.docs[1] = append(st.docs[1], store.Document{
		ID: 101, RepoID: 1, Type: "frameworks", DocID: "FW-0002", Title: "Unexported",
		Status: validText("Draft"), Path: "docs/frameworks/0002-unexported.md",
		GitSha: "def", ContentHash: "hash2", RawMd: "# Unexported\n",
	})
	sessions := fakeSessions{}

	r := chi.NewRouter()
	// The /api/v1 gate: session.Authn (a bearer from the contract key, else
	// the cookie, else the anonymous principal) composed over the authorize
	// seam, matching runServer. A fixed cookie resolves to a fixed identity
	// via fakeSessions.
	authn := session.Authn(session.AuthnOptions{
		Sessions:         sessions,
		Verifier:         contractVerifier(),
		Scopes:           func(id *auth.Identity) []string { return authorize.ScopesFor(id, contractScopeMap) },
		Anonymous:        authDisabled,
		ResourceMetadata: contractMetadata,
	})
	gate := func(next http.Handler) http.Handler {
		return authn(authorize.Middleware(authorize.NewAllReposAuthorizer(st))(next))
	}
	authHandler := authhttp.New(
		auth.NewRegistry([]auth.Provider{stubProvider{}}),
		sessions, fakeUsers{}, []byte(contractStateSecret),
	)
	// Read/search routes plus the gated /auth/session + /auth/logout share the gate.
	extras := []func(chi.Router){authHandler.MountAPI}
	var authServer *oauth.Server
	if !authDisabled {
		// The authorization server, as runServer mounts it with MCP on and a
		// login: the consent endpoints behind the gate, the rest public.
		authServer = contractAuthServer()
		extras = append(extras, authServer.MountAPI)
	}
	NewHandlerWithSearch(st, contractSearcher{}).WithConfluenceExport(true).Mount(r, gate, extras...)
	if !authDisabled {
		// Public auth routes (the signed state is their CSRF guard, not a session).
		authHandler.MountPublic(r)
		authServer.MountPublic(r)
	}
	// The webhook receiver sits outside the gate — its HMAC signature is the auth.
	wh := webhook.New([]byte(contractWebhookSecret), fakeWebhookStore{}, fakeEnqueuer{}, nil)
	r.Post("/webhooks/github", wh.ServeHTTP)
	return r
}

// validateRoundTrip validates one request against the spec, serves it in-process
// against the real handler, then validates the response. The request body is
// snapshotted so it survives both ValidateRequest and ServeHTTP. Security is a
// no-op (NoopAuthenticationFunc): the spec marks /api/v1 as sessionCookie-
// protected, but the middleware is exercised for real here (a fixed cookie) and
// the contract test asserts schemas, not the auth mechanism itself.
func validateRoundTrip(t *testing.T, router routers.Router, h http.Handler, req *http.Request) {
	t.Helper()

	route, pathParams, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("spec has no route for %s %s: %v", req.Method, req.URL, err)
	}

	opts := &openapi3filter.Options{
		MultiError:         true,
		AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
	}

	// Snapshot the request body: ValidateRequest and ServeHTTP each consume it.
	var bodyBytes []byte
	if req.Body != nil && req.Body != http.NoBody {
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		_ = req.Body.Close()
	}

	reqInput := &openapi3filter.RequestValidationInput{
		Request:    req,
		PathParams: pathParams,
		Route:      route,
		Options:    opts,
	}
	if bodyBytes != nil {
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}
	if verr := openapi3filter.ValidateRequest(t.Context(), reqInput); verr != nil {
		t.Errorf("request validation failed for %s %s: %v", req.Method, req.URL, verr)
	}

	// Restore the body for the handler.
	if bodyBytes != nil {
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	res := rec.Result()
	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	_ = res.Body.Close()

	resInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: reqInput,
		Status:                 res.StatusCode,
		Header:                 res.Header,
		Options:                opts,
	}
	resInput.SetBodyBytes(respBody)
	if verr := openapi3filter.ValidateResponse(t.Context(), resInput); verr != nil {
		t.Errorf("response validation failed for %s %s (status %d): %v",
			req.Method, req.URL, res.StatusCode, verr)
	}
}

// contractRequest builds a bodyless request carrying the session cookie so gated
// /api/v1 routes pass the session middleware. The cookie is harmless on the
// public routes.
func contractRequest(t *testing.T, method, target string) *http.Request {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, target, http.NoBody)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: contractSessionID})
	return req
}

// callbackRequest builds the /auth/callback GET with a validly signed state, so
// the handler verifies the state, exchanges via stubProvider, and redirects 302.
func callbackRequest(t *testing.T) *http.Request {
	t.Helper()
	state, err := auth.EncodeState([]byte(contractStateSecret), "github")
	if err != nil {
		t.Fatalf("encode state: %v", err)
	}
	target := "http://localhost/auth/callback?code=test-code&state=" + url.QueryEscape(state)
	return contractRequest(t, http.MethodGet, target)
}

// webhookRequest builds the /webhooks/github POST with a valid HMAC over a ping
// fixture body and the three required GitHub headers, so verification passes and
// the handler returns 202.
func webhookRequest(t *testing.T) *http.Request {
	t.Helper()
	body := []byte(`{"zen":"contract","hook_id":1}`)
	mac := hmac.New(sha256.New, []byte(contractWebhookSecret))
	_, _ = mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"http://localhost/webhooks/github", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-GitHub-Delivery", "contract-delivery-1")
	return req
}

func TestOpenAPIContract(t *testing.T) {
	router := loadContractSpec(t)
	h := buildContractHandler()

	// Most cases are a simple GET/POST at a target; callback and webhook need a
	// custom builder (signed state / HMAC), so build overrides method+target.
	cases := []struct {
		name   string
		method string
		target string
		build  func(*testing.T) *http.Request
	}{
		{name: "listRepos", method: http.MethodGet, target: "http://localhost/api/v1/repos"},
		{name: "getRepo", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/platform"},
		{name: "getRepoIndex", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/platform/index"},
		{name: "getRepoIndexMissing", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/bare/index"},
		{name: "getRepoChangelog", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/platform/changelog"},
		{name: "getRepoChangelogMissing", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/bare/changelog"},
		{name: "listRepoPages", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/platform/pages"},
		{name: "listRepoPagesEmpty", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/bare/pages"},
		// The percent-encoded-as-one-segment spelling is the documented client
		// form (DESIGN-0004 OQ-2a); the literal-slash spelling routes the same
		// server-side and is covered by the handler tests.
		{name: "getRepoPage", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/platform/pages/guides%2Fsetup.md"},
		{name: "getRepoPageDirectory", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/platform/pages/guides"},
		{name: "getRepoPageMissing", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/platform/pages/nope.md"},
		{name: "listTypes", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/platform/types"},
		{name: "listDocs", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/platform/types/frameworks/docs"},
		{name: "getDoc", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/platform/types/FW/docs/FW-0001"},
		{name: "getDocNoConfluence", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/platform/types/FW/docs/FW-0002"},
		{name: "getRepoConfluence", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/platform/confluence"},
		{name: "getRepoConfluenceNever", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/bare/confluence"},
		{name: "searchDocs", method: http.MethodGet, target: "http://localhost/api/v1/search?q=intro"},
		// A sorted search validates the sort enum on the request side. The
		// rejected case is deliberately absent: an out-of-enum value fails
		// kin-openapi's request validation before the handler runs, so it
		// would prove the spec rather than the handler — the 400 envelope is
		// covered by the httpapi unit test instead.
		{
			name: "searchDocsSorted", method: http.MethodGet,
			target: "http://localhost/api/v1/search?q=intro&sort=updated_at:desc",
		},
		{
			name: "searchDocsSource", method: http.MethodGet,
			target: "http://localhost/api/v1/search?q=intro&source=doc",
		},
		{name: "notFound", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/missing"},
		{name: "getSession", method: http.MethodGet, target: "http://localhost/api/v1/auth/session"},
		{name: "logout", method: http.MethodPost, target: "http://localhost/api/v1/auth/logout"},
		{name: "login", method: http.MethodGet, target: "http://localhost/auth/login?provider=github"},
		{name: "callback", build: callbackRequest},
		{name: "githubWebhook", build: webhookRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.build
			var r *http.Request
			if req != nil {
				r = req(t)
			} else {
				r = contractRequest(t, tc.method, tc.target)
			}
			validateRoundTrip(t, router, h, r)
		})
	}
}

// bearerRequest is contractRequest with a token instead of the cookie.
func bearerRequest(t *testing.T, target, token string) *http.Request {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

// Every read route with a bearer token in place of the cookie serves the
// same contract (DESIGN-0023 §6): the token is the session, nothing else
// changes on the wire.
func TestOpenAPIContractBearer(t *testing.T) {
	router := loadContractSpec(t)
	h := buildContractHandler()
	token := contractToken(t, contractAudience, time.Minute)

	for _, target := range []string{
		"http://localhost/api/v1/repos",
		"http://localhost/api/v1/repos/acme/platform",
		"http://localhost/api/v1/repos/acme/platform/index",
		"http://localhost/api/v1/repos/acme/platform/changelog",
		"http://localhost/api/v1/repos/acme/platform/pages",
		"http://localhost/api/v1/repos/acme/platform/pages/guides%2Fsetup.md",
		"http://localhost/api/v1/repos/acme/platform/types",
		"http://localhost/api/v1/repos/acme/platform/types/frameworks/docs",
		"http://localhost/api/v1/repos/acme/platform/types/FW/docs/FW-0001",
		"http://localhost/api/v1/repos/acme/platform/confluence",
		"http://localhost/api/v1/search?q=intro",
		"http://localhost/api/v1/auth/session",
	} {
		t.Run(strings.TrimPrefix(target, "http://localhost"), func(t *testing.T) {
			validateRoundTrip(t, router, h, bearerRequest(t, target, token))
		})
	}
}

// A token the verifier refuses is a 401 with the RFC 9728 challenge, and it
// never falls through to a cookie on the same request (DESIGN-0023 §6).
func TestOpenAPIContractBadBearer(t *testing.T) {
	router := loadContractSpec(t)
	h := buildContractHandler()
	wantChallenge := `Bearer resource_metadata="` + contractMetadata + `", scope="public:read"`

	cases := []struct {
		name  string
		token string
	}{
		{"garbage", "not-a-jwt"},
		{"wrong audience", contractToken(t, "https://other.example/mcp", time.Minute)},
		{"expired", contractToken(t, contractAudience, -2*time.Minute)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := bearerRequest(t, "http://localhost/api/v1/repos", tc.token)
			req.AddCookie(&http.Cookie{Name: "docz_session", Value: contractSessionID})
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 (body %q)", rr.Code, rr.Body.String())
			}
			if got := rr.Header().Get("WWW-Authenticate"); got != wantChallenge {
				t.Errorf("WWW-Authenticate = %q, want %q", got, wantChallenge)
			}
			validateRoundTrip(t, router, h, bearerRequest(t, "http://localhost/api/v1/repos", tc.token))
		})
	}
}

// The session response for a token caller names how it authenticated, its
// scopes, the client, and the issuer; docz-api's own subject resolves to the
// person the token was minted for.
func TestSessionReportsTokenPrincipal(t *testing.T) {
	h := buildContractHandler()
	req := bearerRequest(t, "http://localhost/api/v1/auth/session", contractToken(t, contractAudience, time.Minute))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rr.Code, rr.Body.String())
	}

	var got struct {
		Provider string   `json:"provider"`
		Subject  string   `json:"subject"`
		Via      string   `json:"via"`
		Scopes   []string `json:"scopes"`
		ClientID string   `json:"client_id"`
		Issuer   string   `json:"issuer"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode session body %q: %v", rr.Body.String(), err)
	}
	if got.Provider != "github" || got.Subject != "42" || got.Via != "token" ||
		got.ClientID != "contract-client" || got.Issuer != contractIssuer ||
		len(got.Scopes) != 1 || got.Scopes[0] != "public:read" {
		t.Errorf("session = %+v, want github/42 via token for contract-client from %s with [public:read]", got, contractIssuer)
	}
}

// formRequest is a form-encoded POST with no cookie: the code or the token
// is the credential.
func formRequest(t *testing.T, target string, form url.Values) *http.Request {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

// serveJSON runs req and decodes the body into out, returning the status.
func serveJSON(t *testing.T, h http.Handler, req *http.Request, out any) int {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if out != nil && rr.Code == http.StatusOK {
		if err := json.Unmarshal(rr.Body.Bytes(), out); err != nil {
			t.Fatalf("decode %q: %v", rr.Body.String(), err)
		}
	}
	return rr.Code
}

// authorizeAndConsent runs the authorization request and the consent
// decision for the fixed session, returning the code.
func authorizeAndConsent(t *testing.T, router routers.Router, h http.Handler) string {
	t.Helper()
	q := url.Values{
		"response_type": {"code"}, "client_id": {contractClientID}, "redirect_uri": {contractRedirectURI},
		"code_challenge": {contractChallenge}, "code_challenge_method": {"S256"},
		"resource": {contractAudience}, "scope": {"public:read team:read"}, "state": {"xyz"},
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost/oauth/authorize?"+q.Encode(), http.NoBody))
	if rr.Code != http.StatusFound {
		t.Fatalf("authorize = %d %s", rr.Code, rr.Body.String())
	}
	loc, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	id := loc.Query().Get("authz")
	if id == "" {
		t.Fatalf("authorize redirected to %s, want the consent page", loc)
	}

	target := "http://localhost/api/v1/oauth/authorizations/" + id
	validateRoundTrip(t, router, h, contractRequest(t, http.MethodGet, target))

	decide := contractRequest(t, http.MethodPost, target)
	decide.Body = io.NopCloser(strings.NewReader(`{"decision":"allow"}`))
	decide.Header.Set("Content-Type", "application/json")
	var res struct {
		RedirectTo string `json:"redirect_to"`
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, decide)
	if rr.Code != http.StatusOK {
		t.Fatalf("decide = %d %s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	cb, err := url.Parse(res.RedirectTo)
	if err != nil {
		t.Fatal(err)
	}
	return cb.Query().Get("code")
}

// The authorization server's wire contract (DESIGN-0023 §5): the three
// metadata documents, the consent endpoints, both grants, and the revoke,
// each validated against the spec.
func TestOpenAPIContractOAuth(t *testing.T) {
	router := loadContractSpec(t)
	h := buildContractHandler()

	for _, target := range []string{
		"http://localhost/.well-known/oauth-protected-resource",
		"http://localhost/.well-known/oauth-authorization-server",
		"http://localhost/.well-known/jwks.json",
	} {
		t.Run(strings.TrimPrefix(target, "http://localhost"), func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
			validateRoundTrip(t, router, h, req)
		})
	}

	t.Run("consent, code, refresh, revoke", func(t *testing.T) {
		code := authorizeAndConsent(t, router, h)

		codeForm := url.Values{
			"grant_type": {"authorization_code"}, "code": {code}, "code_verifier": {contractPKCEVerifier},
			"client_id": {contractClientID}, "redirect_uri": {contractRedirectURI}, "resource": {contractAudience},
		}
		validateRoundTrip(t, router, h, formRequest(t, "http://localhost/oauth/token", codeForm))
	})

	t.Run("decision validated", func(t *testing.T) {
		code := authorizeAndConsent(t, router, h)

		// The decision endpoint itself, through the validator.
		id2 := authorizeOnly(t, h)
		validateRoundTrip(t, router, h, contractRequest(t, http.MethodGet, "http://localhost/api/v1/oauth/authorizations/"+id2))
		decide := contractRequest(t, http.MethodPost, "http://localhost/api/v1/oauth/authorizations/"+id2)
		decide.Body = io.NopCloser(strings.NewReader(`{"decision":"deny"}`))
		decide.Header.Set("Content-Type", "application/json")
		validateRoundTrip(t, router, h, decide)

		var tok struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		codeForm := url.Values{
			"grant_type": {"authorization_code"}, "code": {code}, "code_verifier": {contractPKCEVerifier},
			"client_id": {contractClientID}, "redirect_uri": {contractRedirectURI}, "resource": {contractAudience},
		}
		if status := serveJSON(t, h, formRequest(t, "http://localhost/oauth/token", codeForm), &tok); status != http.StatusOK {
			t.Fatalf("token = %d", status)
		}

		// The minted token reads the API like a cookie does.
		validateRoundTrip(t, router, h, bearerRequest(t, "http://localhost/api/v1/repos", tok.AccessToken))
		var sess struct {
			Via    string   `json:"via"`
			Scopes []string `json:"scopes"`
		}
		if status := serveJSON(t, h, bearerRequest(t, "http://localhost/api/v1/auth/session", tok.AccessToken), &sess); status != http.StatusOK {
			t.Fatalf("session = %d", status)
		}
		if sess.Via != "token" || len(sess.Scopes) != 2 {
			t.Errorf("session via token = %+v, want via token with both scopes", sess)
		}

		// The refresh grant, then the revoke, each against the spec.
		refreshForm := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tok.RefreshToken}}
		validateRoundTrip(t, router, h, formRequest(t, "http://localhost/oauth/token", refreshForm))

		// The rotated-out token is now an RFC 6749 error, in the spec's shape.
		validateRoundTrip(t, router, h, formRequest(t, "http://localhost/oauth/token", refreshForm))

		validateRoundTrip(t, router, h, formRequest(t, "http://localhost/oauth/revoke", url.Values{"token": {tok.RefreshToken}}))
	})

	t.Run("errors in the spec's shape", func(t *testing.T) {
		// An unknown grant_type or a missing token fails kin-openapi's request
		// validation before the handler runs, so those would prove the spec
		// rather than the handler (the search sort enum precedent); the
		// package tests cover their RFC 6749 bodies. The consent 404 and a
		// bad code are reachable through the validator.
		validateRoundTrip(t, router, h, contractRequest(t, http.MethodGet, "http://localhost/api/v1/oauth/authorizations/nope"))
		validateRoundTrip(t, router, h, formRequest(t, "http://localhost/oauth/token", url.Values{
			"grant_type": {"authorization_code"}, "code": {"nope"}, "code_verifier": {contractPKCEVerifier},
			"client_id": {contractClientID}, "redirect_uri": {contractRedirectURI}, "resource": {contractAudience},
		}))
	})
}

// authorizeOnly runs the authorization request and returns the pending id.
func authorizeOnly(t *testing.T, h http.Handler) string {
	t.Helper()
	q := url.Values{
		"response_type": {"code"}, "client_id": {contractClientID}, "redirect_uri": {contractRedirectURI},
		"code_challenge": {contractChallenge}, "code_challenge_method": {"S256"},
		"resource": {contractAudience}, "scope": {"public:read"}, "state": {"xyz"},
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost/oauth/authorize?"+q.Encode(), http.NoBody))
	loc, err := url.Parse(rr.Header().Get("Location"))
	if err != nil || rr.Code != http.StatusFound {
		t.Fatalf("authorize = %d %s", rr.Code, rr.Body.String())
	}
	return loc.Query().Get("authz")
}

// Under none-mode the authorization server is absent: the metadata, the
// endpoints, and the consent routes all 404, while /api/v1 still serves.
func TestNoAuthModeOmitsAuthorizationServer(t *testing.T) {
	h := contractHandler(true)
	for _, target := range []string{
		"http://localhost/.well-known/oauth-authorization-server",
		"http://localhost/oauth/authorize?response_type=code",
		"http://localhost/api/v1/oauth/authorizations/x",
	} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", target, rr.Code)
		}
	}
}

// none-mode (AUTH_PROVIDERS=none) must serve the same wire contract with no
// cookie at all: same schemas, same status codes, no spec change. That is what
// makes it usable as a first-setup shape — the docz-site talks to it unmodified.
func TestOpenAPIContractNoAuthMode(t *testing.T) {
	router := loadContractSpec(t)
	h := contractHandler(true)

	cases := []struct {
		name   string
		method string
		target string
	}{
		{"listRepos", http.MethodGet, "http://localhost/api/v1/repos"},
		{"getRepo", http.MethodGet, "http://localhost/api/v1/repos/acme/platform"},
		{"getDoc", http.MethodGet, "http://localhost/api/v1/repos/acme/platform/types/FW/docs/FW-0001"},
		{"searchDocs", http.MethodGet, "http://localhost/api/v1/search?q=intro"},
		{"getSession", http.MethodGet, "http://localhost/api/v1/auth/session"},
		{"logout", http.MethodPost, "http://localhost/api/v1/auth/logout"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Deliberately no session cookie: none-mode must never 401.
			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.target, http.NoBody)
			validateRoundTrip(t, router, h, req)
		})
	}
}

// The synthetic identity is what /api/v1/auth/session reports under none-mode.
// The docz-site drives its login UI off a 401 here, so a 200 with a recognizable
// anonymous provider is what keeps the login panel from appearing at all.
func TestNoAuthModeReportsAnonymousIdentity(t *testing.T) {
	h := contractHandler(true)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"http://localhost/api/v1/auth/session", http.NoBody)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with no cookie under none-mode", rr.Code)
	}
	var got struct {
		Provider string `json:"provider"`
		Subject  string `json:"subject"`
		Via      string `json:"via"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode session body %q: %v", rr.Body.String(), err)
	}
	if got.Provider != session.AnonymousProvider || got.Subject != session.AnonymousSubject || got.Via != "anonymous" {
		t.Errorf("identity = %+v, want provider %q subject %q via anonymous",
			got, session.AnonymousProvider, session.AnonymousSubject)
	}
}

// With no provider to redirect to, the login route is absent rather than
// mounted-and-erroring — the same absent-route precedent as search with no
// searcher configured.
func TestNoAuthModeOmitsLoginRoutes(t *testing.T) {
	h := contractHandler(true)
	for _, target := range []string{
		"http://localhost/auth/login?provider=github",
		"http://localhost/auth/callback?code=x&state=y",
	} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", target, rr.Code)
		}
	}
}
