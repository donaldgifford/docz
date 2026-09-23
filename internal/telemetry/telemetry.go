// Package telemetry wires the docz-api observability stack (OQ 8): OpenTelemetry
// tracing exported over OTLP/HTTP, Prometheus metrics on /metrics, and the
// slog request-logging middleware. Setup is called once from main(); the HTTP
// middlewares and metric helpers are used across the router, queue worker, and
// ingest pipeline.
//
// Everything degrades to a no-op when unconfigured: with no OTLP endpoint the
// global tracer stays the OpenTelemetry no-op (spans are created but not
// exported), so a homelab install without a collector pays no overhead and
// needs no telemetry configuration. Trace context still propagates across the
// HTTP and queue boundaries either way, so turning on a collector later lights
// up end-to-end traces without code changes.
package telemetry

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Config is the telemetry configuration, mapped from config.TelemetryConfig so
// this package stays decoupled from internal/config. The zero value is valid:
// an empty OTLPEndpoint disables trace export.
type Config struct {
	ServiceName    string  // resource service.name; defaults to "docz-api" if empty.
	OTLPEndpoint   string  // OTLP/HTTP collector host:port; empty disables tracing.
	SampleRate     float64 // head-sampling ratio, clamped to [0, 1].
	MetricsEnabled bool    // whether the caller mounts /metrics.
}

// defaultServiceName labels spans when none is configured.
const defaultServiceName = "docz-api"

// Setup installs the global W3C trace-context propagator and, when an OTLP
// endpoint is configured, a batching TracerProvider that exports over OTLP/HTTP.
// It returns a shutdown function that flushes pending spans; the function is
// safe to call even when tracing is disabled. Setup does no network I/O — the
// OTLP/HTTP exporter connects lazily on first export — so it never blocks
// startup even if the collector is unreachable.
func Setup(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	// Always install the propagator so trace context flows across the HTTP and
	// queue boundaries; when tracing is off the spans are simply non-recording.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if cfg.OTLPEndpoint == "" {
		slog.Info("tracing disabled (no OTEL_EXPORTER_OTLP_ENDPOINT set); metrics unaffected")
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("otlp trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewSchemaless(
			attribute.String("service.name", serviceName(cfg.ServiceName)),
		)),
		sdktrace.WithSampler(sdktrace.ParentBased(
			sdktrace.TraceIDRatioBased(clampRate(cfg.SampleRate)),
		)),
	)
	otel.SetTracerProvider(tp)
	slog.Info("tracing enabled",
		"endpoint", cfg.OTLPEndpoint, "sample_rate", clampRate(cfg.SampleRate))
	return tp.Shutdown, nil
}

// serviceName falls back to the default when unset.
func serviceName(name string) string {
	if name == "" {
		return defaultServiceName
	}
	return name
}

// clampRate bounds a sampling ratio to [0, 1] so a misconfigured value can
// never disable sampling or exceed full sampling.
func clampRate(r float64) float64 {
	switch {
	case r < 0:
		return 0
	case r > 1:
		return 1
	default:
		return r
	}
}
