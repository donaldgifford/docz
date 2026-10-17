package session

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/internal/auth"
	"github.com/donaldgifford/docz/v2/internal/oauth"
)

// fakeVerifier accepts one raw token.
type fakeVerifier struct {
	token  string
	claims oauth.Claims
}

func (f *fakeVerifier) Verify(_ context.Context, raw string) (oauth.Claims, error) {
	if raw == f.token {
		return f.claims, nil
	}

	return oauth.Claims{}, oauth.ErrInvalidToken
}

const metadataURL = "https://docz.example/.well-known/oauth-protected-resource"

func TestAuthn(t *testing.T) {
	t.Parallel()

	cookieSess := Session{
		ID:       "good-id",
		Identity: auth.Identity{Provider: "github", Subject: "42", Login: "octocat", Groups: []string{"eng"}},
		Via:      ViaCookie,
	}
	store := &fakeLookuper{id: "good-id", session: cookieSess}
	verifier := &fakeVerifier{token: "good-token", claims: oauth.Claims{
		Issuer: "https://idp.example", Subject: "svc-1", ClientID: "agent", Scopes: []string{"public:read", "team:read"}, ID: "jti-1",
	}}
	ownToken := &fakeVerifier{token: "own-token", claims: oauth.Claims{
		Issuer: "https://docz.example", Subject: "user:github:42", ClientID: "mcp-client",
		Scopes: []string{"public:read"}, Email: "o@example.com", Login: "octocat",
	}}
	scopes := func(id *auth.Identity) []string {
		if slices.Contains(id.Groups, "eng") {
			return []string{"public:read", "team:read"}
		}

		return []string{"public:read"}
	}
	goodCookie := &http.Cookie{Name: cookieName, Value: "good-id"}

	tests := []struct {
		name          string
		opts          AuthnOptions
		cookie        *http.Cookie
		authorization string
		wantStatus    int
		wantChallenge bool
		wantVia       SessionVia
		wantScopes    []string
		wantSubject   string
		wantKey       string
	}{
		{
			name: "cookie only", opts: AuthnOptions{Sessions: store, Verifier: verifier, Scopes: scopes, ResourceMetadata: metadataURL},
			cookie: goodCookie, wantStatus: http.StatusOK, wantVia: ViaCookie, wantScopes: []string{"public:read", "team:read"},
			wantSubject: "42", wantKey: "user:github:42",
		},
		{
			name: "cookie with the default scope func", opts: AuthnOptions{Sessions: store, ResourceMetadata: metadataURL},
			cookie: goodCookie, wantStatus: http.StatusOK, wantVia: ViaCookie, wantScopes: []string{"public:read"}, wantSubject: "42",
		},
		{
			name: "no cookie is 401 with the challenge", opts: AuthnOptions{Sessions: store, Verifier: verifier, ResourceMetadata: metadataURL},
			wantStatus: http.StatusUnauthorized, wantChallenge: true,
		},
		{
			name: "no cookie and no metadata has no challenge", opts: AuthnOptions{Sessions: store, Verifier: verifier},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "unknown cookie is 401", opts: AuthnOptions{Sessions: store, Verifier: verifier, ResourceMetadata: metadataURL},
			cookie: &http.Cookie{Name: cookieName, Value: "bogus"}, wantStatus: http.StatusUnauthorized, wantChallenge: true,
		},
		{
			name:   "session backend down is 503",
			opts:   AuthnOptions{Sessions: &fakeLookuper{err: errors.New("redis down")}, Verifier: verifier, ResourceMetadata: metadataURL},
			cookie: goodCookie, wantStatus: http.StatusServiceUnavailable,
		},
		{
			name: "external bearer token", opts: AuthnOptions{Sessions: store, Verifier: verifier, Scopes: scopes, ResourceMetadata: metadataURL},
			authorization: "Bearer good-token", wantStatus: http.StatusOK, wantVia: ViaToken, wantScopes: []string{"public:read", "team:read"},
			wantSubject: "svc-1", wantKey: "https://idp.example:svc-1",
		},
		{
			name: "bearer scheme is case-insensitive", opts: AuthnOptions{Sessions: store, Verifier: verifier, ResourceMetadata: metadataURL},
			authorization: "bearer good-token", wantStatus: http.StatusOK, wantVia: ViaToken,
			wantScopes: []string{"public:read", "team:read"}, wantSubject: "svc-1",
		},
		{
			name:          "docz-api's own token names the person",
			opts:          AuthnOptions{Sessions: store, Verifier: ownToken, Scopes: scopes, ResourceMetadata: metadataURL},
			authorization: "Bearer own-token", wantStatus: http.StatusOK, wantVia: ViaToken, wantScopes: []string{"public:read"},
			wantSubject: "42", wantKey: "user:github:42",
		},
		{
			name:          "bad bearer never falls through to a good cookie",
			opts:          AuthnOptions{Sessions: store, Verifier: verifier, ResourceMetadata: metadataURL},
			cookie:        goodCookie,
			authorization: "Bearer expired",
			wantStatus:    http.StatusUnauthorized,
			wantChallenge: true,
		},
		{
			name: "empty bearer is no bearer", opts: AuthnOptions{Sessions: store, Verifier: verifier, Scopes: scopes, ResourceMetadata: metadataURL},
			cookie: goodCookie, authorization: "Bearer ", wantStatus: http.StatusOK, wantVia: ViaCookie, wantSubject: "42",
			wantScopes: []string{"public:read", "team:read"},
		},
		{
			name:          "another scheme is no bearer",
			opts:          AuthnOptions{Sessions: store, Verifier: verifier, Scopes: scopes, ResourceMetadata: metadataURL},
			cookie:        goodCookie,
			authorization: "Basic abc",
			wantStatus:    http.StatusOK,
			wantVia:       ViaCookie,
			wantSubject:   "42",
			wantScopes:    []string{"public:read", "team:read"},
		},
		{
			name: "no verifier ignores the header", opts: AuthnOptions{Sessions: store, Scopes: scopes, ResourceMetadata: metadataURL},
			cookie: goodCookie, authorization: "Bearer good-token", wantStatus: http.StatusOK, wantVia: ViaCookie, wantSubject: "42",
			wantScopes: []string{"public:read", "team:read"},
		},
		{
			name: "no verifier and no cookie is 401", opts: AuthnOptions{Sessions: store, ResourceMetadata: metadataURL},
			authorization: "Bearer good-token", wantStatus: http.StatusUnauthorized, wantChallenge: true,
		},
		{
			name: "anonymous", opts: AuthnOptions{Anonymous: true, Verifier: verifier, ResourceMetadata: metadataURL},
			wantStatus: http.StatusOK, wantVia: ViaAnonymous, wantSubject: AnonymousSubject, wantKey: "user:none:anonymous",
		},
		{
			name: "anonymous still honours a bearer", opts: AuthnOptions{Anonymous: true, Verifier: verifier, ResourceMetadata: metadataURL},
			authorization: "Bearer good-token", wantStatus: http.StatusOK, wantVia: ViaToken, wantSubject: "svc-1",
			wantScopes: []string{"public:read", "team:read"},
		},
		{
			name: "anonymous rejects a bad bearer", opts: AuthnOptions{Anonymous: true, Verifier: verifier, ResourceMetadata: metadataURL},
			authorization: "Bearer bad", wantStatus: http.StatusUnauthorized, wantChallenge: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var (
				got     Session
				reached bool
			)
			protected := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached = true
				got, _ = FromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/repos", nil)
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}
			if tc.authorization != "" {
				req.Header.Set("Authorization", tc.authorization)
			}
			rec := httptest.NewRecorder()
			Authn(tc.opts)(protected).ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			challenge := rec.Header().Get("WWW-Authenticate")
			if tc.wantChallenge {
				want := `Bearer resource_metadata="` + metadataURL + `", scope="public:read"`
				if challenge != want {
					t.Errorf("WWW-Authenticate = %q, want %q", challenge, want)
				}
			} else if challenge != "" {
				t.Errorf("WWW-Authenticate = %q, want none", challenge)
			}
			if tc.wantStatus != http.StatusOK {
				if reached {
					t.Error("handler reached on a refused request")
				}

				return
			}
			if !reached {
				t.Fatal("handler not reached")
			}
			if got.Via != tc.wantVia {
				t.Errorf("Via = %s, want %s", got.Via, tc.wantVia)
			}
			if got.Identity.Subject != tc.wantSubject {
				t.Errorf("Subject = %q, want %q", got.Identity.Subject, tc.wantSubject)
			}
			if !slices.Equal(got.Scopes, tc.wantScopes) {
				t.Errorf("Scopes = %v, want %v", got.Scopes, tc.wantScopes)
			}
			if tc.wantKey != "" && got.PrincipalKey() != tc.wantKey {
				t.Errorf("PrincipalKey = %q, want %q", got.PrincipalKey(), tc.wantKey)
			}
			if got.Via == ViaToken && (got.ClientID == "" || got.Issuer == "") {
				t.Errorf("token session missing ClientID/Issuer: %+v", got)
			}
			if got.Via == ViaToken && !strings.Contains(got.ID, got.Identity.Subject) && got.ID != "jti-1" {
				t.Errorf("token session ID = %q, want the jti or issuer:subject", got.ID)
			}
		})
	}
}

func TestSessionViaString(t *testing.T) {
	t.Parallel()

	for v, want := range map[SessionVia]string{ViaCookie: "cookie", ViaToken: "token", ViaAnonymous: "anonymous"} {
		if got := v.String(); got != want {
			t.Errorf("SessionVia(%d).String() = %q, want %q", v, got, want)
		}
	}
}
