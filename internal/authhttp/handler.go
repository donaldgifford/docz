// Package authhttp serves the four site-user auth endpoints and ties together
// the provider registry (internal/auth), the Redis session store
// (internal/session), and the durable users table (internal/store):
//
//	GET  /auth/login?provider=…   begin OAuth/OIDC (redirect to the provider)
//	GET  /auth/callback           complete login: verify state, Exchange, upsert
//	                              user, issue session, set cookie
//	GET  /api/v1/auth/session     the current user, or 401
//	POST /api/v1/auth/logout      revoke the session and clear the cookie
//
// The first two are public (state is the CSRF guard); the last two sit behind
// the session middleware in the /api/v1 group.
package authhttp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/donaldgifford/docz-api/internal/auth"
	"github.com/donaldgifford/docz-api/internal/session"
	"github.com/donaldgifford/docz-api/internal/store"
)

// userUpserter is the durable-user surface authhttp needs. *store.Store
// satisfies it.
type userUpserter interface {
	UpsertUser(ctx context.Context, in store.UserInput) (int64, error)
}

// sessionStore is the session surface authhttp needs. *session.Store satisfies
// it. Lookup is not here — the session middleware handles resolution; handlers
// read the resolved session via session.FromContext.
type sessionStore interface {
	Issue(ctx context.Context, identity *auth.Identity) (string, error)
	Revoke(ctx context.Context, sessionID string) error
	SetCookie(w http.ResponseWriter, sessionID string)
	ClearCookie(w http.ResponseWriter)
}

// The production implementations satisfy the consumer interfaces above.
var (
	_ userUpserter = (*store.Store)(nil)
	_ sessionStore = (*session.Store)(nil)
)

// Handler serves the auth endpoints.
type Handler struct {
	registry    *auth.Registry
	sessions    sessionStore
	users       userUpserter
	stateSecret []byte
}

// New builds a Handler. stateSecret (the session secret) signs the OAuth state.
func New(reg *auth.Registry, sessions sessionStore, users userUpserter, stateSecret []byte) *Handler {
	return &Handler{registry: reg, sessions: sessions, users: users, stateSecret: stateSecret}
}

// MountPublic registers /auth/login and /auth/callback on the root router: they
// run without the session gate (login has no session yet; callback is
// authenticated by its signed state).
func (h *Handler) MountPublic(r chi.Router) {
	r.Get("/auth/login", h.login)
	r.Get("/auth/callback", h.callback)
}

// MountAPI registers /auth/session and /auth/logout on r, which the caller has
// already placed inside the session-gated /api/v1 group.
func (h *Handler) MountAPI(r chi.Router) {
	r.Get("/auth/session", h.getSession)
	r.Post("/auth/logout", h.logout)
}

// sessionDTO is the /api/v1/auth/session response shape.
type sessionDTO struct {
	Provider string   `json:"provider"`
	Subject  string   `json:"subject"`
	Email    string   `json:"email,omitempty"`
	Login    string   `json:"login,omitempty"`
	Groups   []string `json:"groups,omitempty"`
}

// writeJSON serializes v as a 200 JSON response.
func writeJSON(w http.ResponseWriter, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encoding response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, werr := w.Write(body); werr != nil {
		slog.Debug("auth response write failed", "err", werr)
	}
}

// writeError writes a JSON error envelope with the given status. It mirrors
// writeJSON's marshal-then-write shape so the whole package emits JSON one way.
func writeError(w http.ResponseWriter, status int, msg string) {
	body, err := json.Marshal(map[string]string{"error": msg})
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, werr := w.Write(body); werr != nil {
		slog.Debug("auth error response write failed", "status", status, "err", werr)
	}
}

// serverError logs the underlying error and returns an opaque 500.
func serverError(w http.ResponseWriter, op string, err error) {
	slog.Error("authhttp server error", "op", op, "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}
