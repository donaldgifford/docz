package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/go-chi/chi/v5"

	"github.com/donaldgifford/docz-api/api"
	"github.com/donaldgifford/docz-api/internal/auth"
	"github.com/donaldgifford/docz-api/internal/authhttp"
	"github.com/donaldgifford/docz-api/internal/authorize"
	"github.com/donaldgifford/docz-api/internal/queue"
	"github.com/donaldgifford/docz-api/internal/search"
	"github.com/donaldgifford/docz-api/internal/session"
	"github.com/donaldgifford/docz-api/internal/store"
	"github.com/donaldgifford/docz-api/internal/webhook"
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
			Repo: "acme/platform", DocID: "FW-0001", Type: "frameworks",
			Title: "Intro", Status: "Draft", Author: "Jane",
			Snippet: "an <em>intro</em> to frameworks",
		}},
		Facets: map[string]search.FacetMap{
			"type":   {"frameworks": 1},
			"status": {"Draft": 1},
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
		ID: contractSessionID,
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
func buildContractHandler() http.Handler {
	st := seededStore()
	sessions := fakeSessions{}

	r := chi.NewRouter()
	// The /api/v1 gate: session authn composed over the authorize seam, matching
	// runServer. A fixed cookie resolves to a fixed identity via fakeSessions.
	gate := func(next http.Handler) http.Handler {
		return session.Middleware(sessions)(authorize.Middleware(authorize.NewAllReposAuthorizer(st))(next))
	}
	authHandler := authhttp.New(
		auth.NewRegistry([]auth.Provider{stubProvider{}}),
		sessions, fakeUsers{}, []byte(contractStateSecret),
	)
	// Read/search routes plus the gated /auth/session + /auth/logout share the gate.
	NewHandlerWithSearch(st, contractSearcher{}).Mount(r, gate, authHandler.MountAPI)
	// Public auth routes (the signed state is their CSRF guard, not a session).
	authHandler.MountPublic(r)
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
		{name: "listTypes", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/platform/types"},
		{name: "listDocs", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/platform/types/frameworks/docs"},
		{name: "getDoc", method: http.MethodGet, target: "http://localhost/api/v1/repos/acme/platform/types/FW/docs/FW-0001"},
		{name: "searchDocs", method: http.MethodGet, target: "http://localhost/api/v1/search?q=intro"},
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
