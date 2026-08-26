package session

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/donaldgifford/docz-api/internal/auth"
)

// ctxKey is the unexported context key for the resolved Session.
type ctxKey struct{}

// lookuper is the narrow surface Middleware needs; *Store satisfies it. Keeping
// it an interface lets the middleware be tested without a live Redis.
type lookuper interface {
	Lookup(ctx context.Context, sessionID string) (Session, error)
}

var _ lookuper = (*Store)(nil)

// Middleware resolves the session cookie into a Session and injects it into the
// request context. A request with no cookie or an invalid/expired session gets
// 401 and never reaches the next handler — this is the authentication gate for
// the protected routes.
func Middleware(store lookuper) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cookieName)
			if err != nil {
				writeUnauthorized(w)
				return
			}
			sess, err := store.Lookup(r.Context(), cookie.Value)
			if err != nil {
				if !isUnusableSession(err) {
					// The backend could not answer at all — Redis unreachable.
					// That is an outage, not a verdict on the caller: answering
					// 401 would log every user out and leave no trace of why
					// (INV-0007 F7.2).
					slog.ErrorContext(r.Context(), "session lookup failed",
						"err", err, "path", r.URL.Path)
					writeSessionUnavailable(w)
					return
				}
				if errors.Is(err, ErrSessionCorrupt) {
					// Worth a log — it means stored sessions stopped matching
					// the code that reads them — but still a 401, because
					// logging in again is the only way out and /auth/logout
					// sits behind this very gate.
					slog.WarnContext(r.Context(), "discarding an undecodable session",
						"err", err, "path", r.URL.Path)
				}
				writeUnauthorized(w)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKey{}, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Anonymous identity constants for the no-auth mode (AUTH_PROVIDERS=none).
// AnonymousSubject doubles as the session id, which never exists in Redis:
// there is nothing to look up and nothing to revoke, so logout is a harmless
// no-op delete of an absent key.
const (
	AnonymousProvider = "none"
	AnonymousSubject  = "anonymous"
)

// AnonymousMiddleware injects a synthetic Session on every request instead of
// resolving a cookie, which is what makes AUTH_PROVIDERS=none work: the whole
// /api/v1 surface stays mounted and FromContext keeps returning an identity, so
// no handler needs a no-auth branch. It is the only middleware in this package
// that never rejects a request — the read API is open under it, by design.
func AnonymousMiddleware() func(http.Handler) http.Handler {
	sess := Session{
		ID: AnonymousSubject,
		Identity: auth.Identity{
			Provider: AnonymousProvider,
			Subject:  AnonymousSubject,
			Login:    AnonymousSubject,
		},
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, sess)))
		})
	}
}

// isUnusableSession reports whether err means "this caller has no valid
// session" — as opposed to "the session backend could not answer". Only the
// latter warrants a 503; everything else is a 401 the holder can recover from
// by logging in again.
func isUnusableSession(err error) bool {
	return errors.Is(err, ErrSessionNotFound) || errors.Is(err, ErrSessionCorrupt)
}

// FromContext returns the Session injected by Middleware and whether one was
// present.
func FromContext(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(ctxKey{}).(Session)
	return s, ok
}

// writeUnauthorized writes the 401 JSON envelope used by the auth gate.
func writeUnauthorized(w http.ResponseWriter) {
	writeJSONStatus(w, http.StatusUnauthorized, `{"error":"authentication required"}`)
}

// writeSessionUnavailable reports that the session backend could not answer, as
// distinct from the caller being unauthenticated. 503 keeps clients from
// treating a Redis blip as a logout: docz-site drives its login UI off 401, so
// answering 401 here would bounce every signed-in user to the login screen for
// the duration of the outage.
func writeSessionUnavailable(w http.ResponseWriter) {
	writeJSONStatus(w, http.StatusServiceUnavailable, `{"error":"session unavailable"}`)
}

// writeJSONStatus writes a fixed JSON body with the given status. A failed
// write means the client is gone, which is nothing the server can act on.
func writeJSONStatus(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(body)); err != nil {
		slog.Debug("response write failed", "err", err, "status", status)
	}
}
