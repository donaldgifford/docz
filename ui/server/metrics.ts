/*
 * Prometheus metrics (DESIGN-0006 Component 5).
 *
 * Everything here labels from route-class.ts and nowhere else. Metric
 * labels are the one place where unbounded input is not merely noisy
 * but expensive — every distinct label combination is a stored series
 * forever — and SPA paths are unbounded by construction. The closed
 * union caps the series count at roughly
 * 8 methods x 8 routes x ~10 statuses, a few hundred, regardless of
 * what anyone requests.
 *
 * An explicit Registry rather than the global default: the default is
 * process-wide shared state, which makes tests order-dependent and
 * makes "what does this process expose" unanswerable by reading one
 * file.
 */
import {
  collectDefaultMetrics,
  Counter,
  Histogram,
  Registry,
} from "prom-client";

import type { RouteClass } from "./route-class";

/** Reasons a proxy request can fail, closed like every other label. */
export type ProxyErrorReason = "unreachable" | "not_configured";

export interface Metrics {
  readonly registry: Registry;
  recordRequest(
    method: string,
    route: RouteClass,
    status: number,
    durationMs: number,
  ): void;
  recordProxy(route: RouteClass, durationMs: number): void;
  recordProxyError(reason: ProxyErrorReason): void;
}

/**
 * Buckets for request latency. Tuned for a static-asset server in front
 * of an API proxy — most responses are a file read (sub-millisecond),
 * the tail is whatever docz-api takes.
 */
const DURATION_BUCKETS = [0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 2.5, 5, 10];

export function createMetrics(): Metrics {
  const registry = new Registry();

  // Process and runtime metrics. Free, and the only reason anyone
  // needs to scrape a static server at 3am.
  collectDefaultMetrics({ register: registry });

  const httpRequests = new Counter({
    name: "docz_site_http_requests_total",
    help: "Total HTTP requests served, by method, route class, and status.",
    labelNames: ["method", "route", "status"] as const,
    registers: [registry],
  });

  const httpDuration = new Histogram({
    name: "docz_site_http_request_duration_seconds",
    help: "HTTP request duration in seconds, by method and route class.",
    labelNames: ["method", "route"] as const,
    buckets: DURATION_BUCKETS,
    registers: [registry],
  });

  const proxyDuration = new Histogram({
    name: "docz_site_proxy_duration_seconds",
    help: "Upstream docz-api request duration in seconds, by route class.",
    labelNames: ["route"] as const,
    buckets: DURATION_BUCKETS,
    registers: [registry],
  });

  // The one that matters most: this is the docz-api health signal that
  // replaces the readiness check we deliberately do NOT do. It can page
  // a human without evicting a pod from the Service.
  const proxyErrors = new Counter({
    name: "docz_site_proxy_errors_total",
    help: "Failed upstream requests, by reason.",
    labelNames: ["reason"] as const,
    registers: [registry],
  });

  return {
    registry,
    recordRequest(method, route, status, durationMs) {
      httpRequests.inc({ method, route, status: String(status) });
      httpDuration.observe({ method, route }, durationMs / 1000);
    },
    recordProxy(route, durationMs) {
      proxyDuration.observe({ route }, durationMs / 1000);
    },
    recordProxyError(reason) {
      proxyErrors.inc({ reason });
    },
  };
}
