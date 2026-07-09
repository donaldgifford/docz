package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/go-chi/chi/v5"

	"github.com/donaldgifford/docz-api/api"
	"github.com/donaldgifford/docz-api/internal/authorize"
)

// This is the OpenAPI contract test: it loads the hand-authored spec
// (api/openapi.yaml, embedded as api.Spec), drives the real chi handler with
// the same in-memory fakes the golden test uses, and validates every
// request/response against the spec via kin-openapi. A green run means the
// server's actual wire behavior matches the published contract (DESIGN-0002);
// drift in a DTO field, envelope key, status code, or content-type fails here.

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

// buildContractHandler wires the read+search routes with the in-memory fakes
// exactly as main mounts them behind the authorize seam, so validation runs
// against the production handler and its serialization.
func buildContractHandler() http.Handler {
	st := seededStore()
	r := chi.NewRouter()
	NewHandlerWithSearch(st, contractSearcher{}).
		Mount(r, authorize.Middleware(authorize.NewAllReposAuthorizer(st)))
	return r
}

// validateRoundTrip serves one request in-process and validates both the
// request and the response against the spec. Security is a no-op here: the
// spec marks /api/v1 operations as sessionCookie-protected, but the auth
// middleware is exercised by its own tests — the contract test asserts schemas,
// not authentication.
func validateRoundTrip(t *testing.T, router routers.Router, h http.Handler, method, target string) {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, target, http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	res := rec.Result()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	_ = res.Body.Close()

	route, pathParams, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("spec has no route for %s %s: %v", method, target, err)
	}

	opts := &openapi3filter.Options{
		MultiError:         true,
		AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
	}
	reqInput := &openapi3filter.RequestValidationInput{
		Request:    req,
		PathParams: pathParams,
		Route:      route,
		Options:    opts,
	}
	if verr := openapi3filter.ValidateRequest(t.Context(), reqInput); verr != nil {
		t.Errorf("request validation failed for %s %s: %v", method, target, verr)
	}

	resInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: reqInput,
		Status:                 res.StatusCode,
		Header:                 res.Header,
		Options:                opts,
	}
	resInput.SetBodyBytes(body)
	if verr := openapi3filter.ValidateResponse(t.Context(), resInput); verr != nil {
		t.Errorf("response validation failed for %s %s (status %d): %v", method, target, res.StatusCode, verr)
	}
}

func TestOpenAPIContract(t *testing.T) {
	router := loadContractSpec(t)
	h := buildContractHandler()

	cases := []struct {
		name   string
		target string
	}{
		{"listRepos", "http://localhost/api/v1/repos"},
		{"getRepo", "http://localhost/api/v1/repos/acme/platform"},
		{"listTypes", "http://localhost/api/v1/repos/acme/platform/types"},
		{"listDocs", "http://localhost/api/v1/repos/acme/platform/types/frameworks/docs"},
		{"getDoc", "http://localhost/api/v1/repos/acme/platform/types/FW/docs/FW-0001"},
		{"searchDocs", "http://localhost/api/v1/search?q=intro"},
		{"notFound", "http://localhost/api/v1/repos/acme/missing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validateRoundTrip(t, router, h, http.MethodGet, tc.target)
		})
	}
}
