// Package session issues and resolves Redis-backed site-user sessions. A
// successful login stores the authenticated auth.Identity under sess:<id> with
// an expiry (SESSION_TTL) and hands the browser an opaque, high-entropy session
// id in an httpOnly cookie; that id is the only credential. Middleware resolves
// the cookie into a Session on each request (401 when absent/expired) and
// injects it into the request context. Revoke (logout) is a single Redis DEL.
package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/donaldgifford/docz-api/internal/auth"
)

const (
	// cookieName is the session cookie name.
	cookieName = "docz_session"
	// keyPrefix namespaces session keys in Redis.
	keyPrefix = "sess:"
	// idBytes is the session id entropy: 32 random bytes, the sole credential.
	idBytes = 32
)

// ErrSessionNotFound is returned by Lookup when a session id is absent or has
// expired.
var ErrSessionNotFound = errors.New("session not found")

// Session is a resolved session: its id plus the authenticated identity.
type Session struct {
	ID       string
	Identity auth.Identity
}

// sessionData is the JSON value stored in Redis under sess:<id>. It carries no
// secrets — only identity claims — so it is not encrypted.
type sessionData struct {
	Provider  string    `json:"provider"`
	Subject   string    `json:"subject"`
	Email     string    `json:"email"`
	Login     string    `json:"login"`
	Groups    []string  `json:"groups"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Store is the Redis-backed session store. It owns its redis client (separate
// from the queue's) so session and queue lifecycles stay independent.
type Store struct {
	redis  *redis.Client
	ttl    time.Duration
	secure bool
}

// New builds a Store from a redis URL, a session TTL, and whether cookies must
// carry the Secure attribute (true when the service is served over HTTPS).
func New(redisURL string, ttl time.Duration, secure bool) (*Store, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url for sessions: %w", err)
	}
	return &Store{redis: redis.NewClient(opt), ttl: ttl, secure: secure}, nil
}

// Issue creates a session for identity and returns its opaque session id.
func (s *Store) Issue(ctx context.Context, identity *auth.Identity) (string, error) {
	raw := make([]byte, idBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	sessionID := base64.RawURLEncoding.EncodeToString(raw)

	now := time.Now()
	data, err := json.Marshal(sessionData{
		Provider:  identity.Provider,
		Subject:   identity.Subject,
		Email:     identity.Email,
		Login:     identity.Login,
		Groups:    identity.Groups,
		IssuedAt:  now,
		ExpiresAt: now.Add(s.ttl),
	})
	if err != nil {
		return "", fmt.Errorf("marshal session: %w", err)
	}
	if serr := s.redis.Set(ctx, keyPrefix+sessionID, data, s.ttl).Err(); serr != nil {
		return "", fmt.Errorf("store session: %w", serr)
	}
	return sessionID, nil
}

// Lookup returns the Session for sessionID, or ErrSessionNotFound when it is
// absent or expired (Redis has already dropped an expired key).
func (s *Store) Lookup(ctx context.Context, sessionID string) (Session, error) {
	raw, err := s.redis.Get(ctx, keyPrefix+sessionID).Result()
	if errors.Is(err, redis.Nil) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", err)
	}
	var data sessionData
	if uerr := json.Unmarshal([]byte(raw), &data); uerr != nil {
		return Session{}, fmt.Errorf("decode session: %w", uerr)
	}
	return Session{
		ID: sessionID,
		Identity: auth.Identity{
			Provider: data.Provider,
			Subject:  data.Subject,
			Email:    data.Email,
			Login:    data.Login,
			Groups:   data.Groups,
		},
	}, nil
}

// Revoke deletes the session. It is idempotent: deleting a missing key is fine.
func (s *Store) Revoke(ctx context.Context, sessionID string) error {
	if err := s.redis.Del(ctx, keyPrefix+sessionID).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// Ping verifies the session Redis is reachable; it backs the /readyz probe.
func (s *Store) Ping(ctx context.Context) error {
	if err := s.redis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping session redis: %w", err)
	}
	return nil
}

// Close releases the redis client. Safe to call once at shutdown.
func (s *Store) Close() error {
	if err := s.redis.Close(); err != nil {
		return fmt.Errorf("close session redis: %w", err)
	}
	return nil
}

// SetCookie writes the session cookie for sessionID onto w. SameSite=Lax is
// required (not Strict) so the cookie survives the top-level redirect back from
// the auth provider to /auth/callback; HttpOnly keeps it out of JavaScript;
// Secure is set when serving over HTTPS.
func (s *Store) SetCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   int(s.ttl.Seconds()),
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearCookie writes an already-expired session cookie so the browser deletes it.
func (s *Store) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
}
