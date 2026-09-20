/*
 * Prometheus metrics (DESIGN-0006 Component 5).
 *
 * The load-bearing test here is the cardinality one. Everything else
 * checks that the endpoint works; that one checks that it cannot be
 * weaponised, because every distinct label combination is a series
 * stored forever and SPA paths are attacker-supplied.
 */
import { describe, expect, test } from "bun:test";

import { createMetrics } from "./metrics";
import { resolveMetricsEnabled } from "./serve";
import { classifyRoute, normalizeMethod } from "./route-class";

/** Names of the instruments DESIGN-0006 specifies. */
const INSTRUMENTS = [
  "docz_site_http_requests_total",
  "docz_site_http_request_duration_seconds",
  "docz_site_proxy_duration_seconds",
  "docz_site_proxy_errors_total",
];

describe("resolveMetricsEnabled", () => {
  test("defaults ON when unset or empty, matching docz-api", () => {
    expect(resolveMetricsEnabled(undefined)).toBe(true);
    expect(resolveMetricsEnabled("")).toBe(true);
    expect(resolveMetricsEnabled("   ")).toBe(true);
  });

  test("only an explicit falsey word turns it off", () => {
    for (const off of ["false", "FALSE", " off ", "0", "no"]) {
      expect(resolveMetricsEnabled(off)).toBe(false);
    }
  });

  test("a typo leaves metrics ON — the harmless direction", () => {
    // An endpoint nobody scrapes costs nothing; a silently dark one
    // costs a blind spot during an incident.
    expect(resolveMetricsEnabled("flase")).toBe(true);
    expect(resolveMetricsEnabled("disabled")).toBe(true);
  });
});

describe("exposition", () => {
  test("includes every instrument once recorded", async () => {
    const metrics = createMetrics();
    metrics.recordRequest("GET", "spa", 200, 12);
    metrics.recordProxy("proxy:api", 40);
    metrics.recordProxyError("unreachable");

    const text = await metrics.registry.metrics();
    for (const name of INSTRUMENTS) {
      expect(text).toContain(name);
    }
  });

  test("carries HELP and TYPE lines, so it parses as exposition", async () => {
    const metrics = createMetrics();
    metrics.recordRequest("GET", "static", 200, 1);
    const text = await metrics.registry.metrics();

    expect(text).toContain("# HELP docz_site_http_requests_total");
    expect(text).toContain("# TYPE docz_site_http_requests_total counter");
    expect(text).toContain(
      "# TYPE docz_site_http_request_duration_seconds histogram",
    );
  });

  test("includes default process metrics", async () => {
    const text = await createMetrics().registry.metrics();
    expect(text).toContain("process_cpu_user_seconds_total");
  });

  test("labels carry the values recorded", async () => {
    const metrics = createMetrics();
    metrics.recordRequest("POST", "proxy:auth", 302, 5);
    const text = await metrics.registry.metrics();

    expect(text).toContain('method="POST"');
    expect(text).toContain('route="proxy:auth"');
    expect(text).toContain('status="302"');
  });

  test("proxy error reasons are the two closed values", async () => {
    const metrics = createMetrics();
    metrics.recordProxyError("unreachable");
    metrics.recordProxyError("not_configured");
    const text = await metrics.registry.metrics();

    expect(text).toContain('reason="unreachable"');
    expect(text).toContain('reason="not_configured"');
  });

  test("durations are recorded in SECONDS, not milliseconds", async () => {
    // The unit is in the metric name, so getting it wrong makes every
    // dashboard silently wrong by 1000x rather than visibly broken.
    const metrics = createMetrics();
    metrics.recordRequest("GET", "spa", 200, 2000);
    const value = await metrics.registry.getSingleMetricAsString(
      "docz_site_http_request_duration_seconds",
    );
    expect(value).toContain('_sum{method="GET",route="spa"} 2');
  });

  test("registries are independent, so tests cannot leak into each other", async () => {
    const a = createMetrics();
    const b = createMetrics();
    a.recordProxyError("unreachable");

    expect(
      await b.registry.getSingleMetricAsString("docz_site_proxy_errors_total"),
    ).not.toContain('reason="unreachable"');
  });
});

describe("cardinality is bounded by construction", () => {
  /** Count the series lines in an exposition payload. */
  function seriesCount(text: string): number {
    return text.split("\n").filter((line) => line.startsWith("docz_site_"))
      .length;
  }

  test("10 000 hostile paths and methods produce a bounded series count", async () => {
    const metrics = createMetrics();
    const methods = ["GET", "POST", "PROPFIND", "🙂", "x".repeat(200)];

    for (let i = 0; i < 10_000; i += 1) {
      const path = `/owner/repo/design/DESIGN-${String(i).padStart(5, "0")}`;
      const method = methods[i % methods.length] ?? "GET";
      metrics.recordRequest(
        normalizeMethod(method),
        classifyRoute(path, false),
        200,
        1,
      );
    }

    const text = await metrics.registry.metrics();
    // 10 000 distinct paths collapsed to a single `spa` route label;
    // five methods collapsed to three (GET, POST, other).
    expect(seriesCount(text)).toBeLessThan(100);
  });

  test("no request path or method text appears in the exposition", async () => {
    const metrics = createMetrics();
    const hostile = '/a/../../etc/shadow?x=1"><script>';
    metrics.recordRequest(
      normalizeMethod("EVIL-METHOD"),
      classifyRoute(hostile, false),
      200,
      1,
    );

    // Scope the check to OUR series. prom-client's own HELP text is not
    // ours to police, and "Number of open file descriptors" contains the
    // substring "script" — which made a whole-text assertion pass on
    // macOS and fail on Linux, since process_open_fds is /proc-only.
    const ours = (await metrics.registry.metrics())
      .split("\n")
      .filter((line) => line.startsWith("docz_site_"))
      .join("\n");

    expect(ours).not.toContain("etc/shadow");
    expect(ours).not.toContain("script");
    expect(ours).not.toContain("EVIL-METHOD");
    expect(ours).toContain('method="other"');
    expect(ours).toContain('route="spa"');
  });

  test("the cardinality guard would FAIL on an unbounded label", async () => {
    // Verify the guard fires before trusting it green: record the raw
    // path as the route label and confirm the count explodes past the
    // bound the test above asserts.
    const metrics = createMetrics();
    for (let i = 0; i < 200; i += 1) {
      metrics.recordRequest("GET", `/unbounded/${String(i)}` as never, 200, 1);
    }
    const text = await metrics.registry.metrics();
    expect(seriesCount(text)).toBeGreaterThan(100);
  });
});
