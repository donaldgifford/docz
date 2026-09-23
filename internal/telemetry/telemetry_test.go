package telemetry

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestSetupNoopWhenNoEndpoint(t *testing.T) {
	shutdown, err := Setup(context.Background(), Config{ServiceName: "test"})
	if err != nil {
		t.Fatalf("Setup with no endpoint: %v", err)
	}
	if shutdown == nil {
		t.Fatal("Setup returned a nil shutdown func")
	}
	if serr := shutdown(context.Background()); serr != nil {
		t.Errorf("no-op shutdown returned %v, want nil", serr)
	}

	// Setup must install the W3C propagator regardless of export, so trace
	// context can still cross the HTTP/queue boundaries.
	carrier := propagation.MapCarrier{}
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x1},
		SpanID:     trace.SpanID{0x2},
		TraceFlags: trace.FlagsSampled,
	}))
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	if carrier["traceparent"] == "" {
		t.Error("propagator did not inject a traceparent after Setup")
	}
}

func TestClampRate(t *testing.T) {
	tests := []struct {
		in, want float64
	}{
		{-1, 0}, {0, 0}, {0.25, 0.25}, {1, 1}, {2, 1},
	}
	for _, tc := range tests {
		if got := clampRate(tc.in); got != tc.want {
			t.Errorf("clampRate(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestServiceName(t *testing.T) {
	if got := serviceName(""); got != defaultServiceName {
		t.Errorf("serviceName(\"\") = %q, want %q", got, defaultServiceName)
	}
	if got := serviceName("custom"); got != "custom" {
		t.Errorf("serviceName(\"custom\") = %q, want custom", got)
	}
}

func TestRequestLoggerSkipsProbes(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	handler := RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// A probe path is not logged.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil))
	if buf.Len() != 0 {
		t.Errorf("probe request was logged: %q", buf.String())
	}

	// A real request is logged with method and status.
	handler.ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/repos", nil),
	)
	line := buf.String()
	if !strings.Contains(line, "http request") || !strings.Contains(line, "status=200") {
		t.Errorf("request log = %q, want an http request line with status=200", line)
	}
}

func TestInstrumentRecordsHTTPMetrics(t *testing.T) {
	r := chi.NewRouter()
	r.Use(Instrument())
	r.Get("/api/v1/repos/{owner}/{name}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/api/v1/repos/acme/platform", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	body := scrapeMetrics(t)
	// The route label must be the chi template, not the expanded path.
	if !strings.Contains(body, `docz_api_http_requests_total{method="GET",route="/api/v1/repos/{owner}/{name}",status="200"}`) {
		t.Errorf("metrics missing the templated HTTP counter; got:\n%s", body)
	}
	if strings.Contains(body, "acme/platform") {
		t.Error("metrics leaked the expanded URL as a label (cardinality risk)")
	}
}

func TestObserveIngestRecorded(t *testing.T) {
	ObserveIngest("push", "success", 0)
	body := scrapeMetrics(t)
	if !strings.Contains(body, `docz_api_ingest_jobs_total{reason="push",status="success"}`) {
		t.Errorf("metrics missing the ingest counter; got:\n%s", body)
	}
}

func TestRoutePatternFallback(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/no/route/ctx", nil)
	if got := routePattern(req); got != unmatchedRoute {
		t.Errorf("routePattern with no chi context = %q, want %q", got, unmatchedRoute)
	}
}

// scrapeMetrics fetches the Prometheus exposition from MetricsHandler.
func scrapeMetrics(t *testing.T) string {
	t.Helper()
	rec := httptest.NewRecorder()
	MetricsHandler().ServeHTTP(
		rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}
