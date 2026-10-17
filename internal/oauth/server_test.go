package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/donaldgifford/docz/v2/internal/auth"
	"github.com/donaldgifford/docz/v2/internal/store"
)

// fakeStore is the durable surface in memory.
type fakeStore struct {
	mu       sync.Mutex
	users    map[string]store.TokenUser
	tokens   map[string]*store.RefreshToken
	consents map[string]string
	nextID   int64
	now      func() time.Time
}

func newFakeStore(now func() time.Time) *fakeStore {
	return &fakeStore{users: map[string]store.TokenUser{}, tokens: map[string]*store.RefreshToken{}, consents: map[string]string{}, now: now}
}

func (f *fakeStore) UpsertUser(_ context.Context, in store.UserInput) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := in.Provider + ":" + in.Subject
	u, ok := f.users[key]
	if !ok {
		f.nextID++
		u = store.TokenUser{ID: f.nextID, Provider: in.Provider, Subject: in.Subject}
	}
	u.Email, u.Login = in.Email, in.Login
	f.users[key] = u

	return u.ID, nil
}

func (f *fakeStore) GetUser(_ context.Context, id int64) (store.TokenUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}

	return store.TokenUser{}, store.ErrNotFound
}

func (f *fakeStore) InsertRefreshToken(_ context.Context, hash, familyID string, userID int64, clientID, scope string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens[hash] = &store.RefreshToken{TokenHash: hash, FamilyID: familyID, UserID: userID, ClientID: clientID, Scope: scope, IssuedAt: f.now()}

	return nil
}

func (f *fakeStore) GetRefreshToken(_ context.Context, hash string) (store.RefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tokens[hash]
	if !ok {
		return store.RefreshToken{}, store.ErrNotFound
	}

	return *t, nil
}

func (f *fakeStore) RotateRefreshToken(_ context.Context, oldHash, newHash, scope string) (store.RefreshToken, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	old, ok := f.tokens[oldHash]
	if !ok || old.LastUsedAt != nil || old.RevokedAt != nil {
		return store.RefreshToken{}, false, nil
	}
	used := f.now()
	old.LastUsedAt = &used
	next := &store.RefreshToken{TokenHash: newHash, FamilyID: old.FamilyID, UserID: old.UserID, ClientID: old.ClientID, Scope: scope, IssuedAt: used}
	f.tokens[newHash] = next

	return *next, true, nil
}

func (f *fakeStore) RevokeRefreshFamily(_ context.Context, familyID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.now()
	for _, t := range f.tokens {
		if t.FamilyID == familyID && t.RevokedAt == nil {
			t.RevokedAt = &now
		}
	}

	return nil
}

func (f *fakeStore) UpsertConsent(_ context.Context, userID int64, clientID, scope string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.consents[itoa(userID)+"|"+clientID] = scope

	return nil
}

// flow is a Server mounted on a router with a fake session, a fake store,
// and a client metadata server, plus the knobs a test turns.
type flow struct {
	srv       *Server
	router    chi.Router
	store     *fakeStore
	clientID  string
	principal Principal
	hasSess   bool
	now       time.Time
	verifier  *Verifier
}

func newFlow(t *testing.T, scopeMap map[string][]string) *flow {
	t.Helper()

	key := newKey(t)
	signer, err := NewSigner(testIssuer, testAudience, key, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	fl := &flow{now: time.Now()}
	signer.now = func() time.Time { return fl.now }

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth.json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(goodDocument(fl.clientID)))
	})
	cs := httptest.NewTLSServer(mux)
	t.Cleanup(cs.Close)
	fl.clientID = cs.URL + "/oauth.json"

	fl.store = newFakeStore(func() time.Time { return fl.now })
	fl.principal = Principal{
		SessionID: "sess-1",
		Identity:  auth.Identity{Provider: "github", Subject: "42", Email: "o@example.com", Login: "octocat"},
		Scopes:    []string{"public:read", "team:read"},
	}
	fl.hasSess = true

	cache := NewMemoryCache()
	cache.now = func() time.Time { return fl.now }
	fl.srv = NewServer(&ServerOptions{
		Issuer: testIssuer, Audience: testAudience, SiteURL: "https://site.example",
		Signer: signer, ScopeMap: scopeMap,
		Clients:    NewClientFetcher(&ClientFetcherOptions{Transport: cs.Client().Transport}),
		Cache:      cache,
		Store:      fl.store,
		RefreshTTL: 720 * time.Hour,
		Session: func(context.Context) (Principal, bool) {
			return fl.principal, fl.hasSess
		},
	})
	fl.srv.now = func() time.Time { return fl.now }

	fl.verifier, err = NewVerifier(t.Context(), &VerifierOptions{Audience: testAudience, Issuer: testIssuer, Key: &key.PublicKey})
	if err != nil {
		t.Fatal(err)
	}

	fl.router = chi.NewRouter()
	fl.srv.MountPublic(fl.router)
	fl.router.Route("/api/v1", func(r chi.Router) { fl.srv.MountAPI(r) })

	return fl
}

