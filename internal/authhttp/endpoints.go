package authhttp

import (
	"log/slog"
	"net/http"

	"github.com/donaldgifford/docz-api/internal/auth"
	"github.com/donaldgifford/docz-api/internal/session"
	"github.com/donaldgifford/docz-api/internal/store"
)

// defaultProvider is used when /auth/login carries no provider query parameter.
const defaultProvider = "github"

// login begins the OAuth/OIDC flow: it builds a signed state (carrying the
// provider) and redirects to the provider's authorization URL.
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("provider")
	if name == "" {
		name = defaultProvider
	}
	provider, ok := h.registry.Get(name)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown or disabled auth provider")
		return
	}

	state, err := auth.EncodeState(h.stateSecret, name)
	if err != nil {
		serverError(w, "encode state", err)
		return
	}
	http.Redirect(w, r, provider.AuthCodeURL(state), http.StatusFound)
}

// callback completes the flow: verify state, Exchange the code for an Identity,
// upsert the durable user row, issue a session, set the cookie, and redirect to
// the app root.
func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if provErr := q.Get("error"); provErr != "" {
		slog.Warn("auth provider returned an error at callback", "error", provErr)
		writeError(w, http.StatusUnauthorized, "authorization denied")
		return
	}
	code, state := q.Get("code"), q.Get("state")
	if code == "" || state == "" {
		writeError(w, http.StatusBadRequest, "missing code or state")
		return
	}

	providerName, err := auth.VerifyState(h.stateSecret, state)
	if err != nil {
		slog.Warn("rejecting callback with invalid state", "err", err)
		writeError(w, http.StatusBadRequest, "invalid state")
		return
	}
	provider, ok := h.registry.Get(providerName)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown provider in state")
		return
	}

	identity, err := provider.Exchange(r.Context(), code)
	if err != nil {
		slog.Error("provider token exchange failed", "provider", providerName, "err", err)
		writeError(w, http.StatusUnauthorized, "authentication failed")
		return
	}

	if _, uerr := h.users.UpsertUser(r.Context(), store.UserInput{
		Provider: identity.Provider,
		Subject:  identity.Subject,
		Email:    identity.Email,
		Login:    identity.Login,
	}); uerr != nil {
		serverError(w, "upsert user", uerr)
		return
	}

	sessionID, err := h.sessions.Issue(r.Context(), identity)
	if err != nil {
		serverError(w, "issue session", err)
		return
	}
	h.sessions.SetCookie(w, sessionID)
	slog.Info("site user logged in", "provider", identity.Provider, "subject", identity.Subject)
	http.Redirect(w, r, "/", http.StatusFound)
}

// getSession returns the current user. It runs behind the session middleware,
// so a valid session is present; the defensive 401 guards a misconfigured
// mount. It reads only from the request context, so it needs no receiver.
func (*Handler) getSession(w http.ResponseWriter, r *http.Request) {
	sess, ok := session.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	writeJSON(w, sessionDTO{
		Provider: sess.Identity.Provider,
		Subject:  sess.Identity.Subject,
		Email:    sess.Identity.Email,
		Login:    sess.Identity.Login,
		Groups:   sess.Identity.Groups,
	})
}

// logout revokes the current session and clears the cookie. It is idempotent:
// a request without a session still clears the cookie and returns 200.
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if sess, ok := session.FromContext(r.Context()); ok {
		if err := h.sessions.Revoke(r.Context(), sess.ID); err != nil {
			serverError(w, "revoke session", err)
			return
		}
	}
	h.sessions.ClearCookie(w)
	writeJSON(w, map[string]string{"status": "logged out"})
}
