/*
 * OpenTelemetry tracing (DESIGN-0006 Component 6).
 *
 * HAND-INSTRUMENTED, and that is a security decision rather than a
 * stylistic one. OTel's HTTP auto-instrumentation records `url.full`
 * by default; on a server whose whole job is proxying `/auth/*`, that
 * means shipping OAuth authorization codes to a collector (INV-0006
 * F7). No auto-instrumentation package is installed, and none should
 * ever be — a test asserts that.
 *
 * Unconfigured means OFF: with no endpoint we never register a
 * provider, so the global tracer stays OTel's no-op. Spans are still
 * "created" at the call sites, cost nothing, and go nowhere. That
 * mirrors docz-api's degrades-to-nothing property, which is what makes
 * this safe to ship enabled-by-default-when-configured.
 */
import { trace, type Tracer } from "@opentelemetry/api";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { resourceFromAttributes } from "@opentelemetry/resources";
import {
  BatchSpanProcessor,
  NodeTracerProvider,
  TraceIdRatioBasedSampler,
} from "@opentelemetry/sdk-trace-node";
import {
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
} from "@opentelemetry/semantic-conventions";

/** Name reported to the collector when nothing else is configured. */
export const DEFAULT_SERVICE_NAME = "docz-site";

/** Service-name charset, so no env text reaches a collector unchecked. */
const SERVICE_NAME_RE = /^[\w.-]{1,64}$/;

/** Resolve OTEL_SERVICE_NAME; garbage or empty → "docz-site". */
export function resolveServiceName(raw: string | undefined): string {
  const value = (raw ?? "").trim();
  return SERVICE_NAME_RE.test(value) ? value : DEFAULT_SERVICE_NAME;
}

/**
 * Resolve the OTLP endpoint. Only absolute http(s) URLs are accepted;
 * anything else means "unconfigured", which means no export at all.
 *
 * Failing closed matters more here than for a log level: this value
 * decides where request telemetry is SENT, so a malformed one must
 * never be coerced into some other destination.
 */
export function resolveOtelEndpoint(
  raw: string | undefined,
): string | undefined {
  const value = (raw ?? "").trim();
  if (value === "") {
    return undefined;
  }
  try {
    const url = new URL(value);
    return url.protocol === "http:" || url.protocol === "https:"
      ? value
      : undefined;
  } catch {
    return undefined;
  }
}

/**
 * Resolve the head sample rate, CLAMPED to [0, 1]. A rate outside that
 * range is not an error the server should die on, but it must not be
 * handed to the sampler — `TraceIdRatioBasedSampler` treats out-of-range
 * input inconsistently, and "sample 500% of traces" has no meaning.
 * Unparseable → 1 (sample everything), the least surprising default for
 * someone who just turned tracing on.
 */
export function resolveSampleRate(raw: string | undefined): number {
  const text = (raw ?? "").trim();
  // Number("") is 0, not NaN — without this an unset rate would sample
  // NOTHING, which is a silent, total loss of traces.
  if (text === "") {
    return 1;
  }
  const value = Number(text);
  if (!Number.isFinite(value)) {
    return 1;
  }
  return Math.min(1, Math.max(0, value));
}

export interface TracingOptions {
  endpoint: string | undefined;
  serviceName: string;
  sampleRate: number;
  version?: string;
}

export interface TracingHandle {
  /** True when a provider was registered and spans will be exported. */
  readonly enabled: boolean;
  readonly tracer: Tracer;
  shutdown(): Promise<void>;
}

/**
 * Register the tracer provider, or do nothing at all when unconfigured.
 *
 * Returns a handle either way, so callers never branch on whether
 * tracing is on — they just start spans, which are no-ops when it is
 * off.
 */
export function initTracing(options: TracingOptions): TracingHandle {
  const { endpoint, serviceName, sampleRate, version } = options;

  if (endpoint === undefined) {
    return {
      enabled: false,
      tracer: trace.getTracer(serviceName),
      shutdown: () => Promise.resolve(),
    };
  }

  const provider = new NodeTracerProvider({
    resource: resourceFromAttributes({
      [ATTR_SERVICE_NAME]: serviceName,
      ...(version === undefined ? {} : { [ATTR_SERVICE_VERSION]: version }),
    }),
    sampler: new TraceIdRatioBasedSampler(sampleRate),
    spanProcessors: [
      new BatchSpanProcessor(new OTLPTraceExporter({ url: endpoint })),
    ],
  });
  // register() installs the W3C TraceContext propagator, which is what
  // lets us inject a traceparent docz-api already knows how to extract.
  provider.register();

  return {
    enabled: true,
    tracer: trace.getTracer(serviceName),
    shutdown: () => provider.shutdown(),
  };
}