func (fl *flow) do(t *testing.T, method, target, body string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader = http.NoBody
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequestWithContext(t.Context(), method, target, rd)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	fl.router.ServeHTTP(rec, req)

	return rec
}

const (
	verifier  = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" // s256(verifier), RFC 7636 §B
	redirect  = "https://agent.example/cb"
)

// authorizeURL builds a valid authorization request with overrides.
func (fl *flow) authorizeURL(override url.Values) string {
	q := url.Values{
		"response_type": {"code"}, "client_id": {fl.clientID}, "redirect_uri": {redirect},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"},
		"resource": {testAudience}, "scope": {"public:read team:read"}, "state": {"xyz"},
	}
	for k, v := range override {
		if len(v) == 1 && v[0] == "" {
			q.Del(k)
		} else {
			q[k] = v
		}
	}

	return AuthorizePath + "?" + q.Encode()
}

// authorize runs GET /oauth/authorize and returns the pending id.
func (fl *flow) authorize(t *testing.T) string {
	t.Helper()
	rec := fl.do(t, http.MethodGet, fl.authorizeURL(nil), "", nil)
	if rec.Code != http.StatusFound {
		t.Fatalf("authorize = %d %s", rec.Code, rec.Body.String())
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(loc.String(), "https://site.example/oauth/consent?authz=") {
		t.Fatalf("authorize redirected to %s", loc)
	}

	return loc.Query().Get("authz")
}

// consent reads then decides, returning the code from redirect_to.
func (fl *flow) consent(t *testing.T, id, decision string) (*url.URL, int) {
	t.Helper()
	rec := fl.do(t, http.MethodGet, "/api/v1/oauth/authorizations/"+id, "", nil)
	if rec.Code != http.StatusOK {
		return nil, rec.Code
	}
	rec = fl.do(
		t,
		http.MethodPost,
		"/api/v1/oauth/authorizations/"+id,
		`{"decision":"`+decision+`"}`,
		map[string]string{"Content-Type": "application/json"},
	)
	if rec.Code != http.StatusOK {
		return nil, rec.Code
	}
	var res DecisionResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(res.RedirectTo)
	if err != nil {
		t.Fatal(err)
	}

	return u, rec.Code
}

func (fl *flow) tokenRequest(t *testing.T, form url.Values) (TokenResponse, oauthError, int) {
	t.Helper()
	rec := fl.do(t, http.MethodPost, TokenPath, form.Encode(), map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	var (
		ok  TokenResponse
		bad oauthError
	)
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &ok); err != nil {
			t.Fatal(err)
		}
	} else if err := json.Unmarshal(rec.Body.Bytes(), &bad); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("token response Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
	}

	return ok, bad, rec.Code
}

func (fl *flow) codeForm(code string) url.Values {
	return url.Values{
		"grant_type": {GrantAuthorizationCode}, "code": {code}, "code_verifier": {verifier},
		"client_id": {fl.clientID}, "redirect_uri": {redirect}, "resource": {testAudience},
	}
}

