/*
 * OpenTelemetry tracing (DESIGN-0006 Component 6).
 *
 * The resolvers are tested here; the span shape and the attribute
 * redaction gate live in tracing-spans.test.ts, which needs a registered
 * provider and therefore its own module registry.
 */
import { describe, expect, test } from "bun:test";

import {
  DEFAULT_SERVICE_NAME,
  initTracing,
  resolveOtelEndpoint,
  resolveSampleRate,
  resolveServiceName,
} from "./tracing";

describe("resolveOtelEndpoint", () => {
  test("unset or empty means unconfigured", () => {
    expect(resolveOtelEndpoint(undefined)).toBeUndefined();
    expect(resolveOtelEndpoint("")).toBeUndefined();
    expect(resolveOtelEndpoint("   ")).toBeUndefined();
  });

  test("accepts absolute http and https URLs", () => {
    expect(resolveOtelEndpoint("http://collector:4318/v1/traces")).toBe(
      "http://collector:4318/v1/traces",
    );
    expect(resolveOtelEndpoint(" https://otel.example.com ")).toBe(
      "https://otel.example.com",
    );
  });

  test("anything else fails CLOSED — no export rather than a guess", () => {
    // This value decides where request telemetry is SENT, so a
    // malformed one must never be coerced into some other destination.
    for (const bad of [
      "collector:4318",
      "/v1/traces",
      "ftp://collector",
      "file:///etc/passwd",
      "javascript:alert(1)",
      "not a url",
    ]) {
      expect(resolveOtelEndpoint(bad)).toBeUndefined();
    }
  });
});

describe("resolveServiceName", () => {
  test("defaults when unset, empty, or outside the charset", () => {
    expect(resolveServiceName(undefined)).toBe(DEFAULT_SERVICE_NAME);
    expect(resolveServiceName("")).toBe(DEFAULT_SERVICE_NAME);
    expect(resolveServiceName("has spaces")).toBe(DEFAULT_SERVICE_NAME);
    expect(resolveServiceName("x".repeat(65))).toBe(DEFAULT_SERVICE_NAME);
  });

  test("accepts sane names", () => {
    expect(resolveServiceName("docz-site")).toBe("docz-site");
    expect(resolveServiceName(" docz-site.staging ")).toBe("docz-site.staging");
    expect(resolveServiceName("docz_site_2")).toBe("docz_site_2");
  });
});

describe("resolveSampleRate", () => {
  test("defaults to 1 when unset or unparseable", () => {
    expect(resolveSampleRate(undefined)).toBe(1);
    expect(resolveSampleRate("")).toBe(1);
    expect(resolveSampleRate("always")).toBe(1);
  });

  test("passes through valid rates", () => {
    expect(resolveSampleRate("0")).toBe(0);
    expect(resolveSampleRate("0.1")).toBe(0.1);
    expect(resolveSampleRate("1")).toBe(1);
  });

  test("CLAMPS out-of-range values instead of passing them on", () => {
    // "Sample 500% of traces" has no meaning, and the ratio sampler
    // handles out-of-range input inconsistently.
    expect(resolveSampleRate("5")).toBe(1);
    expect(resolveSampleRate("-3")).toBe(0);
    expect(resolveSampleRate("1e9")).toBe(1);
  });
});

describe("initTracing when unconfigured", () => {
  test("registers nothing and reports disabled", () => {
    const handle = initTracing({
      endpoint: undefined,
      serviceName: "docz-site",
      sampleRate: 1,
    });
    expect(handle.enabled).toBe(false);
  });

  test("still hands back a usable tracer, so callers never branch", () => {
    const handle = initTracing({
      endpoint: undefined,
      serviceName: "docz-site",
      sampleRate: 1,
    });
    const span = handle.tracer.startSpan("test");
    span.setAttribute("http.route", "spa");
    span.end();
    // A non-recording span: created, costs nothing, goes nowhere.
    expect(span.isRecording()).toBe(false);
  });

  test("shutdown resolves without a provider", async () => {
    const handle = initTracing({
      endpoint: undefined,
      serviceName: "docz-site",
      sampleRate: 1,
    });
    await handle.shutdown();
    expect(handle.enabled).toBe(false);
  });
});

describe("no auto-instrumentation is installed", () => {
  test("package.json declares no instrumentation package", async () => {
    // This is the mechanism that would ship OAuth codes to a collector:
    // OTel's HTTP instrumentation records url.full by default, and on
    // this proxy the full URL of an /auth/* hop carries the code
    // (INV-0006 F7). Hand-instrumentation is the mitigation, so the
    // absence of these packages is a security property worth pinning.
    const pkg = (await Bun.file(
      new URL("../package.json", import.meta.url).pathname,
    ).json()) as {
      dependencies?: Record<string, string>;
      devDependencies?: Record<string, string>;
    };
    const names = [
      ...Object.keys(pkg.dependencies ?? {}),
      ...Object.keys(pkg.devDependencies ?? {}),
    ];
    expect(names.filter((n) => n.includes("instrumentation"))).toEqual([]);
    expect(names.filter((n) => n.includes("auto-instrumentations"))).toEqual(
      [],
    );
  });
});
