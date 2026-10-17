package oauth

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

const (
	testIssuer   = "https://docz-api.example"
	testAudience = testIssuer + "/mcp"
)

func newKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// fakeIssuer is an external issuer: a discovery document, a JWKS, and a
// signer over its keys. fetches counts JWKS requests.
type fakeIssuer struct {
	srv     *httptest.Server
	keys    []*ecdsa.PrivateKey
	kids    []string
	fetches atomic.Int32
}

func newFakeIssuer(t *testing.T) *fakeIssuer {
	t.Helper()
	f := &fakeIssuer{}
	f.addKey(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": f.srv.URL, "jwks_uri": f.srv.URL + "/keys"})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		f.fetches.Add(1)
		set := jose.JSONWebKeySet{}
		for i, k := range f.keys {
			set.Keys = append(set.Keys, jose.JSONWebKey{Key: &k.PublicKey, KeyID: f.kids[i], Algorithm: "ES256", Use: "sig"})
		}
		_ = json.NewEncoder(w).Encode(set)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeIssuer) addKey(t *testing.T) {
	t.Helper()
	key := newKey(t)
	f.keys = append(f.keys, key)
	f.kids = append(f.kids, "k"+string(rune('0'+len(f.keys))))
}

// sign mints a token from the issuer's key i with the given registered and
// custom claims.
func (f *fakeIssuer) sign(t *testing.T, i int, std *jwt.Claims, custom any) string {
	t.Helper()
	return signWith(t, f.keys[i], f.kids[i], std, custom)
}

func signWith(t *testing.T, key *ecdsa.PrivateKey, kid string, std *jwt.Claims, custom any) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", kid))
	if err != nil {
		t.Fatal(err)
	}
	b := jwt.Signed(signer).Claims(std)
	if custom != nil {
		b = b.Claims(custom)
	}
	raw, err := b.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func stdClaims(issuer, sub string, exp time.Time) *jwt.Claims {
	return &jwt.Claims{
		Issuer: issuer, Subject: sub, Audience: jwt.Audience{testAudience},
		Expiry: jwt.NewNumericDate(exp), IssuedAt: jwt.NewNumericDate(time.Now()),
	}
}

func TestVerifierOwnKey(t *testing.T) {
	key := newKey(t)
	signer, err := NewSigner(testIssuer, testAudience, key, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	var observed []string
	v, err := NewVerifier(t.Context(), &VerifierOptions{
		Audience: testAudience, Issuer: testIssuer, Key: &key.PublicKey,
		Observe: func(issuer, status string) { observed = append(observed, issuer+" "+status) },
	})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := signer.Issue(&Claims{
		Subject: "user:github:1", ClientID: "https://client.example/c.json",
		Scopes: []string{"public:read", "platform-team:read"}, Email: "a@example.com", Login: "a",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := v.Verify(t.Context(), raw)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.Issuer != testIssuer || got.Subject != "user:github:1" || got.ClientID != "https://client.example/c.json" ||
		got.Email != "a@example.com" || got.Login != "a" || got.ID == "" {
		t.Errorf("claims = %+v", got)
	}
	if strings.Join(got.Scopes, " ") != "public:read platform-team:read" {
		t.Errorf("scopes = %q", got.Scopes)
	}
	if until := time.Until(got.Expiry); until < 14*time.Minute || until > 15*time.Minute {
		t.Errorf("expiry in %v, want ~15m", until)
	}
	if len(observed) != 1 || observed[0] != testIssuer+" ok" {
		t.Errorf("observed = %q", observed)
	}

	provider, subject, ok := UserSubject(got.Subject)
	if !ok || provider != "github" || subject != "1" {
		t.Errorf("UserSubject = %q %q %v", provider, subject, ok)
	}

	// The published JWKS carries the kid the token names.
	set := signer.JWKS()
	if len(set.Keys) != 1 || set.Keys[0].KeyID != v.own.kid {
		t.Errorf("JWKS kid = %q, want %q", set.Keys[0].KeyID, v.own.kid)
	}
}

func TestVerifierRefusals(t *testing.T) {
	key := newKey(t)
	other := newKey(t)
	kid, _ := KeyID(&key.PublicKey)
	v, err := NewVerifier(t.Context(), &VerifierOptions{Audience: testAudience, Issuer: testIssuer, Key: &key.PublicKey})
	if err != nil {
		t.Fatal(err)
	}
	in := time.Now().Add(time.Hour)

	tests := []struct {
		name string
		raw  string
		want error
	}{
		{name: "garbage", raw: "not.a.jwt", want: ErrInvalidToken},
		{name: "wrong key", raw: signWith(t, other, kid, stdClaims(testIssuer, "s", in), nil), want: ErrInvalidToken},
		{name: "unknown kid", raw: signWith(t, key, "nope", stdClaims(testIssuer, "s", in), nil), want: ErrUnknownKey},
		{name: "unknown issuer", raw: signWith(t, key, kid, stdClaims("https://else.example", "s", in), nil), want: ErrUnknownIssuer},
		{name: "expired", raw: signWith(t, key, kid, stdClaims(testIssuer, "s", time.Now().Add(-time.Minute)), nil), want: ErrTokenExpired},
		{
			name: "no exp",
			raw:  signWith(t, key, kid, &jwt.Claims{Issuer: testIssuer, Audience: jwt.Audience{testAudience}}, nil),
			want: ErrTokenExpired,
		},
		{name: "not yet valid", raw: signWith(t, key, kid, func() *jwt.Claims {
			c := stdClaims(testIssuer, "s", in)
			c.NotBefore = jwt.NewNumericDate(time.Now().Add(time.Hour))
			return c
		}(), nil), want: ErrTokenNotYetValid},
		{name: "wrong audience", raw: signWith(t, key, kid, func() *jwt.Claims {
			c := stdClaims(testIssuer, "s", in)
			c.Audience = jwt.Audience{"https://else.example/mcp"}
			return c
		}(), nil), want: ErrWrongAudience},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := v.Verify(t.Context(), tt.raw)
			if !errors.Is(err, tt.want) || !errors.Is(err, ErrInvalidToken) {
				t.Errorf("Verify = %v, want %v wrapped in ErrInvalidToken", err, tt.want)
			}
		})
	}

	// Within the skew is fine.
	raw := signWith(t, key, kid, stdClaims(testIssuer, "s", time.Now().Add(-10*time.Second)), nil)
	if _, err := v.Verify(t.Context(), raw); err != nil {
		t.Errorf("Verify within skew = %v, want nil", err)
	}
}

func TestVerifierTrustedIssuer(t *testing.T) {
	iss := newFakeIssuer(t)
	cache := NewMemoryCache()
	v, err := NewVerifier(t.Context(), &VerifierOptions{Audience: testAudience, TrustedIssuers: []string{iss.srv.URL + "/"}, Cache: cache})
	if err != nil {
		t.Fatal(err)
	}
	in := time.Now().Add(time.Hour)

	// scp, the array form, and a trailing slash on iss.
	raw := iss.sign(t, 0, stdClaims(iss.srv.URL+"/", "agent-1", in), map[string]any{"scp": []string{"public:read", "", "public:read", "team:read"}})
	got, err := v.Verify(t.Context(), raw)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.Issuer != iss.srv.URL || got.Subject != "agent-1" || strings.Join(got.Scopes, " ") != "public:read team:read" {
		t.Errorf("claims = %+v", got)
	}
	if n := iss.fetches.Load(); n != 1 {
		t.Errorf("JWKS fetched %d times, want 1", n)
	}

	// A second verifier on the same cache needs no fetch.
	v2, err := NewVerifier(t.Context(), &VerifierOptions{Audience: testAudience, TrustedIssuers: []string{iss.srv.URL}, Cache: cache})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v2.Verify(t.Context(), raw); err != nil {
		t.Fatalf("Verify through the cache: %v", err)
	}
	if n := iss.fetches.Load(); n != 1 {
		t.Errorf("JWKS fetched %d times after a cached verifier, want 1", n)
	}

	// A rotated key: the unknown kid triggers one refresh.
	iss.addKey(t)
	rotated := iss.sign(t, 1, stdClaims(iss.srv.URL, "agent-2", in), map[string]any{"scope": "public:read"})
	if _, err := v.Verify(t.Context(), rotated); err != nil {
		t.Fatalf("Verify after rotation: %v", err)
	}
	if n := iss.fetches.Load(); n != 2 {
		t.Errorf("JWKS fetched %d times after rotation, want 2", n)
	}

	// A second unknown kid inside the minute does not refetch.
	unknown := signWith(t, newKey(t), "k9", stdClaims(iss.srv.URL, "agent-3", in), nil)
	if _, err := v.Verify(t.Context(), unknown); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("Verify with an unknown kid = %v, want ErrUnknownKey", err)
	}
	if n := iss.fetches.Load(); n != 2 {
		t.Errorf("JWKS fetched %d times on a throttled unknown kid, want 2", n)
	}

	// The audience is checked for external tokens too.
	wrong := iss.sign(t, 0, func() *jwt.Claims {
		c := stdClaims(iss.srv.URL, "agent-1", in)
		c.Audience = jwt.Audience{"https://else.example"}
		return c
	}(), nil)
	if _, err := v.Verify(t.Context(), wrong); !errors.Is(err, ErrWrongAudience) {
		t.Errorf("Verify with a foreign audience = %v, want ErrWrongAudience", err)
	}
}

func TestNewVerifierBadIssuer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": "https://someone.else", "jwks_uri": "https://someone.else/keys"})
	}))
	t.Cleanup(srv.Close)

	if _, err := NewVerifier(t.Context(), &VerifierOptions{Audience: testAudience, TrustedIssuers: []string{srv.URL}}); err == nil {
		t.Error("NewVerifier accepted a discovery document naming another issuer")
	}
	if _, err := NewVerifier(t.Context(), &VerifierOptions{Audience: testAudience, TrustedIssuers: []string{"https://127.0.0.1:1"}}); err == nil {
		t.Error("NewVerifier accepted an unreachable issuer")
	}
	if _, err := NewVerifier(context.Background(), &VerifierOptions{}); err == nil {
		t.Error("NewVerifier accepted no audience")
	}
}

func TestMemoryCacheExpiry(t *testing.T) {
	c := NewMemoryCache()
	now := time.Now()
	c.now = func() time.Time { return now }
	if err := c.Set(t.Context(), "k", []byte("v"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if got, err := c.Get(t.Context(), "k"); err != nil || string(got) != "v" {
		t.Errorf("Get = %q, %v", got, err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := c.Get(t.Context(), "k"); !errors.Is(err, ErrCacheMiss) {
		t.Errorf("Get after expiry = %v, want ErrCacheMiss", err)
	}
}

// fixedKey is one P-256 key for every golden, so the thumbprint is stable.
func fixedKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	// A fixed scalar; the key is for tests only and secures nothing.
	key, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), bytes.Repeat([]byte{0x42}, 32))
	if err != nil {
		t.Fatal(err)
	}

	return key
}
