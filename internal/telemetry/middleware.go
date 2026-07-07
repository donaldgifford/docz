package telemetry

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// serverErrorFloor is the status at and above which a server span is marked as
// an error, matching the OTel HTTP semantic conventions (5xx = error).
const serverErrorFloor = 500

// tracerName is the instrumentation scope for the HTTP server spans.
const tracerName = "github.com/donaldgifford/docz-api/internal/telemetry"

// unmatchedRoute labels requests that matched no chi route (e.g. a stray path),
// keeping the metric/trace route label bounded instead of echoing raw URLs.
const unmatchedRoute = "unmatched"

// skipPaths are the operational probes excluded from request logging and
// tracing: high-frequency, low-signal, and self-referential for /metrics.
var skipPaths = map[string]bool{
	"/healthz": true,
	"/readyz":  true,
	"/metrics": true,
}

// RequestLogger writes one structured slog line per request (method, path,
// status, bytes, duration, request id). Probe paths are skipped to keep the
// log stream signal-dense. Mount it after middleware.RequestID (so the request
// id is present) and outside Recoverer (so panics still get logged as 500s).
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skipPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)
			logger.LogAttrs(r.Context(), slog.LevelInfo, "http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.Status()),
				slog.Int("bytes", ww.BytesWritten()),
				slog.Duration("duration", time.Since(start)),
				slog.String("request_id", middleware.GetReqID(r.Context())),
			)
		})
	}
}

// Instrument starts an OpenTelemetry server span for each request and records
// the Prometheus HTTP metrics. Incoming W3C trace context is extracted so the
// span joins an upstream trace when present. The span name and route label use
// chi's matched route template (e.g. "/api/v1/repos/{owner}/{name}") rather
// than the expanded URL, so trace and metric cardinality stay bounded. Probe
// paths are skipped.
func Instrument() func(http.Handler) http.Handler {
	tracer := otel.Tracer(tracerName)
	propagator := otel.GetTextMapPropagator()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skipPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			ctx, span := tracer.Start(ctx, r.Method,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("http.request.method", r.Method),
					attribute.String("url.path", r.URL.Path),
				),
			)
			defer span.End()

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r.WithContext(ctx))

			route := routePattern(r)
			status := ww.Status()
			span.SetName(r.Method + " " + route)
			span.SetAttributes(
				attribute.String("http.route", route),
				attribute.Int("http.response.status_code", status),
			)
			// Mark 5xx as an error so failed requests aren't rendered as
			// successful in trace tooling (4xx is a client fault, not a span error).
			if status >= serverErrorFloor {
				span.SetStatus(codes.Error, http.StatusText(status))
			}
			observeHTTP(r.Method, route, status, time.Since(start))
		})
	}
}

// routePattern returns chi's matched route template for r, populated once the
// request has been routed. It falls back to unmatchedRoute when nothing
// matched, guarding metric and trace label cardinality.
func routePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if p := rc.RoutePattern(); p != "" {
			return p
		}
	}
	return unmatchedRoute
}
