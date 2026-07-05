package session

import (
	"context"
	"log/slog"
	"net/http"
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
				writeUnauthorized(w)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKey{}, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// FromContext returns the Session injected by Middleware and whether one was
// present.
func FromContext(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(ctxKey{}).(Session)
	return s, ok
}

// writeUnauthorized writes the 401 JSON envelope used by the auth gate.
func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	if _, err := w.Write([]byte(`{"error":"authentication required"}`)); err != nil {
		slog.Debug("unauthorized response write failed", "err", err)
	}
}