func TestFlowHappyPath(t *testing.T) {
	t.Parallel()

	fl := newFlow(t, map[string][]string{"github:group:eng": {"team:read"}})

	id := fl.authorize(t)
	cb, status := fl.consent(t, id, "allow")
	if status != http.StatusOK {
		t.Fatalf("consent = %d", status)
	}
	if cb.Query().Get("state") != "xyz" || cb.Query().Get("iss") != testIssuer || cb.Query().Get("code") == "" {
		t.Fatalf("redirect_to = %s", cb)
	}
	if cb.Scheme+"://"+cb.Host+cb.Path != redirect {
		t.Errorf("redirect_to base = %s, want %s", cb, redirect)
	}

	tok, bad, status := fl.tokenRequest(t, fl.codeForm(cb.Query().Get("code")))
	if status != http.StatusOK {
		t.Fatalf("token = %d %+v", status, bad)
	}
	if tok.TokenType != "Bearer" || tok.ExpiresIn != 900 || tok.Scope != "public:read team:read" || tok.RefreshToken == "" {
		t.Errorf("token response = %+v", tok)
	}

	claims, err := fl.verifier.Verify(t.Context(), tok.AccessToken)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Subject != "user:github:42" || claims.ClientID != fl.clientID || claims.Email != "o@example.com" || claims.Login != "octocat" ||
		len(claims.Scopes) != 2 {
		t.Errorf("claims = %+v", claims)
	}
	if got := fl.store.consents["1|"+fl.clientID]; got != "public:read team:read" {
		t.Errorf("consent recorded = %q", got)
	}

	// Refresh rotates: the new token works, the old one is reuse. The clock
	// moves a second, not an hour, because the verifier reads the wall clock.
	fl.now = fl.now.Add(time.Second)
	next, bad, status := fl.tokenRequest(t, url.Values{"grant_type": {GrantRefreshToken}, "refresh_token": {tok.RefreshToken}})
	if status != http.StatusOK {
		t.Fatalf("refresh = %d %+v", status, bad)
	}
	if next.RefreshToken == tok.RefreshToken || next.AccessToken == tok.AccessToken || next.Scope != tok.Scope {
		t.Errorf("refresh response = %+v", next)
	}
	if _, err := fl.verifier.Verify(t.Context(), next.AccessToken); err != nil {
		t.Errorf("Verify refreshed token: %v", err)
	}

	_, bad, status = fl.tokenRequest(t, url.Values{"grant_type": {GrantRefreshToken}, "refresh_token": {tok.RefreshToken}})
	if status != http.StatusBadRequest || bad.Error != ErrorInvalidGrant {
		t.Fatalf("reuse of a rotated-out token = %d %+v, want invalid_grant", status, bad)
	}
	// The whole family is revoked, the newest token included.
	_, bad, status = fl.tokenRequest(t, url.Values{"grant_type": {GrantRefreshToken}, "refresh_token": {next.RefreshToken}})
	if status != http.StatusBadRequest || bad.Error != ErrorInvalidGrant {
		t.Errorf("newest token after a family revocation = %d %+v, want invalid_grant", status, bad)
	}
}

func TestFlowCodeReplayRevokesFamily(t *testing.T) {
	t.Parallel()

	fl := newFlow(t, map[string][]string{"github:group:eng": {"team:read"}})
	cb, _ := fl.consent(t, fl.authorize(t), "allow")
	code := cb.Query().Get("code")

	tok, _, status := fl.tokenRequest(t, fl.codeForm(code))
	if status != http.StatusOK {
		t.Fatal(status)
	}
	_, bad, status := fl.tokenRequest(t, fl.codeForm(code))
	if status != http.StatusBadRequest || bad.Error != ErrorInvalidGrant {
		t.Fatalf("second redemption = %d %+v", status, bad)
	}
	_, bad, status = fl.tokenRequest(t, url.Values{"grant_type": {GrantRefreshToken}, "refresh_token": {tok.RefreshToken}})
	if status != http.StatusBadRequest || bad.Error != ErrorInvalidGrant || !strings.Contains(bad.Description, "revoked") {
		t.Errorf("refresh after a code replay = %d %+v, want revoked", status, bad)
	}
}

