package mcp

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/httprate"

	"github.com/donaldgifford/docz/v2/internal/session"
	"github.com/donaldgifford/docz/v2/internal/telemetry"
)

// The surface labels of docz_api_rate_limited_total.
const (
	surfaceMCP = "mcp"
	surfaceAPI = "api"
)

// Limiter is the per-principal rate limiter (DESIGN-0023 §8): one window
// of a minute, MCP_RATE_LIMIT requests in it, in process and per replica,
// so the effective limit is the configured one times the replica count.
// One Limiter serves both surfaces, so a token caller's /mcp and /api/v1
// requests draw on one budget.
type Limiter struct {
	rl *httprate.RateLimiter
}

// NewLimiter builds a Limiter allowing perMinute requests per key. The key
// is the session.s principal key when the session has one, and the client
// IP for the anonymous principal under
// AUTH_PROVIDERS=none, where every caller would otherwise share one bucket
// (IMPL-0026 question 7). A refused request is 429 with Retry-After and
// counts in docz_api_rate_limited_total{surface,via}.
func NewLimiter(perMinute int) *Limiter {
	return &Limiter{rl: httprate.NewRateLimiter(perMinute, time.Minute,
		httprate.WithKeyFuncs(principalKey),
		httprate.WithLimitHandler(limited),
	)}
}

// MCP limits every request to /mcp.
func (l *Limiter) MCP(next http.Handler) http.Handler {
	return l.rl.Handler(next)
}

// API limits a request to /api/v1 only when the caller is a token: the
// cookie path is docz-site, one page load of many calls, and is not
// limited (DESIGN-0023 §8).
func (l *Limiter) API(next http.Handler) http.Handler {
	limited := l.rl.Handler(next)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sess, ok := session.FromContext(r.Context()); ok && sess.Via == session.ViaToken {
			limited.ServeHTTP(w, r)

			return
		}
		next.ServeHTTP(w, r)
	})
}

// principalKey is the limiter's key function.
func principalKey(r *http.Request) (string, error) {
	sess, ok := session.FromContext(r.Context())
	if !ok || sess.Via == session.ViaAnonymous {
		return clientIP(r), nil
	}

	return sess.PrincipalKey(), nil
}

// clientIP is the anonymous principal's key: the connection's peer, which
// net/http sets and no header can forge. httprate's own key functions are
// deprecated because neither a forwarding header nor the peer is right
// everywhere; the peer is the one a client cannot choose.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	return httprate.CanonicalizeIP(host)
}

// limited is the 429 handler. httprate has set Retry-After already; the
// body is the REST error envelope and the metric is the signal.
func limited(w http.ResponseWriter, r *http.Request) {
	via := session.ViaAnonymous.String()
	if sess, ok := session.FromContext(r.Context()); ok {
		via = sess.Via.String()
	}
	telemetry.ObserveRateLimited(surfaceOf(r), via)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	if _, err := w.Write([]byte(`{"error":"rate limited"}` + "\n")); err != nil {
		slog.Debug("rate limit response write failed", "err", err)
	}
}

// surfaceOf labels the request by path.
func surfaceOf(r *http.Request) string {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		return surfaceAPI
	}

	return surfaceMCP
}
