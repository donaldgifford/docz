package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewRejectsBadURL(t *testing.T) {
	if _, err := New("://not-a-url", time.Hour, false); err == nil {
		t.Error("New with a malformed redis URL returned nil error, want a parse error")
	}
}

func TestNewAcceptsValidURL(t *testing.T) {
	// New must not dial Redis (go-redis connects lazily), so a valid URL
	// succeeds without a running server.
	store, err := New("redis://localhost:6379/0", time.Hour, true)
	if err != nil {
		t.Fatalf("New with a valid URL: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if store.ttl != time.Hour || !store.secure {
		t.Errorf("store = %+v, want ttl=1h secure=true", store)
	}
}

func TestSetCookieAttributes(t *testing.T) {
	store := &Store{ttl: 2 * time.Hour, secure: true}
	rec := httptest.NewRecorder()
	store.SetCookie(rec, "sess-abc")

	c := readCookie(t, rec, cookieName)
	if c.Value != "sess-abc" {
		t.Errorf("value = %q, want sess-abc", c.Value)
	}
	if !c.HttpOnly {
		t.Error("cookie is not HttpOnly")
	}
	if !c.Secure {
		t.Error("cookie is not Secure when the store is secure")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax (must survive the provider redirect)", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("path = %q, want /", c.Path)
	}
	if c.MaxAge != int((2 * time.Hour).Seconds()) {
		t.Errorf("MaxAge = %d, want %d", c.MaxAge, int((2 * time.Hour).Seconds()))
	}
}

func TestSetCookieInsecureWhenNotHTTPS(t *testing.T) {
	store := &Store{ttl: time.Hour, secure: false}
	rec := httptest.NewRecorder()
	store.SetCookie(rec, "x")
	if readCookie(t, rec, cookieName).Secure {
		t.Error("cookie is Secure even though the store is not secure")
	}
}

func TestClearCookieExpires(t *testing.T) {
	store := &Store{ttl: time.Hour, secure: true}
	rec := httptest.NewRecorder()
	store.ClearCookie(rec)

	c := readCookie(t, rec, cookieName)
	if c.MaxAge != -1 {
		t.Errorf("MaxAge = %d, want -1 (delete)", c.MaxAge)
	}
	if c.Value != "" {
		t.Errorf("value = %q, want empty on clear", c.Value)
	}
}

// readCookie extracts the named cookie from a recorded response.
func readCookie(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("cookie %q not set", name)
	return nil
}
