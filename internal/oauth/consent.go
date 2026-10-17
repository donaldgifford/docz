package oauth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/donaldgifford/docz/v2/internal/store"
)

// AuthorizationView is what the consent page renders: the client, what it
// asked for, and what the person can actually grant it.
type AuthorizationView struct {
	ClientID        string   `json:"client_id"`
	ClientName      string   `json:"client_name"`
	ClientURI       string   `json:"client_uri,omitempty"`
	ScopesRequested []string `json:"scopes_requested"`
	ScopesGranted   []string `json:"scopes_granted"`
}

// Decision is the consent page's answer.
type Decision struct {
	Decision string `json:"decision"`
}

// DecisionResult is where the page sends the browser next.
type DecisionResult struct {
	RedirectTo string `json:"redirect_to"`
}

// getAuthorization is GET /api/v1/oauth/authorizations/{id}: the pending
// request, bound to this session on first read. Unknown, expired, and
// another session's are all 404, so the id is the only secret.
func (s *Server) getAuthorization(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	principal, ok := s.session(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "authentication required")

		return
	}

	id := chi.URLParam(r, "id")
	p, found := s.loadPending(r, id)
	if !found || (p.SessionID != "" && p.SessionID != principal.SessionID) {
		writeAPIError(w, http.StatusNotFound, "authorization not found")

		return
	}

	if p.SessionID == "" {
		p.SessionID = principal.SessionID
		if err := s.put(r.Context(), pendingPrefix+id, p, time.Until(p.ExpiresAt)); err != nil {
			slog.ErrorContext(r.Context(), "oauth consent: bind pending request", "err", err)
			writeAPIError(w, http.StatusServiceUnavailable, "could not store the request")

			return
		}
	}

	writeJSON(w, http.StatusOK, AuthorizationView{
		ClientID: p.ClientID, ClientName: p.ClientName, ClientURI: p.ClientURI,
		ScopesRequested: p.Scopes, ScopesGranted: intersect(p.Scopes, principal.Scopes),
	})
}

// decideAuthorization is POST /api/v1/oauth/authorizations/{id}: allow
// mints a single-use code and remembers the consent; deny sends the client
// access_denied. The id is consumed either way.
func (s *Server) decideAuthorization(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	principal, ok := s.session(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "authentication required")

		return
	}

	var d Decision
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&d); err != nil || (d.Decision != "allow" && d.Decision != "deny") {
		writeAPIError(w, http.StatusBadRequest, `decision must be "allow" or "deny"`)

		return
	}

	id := chi.URLParam(r, "id")
	p, found := s.loadPending(r, id)
	if !found || p.SessionID == "" || p.SessionID != principal.SessionID {
		writeAPIError(w, http.StatusNotFound, "authorization not found")

		return
	}
	if _, err := s.cache.Take(r.Context(), pendingPrefix+id); err != nil {
		// Raced by another decision: consumed is 404, as a replay is.
		writeAPIError(w, http.StatusNotFound, "authorization not found")

		return
	}

	if d.Decision == "deny" {
		writeJSON(w, http.StatusOK, DecisionResult{RedirectTo: s.redirectWith(p.RedirectURI, url.Values{
			paramError: {ErrorAccessDenied}, paramState: {p.State},
		})})

		return
	}

	userID, err := s.store.UpsertUser(r.Context(), store.UserInput{
		Provider: principal.Identity.Provider, Subject: principal.Identity.Subject,
		Email: principal.Identity.Email, Login: principal.Identity.Login,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "oauth consent: upsert user", "err", err)
		writeAPIError(w, http.StatusServiceUnavailable, "could not record the decision")

		return
	}

	granted := intersect(p.Scopes, principal.Scopes)
	raw, err := randomSecret()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "could not mint a code")

		return
	}
	c := code{Pending: p, UserID: userID, Identity: principal.Identity, Granted: granted}
	if err := s.put(r.Context(), codePrefix+hashToken(raw), c, codeTTL); err != nil {
		slog.ErrorContext(r.Context(), "oauth consent: store code", "err", err)
		writeAPIError(w, http.StatusServiceUnavailable, "could not store the code")

		return
	}
	if err := s.store.UpsertConsent(r.Context(), userID, p.ClientID, joinScopes(granted)); err != nil {
		slog.ErrorContext(r.Context(), "oauth consent: upsert consent", "err", err)
	}

	writeJSON(w, http.StatusOK, DecisionResult{RedirectTo: s.redirectWith(p.RedirectURI, url.Values{
		paramCode: {raw}, paramState: {p.State},
	})})
}

// loadPending reads a pending request that has not expired.
func (s *Server) loadPending(r *http.Request, id string) (pending, bool) {
	if id == "" {
		return pending{}, false
	}
	raw, err := s.cache.Get(r.Context(), pendingPrefix+id)
	if err != nil {
		if !errors.Is(err, ErrCacheMiss) {
			slog.WarnContext(r.Context(), "oauth consent: read pending request", "err", err)
		}

		return pending{}, false
	}

	var p pending
	if json.Unmarshal(raw, &p) != nil || !s.now().Before(p.ExpiresAt) {
		return pending{}, false
	}

	return p, true
}

// writeAPIError is the /api/v1 error envelope, which the consent endpoints
// share with every other gated route.
func writeAPIError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