func TestFlowCodeRefusals(t *testing.T) { //nolint:paralleltest // subtests share and move one clock
	fl := newFlow(t, map[string][]string{"github:group:eng": {"team:read"}})

	tests := []struct {
		name   string
		mutate func(url.Values)
		want   string
	}{
		{"wrong verifier", func(f url.Values) { f.Set("code_verifier", strings.Repeat("a", 43)) }, ErrorInvalidGrant},
		{"malformed verifier", func(f url.Values) { f.Set("code_verifier", "short") }, ErrorInvalidGrant},
		{"wrong client", func(f url.Values) { f.Set("client_id", "https://other.example/oauth.json") }, ErrorInvalidClient},
		{"wrong redirect_uri", func(f url.Values) { f.Set("redirect_uri", "http://127.0.0.1:9999/cb") }, ErrorInvalidGrant},
		{"wrong resource", func(f url.Values) { f.Set("resource", "https://other.example/mcp") }, ErrorInvalidTarget},
		{"no resource", func(f url.Values) { f.Del("resource") }, ErrorInvalidTarget},
		{"unknown code", func(f url.Values) { f.Set("code", "nope") }, ErrorInvalidGrant},
		{"no code", func(f url.Values) { f.Del("code") }, ErrorInvalidRequest},
		{"unsupported grant", func(f url.Values) { f.Set("grant_type", "password") }, ErrorUnsupportedGrant},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb, _ := fl.consent(t, fl.authorize(t), "allow")
			form := fl.codeForm(cb.Query().Get("code"))
			tt.mutate(form)
			_, bad, status := fl.tokenRequest(t, form)
			if status == http.StatusOK || bad.Error != tt.want {
				t.Errorf("token = %d %+v, want %s", status, bad, tt.want)
			}
		})
	}

	t.Run("expired code", func(t *testing.T) {
		cb, _ := fl.consent(t, fl.authorize(t), "allow")
		fl.now = fl.now.Add(2 * time.Minute)
		_, bad, status := fl.tokenRequest(t, fl.codeForm(cb.Query().Get("code")))
		if status != http.StatusBadRequest || bad.Error != ErrorInvalidGrant {
			t.Errorf("expired code = %d %+v", status, bad)
		}
	})
}

func TestFlowRefreshRules(t *testing.T) { //nolint:paralleltest // subtests share and move one clock
	fl := newFlow(t, map[string][]string{"github:group:eng": {"team:read"}})
	issue := func(t *testing.T) TokenResponse {
		t.Helper()
		cb, _ := fl.consent(t, fl.authorize(t), "allow")
		tok, bad, status := fl.tokenRequest(t, fl.codeForm(cb.Query().Get("code")))
		if status != http.StatusOK {
			t.Fatalf("issue = %d %+v", status, bad)
		}

		return tok
	}

	t.Run("expired by disuse", func(t *testing.T) {
		tok := issue(t)
		fl.now = fl.now.Add(721 * time.Hour)
		_, bad, status := fl.tokenRequest(t, url.Values{"grant_type": {GrantRefreshToken}, "refresh_token": {tok.RefreshToken}})
		if status != http.StatusBadRequest || bad.Error != ErrorInvalidGrant || !strings.Contains(bad.Description, "expired") {
			t.Errorf("refresh = %d %+v", status, bad)
		}
	})

	t.Run("wrong client", func(t *testing.T) {
		tok := issue(t)
		_, bad, status := fl.tokenRequest(t, url.Values{
			"grant_type": {GrantRefreshToken}, "refresh_token": {tok.RefreshToken}, "client_id": {"https://other.example/oauth.json"},
		})
		if status != http.StatusUnauthorized || bad.Error != ErrorInvalidClient {
			t.Errorf("refresh = %d %+v", status, bad)
		}
	})

	t.Run("narrower scope", func(t *testing.T) {
		tok := issue(t)
		next, bad, status := fl.tokenRequest(t, url.Values{
			"grant_type": {GrantRefreshToken}, "refresh_token": {tok.RefreshToken}, "scope": {"public:read"},
		})
		if status != http.StatusOK || next.Scope != "public:read" {
			t.Errorf("refresh = %d %+v %+v", status, next, bad)
		}
	})

	t.Run("wider scope", func(t *testing.T) {
		tok := issue(t)
		_, bad, status := fl.tokenRequest(t, url.Values{
			"grant_type": {GrantRefreshToken}, "refresh_token": {tok.RefreshToken}, "scope": {"public:read other:read"},
		})
		if status != http.StatusBadRequest || bad.Error != ErrorInvalidScope {
			t.Errorf("refresh = %d %+v", status, bad)
		}
	})

	t.Run("unknown token", func(t *testing.T) {
		_, bad, status := fl.tokenRequest(t, url.Values{"grant_type": {GrantRefreshToken}, "refresh_token": {"nope"}})
		if status != http.StatusBadRequest || bad.Error != ErrorInvalidGrant {
			t.Errorf("refresh = %d %+v", status, bad)
		}
	})

	t.Run("revoke ends the family", func(t *testing.T) {
		tok := issue(t)
		rec := fl.do(t, http.MethodPost, RevokePath, url.Values{"token": {tok.RefreshToken}}.Encode(),
			map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
		if rec.Code != http.StatusOK {
			t.Fatalf("revoke = %d %s", rec.Code, rec.Body.String())
		}
		_, bad, status := fl.tokenRequest(t, url.Values{"grant_type": {GrantRefreshToken}, "refresh_token": {tok.RefreshToken}})
		if status != http.StatusBadRequest || bad.Error != ErrorInvalidGrant {
			t.Errorf("refresh after revoke = %d %+v", status, bad)
		}
		// An unknown token is 200 too.
		rec = fl.do(t, http.MethodPost, RevokePath, "token=nope", map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
		if rec.Code != http.StatusOK {
			t.Errorf("revoke unknown = %d", rec.Code)
		}
	})
}

func TestAuthorizeValidation(t *testing.T) {
	t.Parallel()

	fl := newFlow(t, map[string][]string{"github:group:eng": {"team:read"}})

	pre := []struct {
		name     string
		override url.Values
		want     string
	}{
		{"response_type", url.Values{"response_type": {"token"}}, ErrorUnsupportedResponse},
		{"client_id http", url.Values{"client_id": {strings.Replace(fl.clientID, "https://", "http://", 1)}}, ErrorInvalidClient},
		{"client_id unknown", url.Values{"client_id": {"https://nowhere.invalid/oauth.json"}}, ErrorInvalidClient},
		{"redirect_uri unregistered", url.Values{"redirect_uri": {"https://agent.example/other"}}, ErrorInvalidRequest},
	}
	for _, tt := range pre {
		t.Run("page "+tt.name, func(t *testing.T) {
			t.Parallel()

			rec := fl.do(t, http.MethodGet, fl.authorizeURL(tt.override), "", nil)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tt.want) {
				t.Errorf("authorize = %d %q, want a 400 page naming %s", rec.Code, rec.Body.String(), tt.want)
			}
			if !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
				t.Errorf("Content-Type = %q", rec.Header().Get("Content-Type"))
			}
		})
	}

	post := []struct {
		name     string
		override url.Values
		want     string
	}{
		{"no code_challenge", url.Values{"code_challenge": {""}}, ErrorInvalidRequest},
		{"plain method", url.Values{"code_challenge_method": {"plain"}}, ErrorInvalidRequest},
		{"short challenge", url.Values{"code_challenge": {"short"}}, ErrorInvalidRequest},
		{"wrong resource", url.Values{"resource": {"https://other.example/mcp"}}, ErrorInvalidTarget},
		{"unknown scope", url.Values{"scope": {"public:read secret:read"}}, ErrorInvalidScope},
		{"no state", url.Values{"state": {""}}, ErrorInvalidRequest},
	}
	for _, tt := range post {
		t.Run("redirect "+tt.name, func(t *testing.T) {
			t.Parallel()

			rec := fl.do(t, http.MethodGet, fl.authorizeURL(tt.override), "", nil)
			if rec.Code != http.StatusFound {
				t.Fatalf("authorize = %d %q", rec.Code, rec.Body.String())
			}
			loc, err := url.Parse(rec.Header().Get("Location"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(loc.String(), redirect+"?") || loc.Query().Get("error") != tt.want || loc.Query().Get("iss") != testIssuer {
				t.Errorf("Location = %s, want error=%s with iss", loc, tt.want)
			}
			if tt.name != "no state" && loc.Query().Get("state") != "xyz" {
				t.Errorf("Location = %s, want state echoed", loc)
			}
		})
	}

	t.Run("empty scope means public:read", func(t *testing.T) {
		t.Parallel()

		rec := fl.do(t, http.MethodGet, fl.authorizeURL(url.Values{"scope": {""}}), "", nil)
		if rec.Code != http.StatusFound || !strings.Contains(rec.Header().Get("Location"), "/oauth/consent?authz=") {
			t.Fatalf("authorize = %d %s", rec.Code, rec.Header().Get("Location"))
		}
	})
}

func TestConsentEndpoints(t *testing.T) { //nolint:paralleltest // subtests share one session and clock
	fl := newFlow(t, map[string][]string{"github:group:eng": {"team:read", "docz:read"}})

	t.Run("view shows the intersection", func(t *testing.T) {
		id := fl.authorize(t)
		rec := fl.do(t, http.MethodGet, "/api/v1/oauth/authorizations/"+id, "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET = %d %s", rec.Code, rec.Body.String())
		}
		var v AuthorizationView
		if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		if v.ClientName != "Agent" || v.ClientURI != "https://agent.example" || len(v.ScopesRequested) != 2 || len(v.ScopesGranted) != 2 {
			t.Errorf("view = %+v", v)
		}
	})

	t.Run("deny redirects access_denied", func(t *testing.T) {
		cb, status := fl.consent(t, fl.authorize(t), "deny")
		if status != http.StatusOK || cb.Query().Get("error") != ErrorAccessDenied || cb.Query().Get("state") != "xyz" ||
			cb.Query().Get("iss") != testIssuer {
			t.Errorf("deny = %d %s", status, cb)
		}
	})

	t.Run("unknown, consumed, expired, and another session's are 404", func(t *testing.T) {
		if _, status := fl.consent(t, "nope", "allow"); status != http.StatusNotFound {
			t.Errorf("unknown = %d", status)
		}

		id := fl.authorize(t)
		if _, status := fl.consent(t, id, "allow"); status != http.StatusOK {
			t.Fatalf("first decision = %d", status)
		}
		if _, status := fl.consent(t, id, "allow"); status != http.StatusNotFound {
			t.Errorf("consumed = %d", status)
		}

		id = fl.authorize(t)
		fl.now = fl.now.Add(11 * time.Minute)
		if _, status := fl.consent(t, id, "allow"); status != http.StatusNotFound {
			t.Errorf("expired = %d", status)
		}

		id = fl.authorize(t)
		rec := fl.do(t, http.MethodGet, "/api/v1/oauth/authorizations/"+id, "", nil)
		if rec.Code != http.StatusOK {
			t.Fatal(rec.Code)
		}
		other := fl.principal
		other.SessionID = "sess-2"
		fl.principal = other
		rec = fl.do(t, http.MethodPost, "/api/v1/oauth/authorizations/"+id, `{"decision":"allow"}`, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("POST from another session = %d", rec.Code)
		}
		rec = fl.do(t, http.MethodGet, "/api/v1/oauth/authorizations/"+id, "", nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET from another session = %d", rec.Code)
		}
		fl.principal.SessionID = "sess-1"
	})

	t.Run("POST without the GET is 404", func(t *testing.T) {
		id := fl.authorize(t)
		rec := fl.do(t, http.MethodPost, "/api/v1/oauth/authorizations/"+id, `{"decision":"allow"}`, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("POST unbound = %d", rec.Code)
		}
	})

	t.Run("bad decision", func(t *testing.T) {
		id := fl.authorize(t)
		rec := fl.do(t, http.MethodPost, "/api/v1/oauth/authorizations/"+id, `{"decision":"maybe"}`, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("bad decision = %d", rec.Code)
		}
	})

	t.Run("no session is 401", func(t *testing.T) {
		id := fl.authorize(t)
		fl.hasSess = false
		rec := fl.do(t, http.MethodGet, "/api/v1/oauth/authorizations/"+id, "", nil)
		fl.hasSess = true
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("no session = %d", rec.Code)
		}
	})
}

func TestErrorFor(t *testing.T) {
	t.Parallel()

	if errorFor(ErrInvalidClient) != ErrorInvalidClient || errorFor(errors.New("x")) != ErrorInvalidGrant {
		t.Error("errorFor mapping")
	}
}
