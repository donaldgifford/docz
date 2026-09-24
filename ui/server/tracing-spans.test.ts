/*
 * Span shape and the attribute-redaction gate (DESIGN-0006 Component 6).
 *
 * This is the tracing counterpart to Phase 2's log redaction gate, and
 * it matters for the same reason: a span attribute reaches a collector,
 * which is no more allowed to hold an OAuth code than a log line is.
 *
 * It runs in a CHILD PROCESS because the tracer provider is global and
 * is registered once at serve.ts import — and `bun test` shares one
 * module registry across the whole run, so a sibling suite has already
 * imported serve.ts with tracing off. The child swaps the exporter for
 * an in-memory one before the first import, then drives the real
 * handleRequest and prints the finished spans.
 */
import { describe, expect, test } from "bun:test";

const SERVE = new URL("./serve.ts", import.meta.url).pathname;

/** Distinguishes the span payload from the server's own log lines. */
const MARKER = "__SPANS__";

const CODE = "4/0AY0e-g7pK9xLmQ2vR8sT1uV3wX5yZ";
const STATE = "n0nce-9f8e7d6c5b4a3210";

interface CapturedSpan {
  name: string;
  attributes: Record<string, unknown>;
  status: number;
  parentSpanId: string | undefined;
  spanId: string;
  traceId: string;
}

/** A path, or a path with a method other than GET. */
type Hit = string | { path: string; method: string };

/**
 * Drive requests through the real pipeline with an in-memory exporter
 * registered, and return every span that finished.
 *
 * Nothing here may assume a built `dist/`: `just ci` runs `test-server`
 * BEFORE `build`, so on a clean checkout every SPA path answers 500.
 * Assert on responses the router produces on its own.
 */
async function spansFor(paths: Hit[]): Promise<CapturedSpan[]> {
  const script = `
    const { NodeTracerProvider, InMemorySpanExporter, SimpleSpanProcessor } =
      await import("@opentelemetry/sdk-trace-node");
    const exporter = new InMemorySpanExporter();
    const provider = new NodeTracerProvider({
      spanProcessors: [new SimpleSpanProcessor(exporter)],
    });
    provider.register();

    const { handleRequest } = await import(${JSON.stringify(SERVE)});
    for (const hit of ${JSON.stringify(paths)}) {
      const { path, method } =
        typeof hit === "string" ? { path: hit, method: "GET" } : hit;
      await handleRequest(new Request("http://localhost" + path, { method }));
    }
    await provider.forceFlush();

    // Marked, because the server's own structured logs share this
    // stdout — a proxy.error line would otherwise be parsed as spans.
    console.log(${JSON.stringify(MARKER)} + JSON.stringify(
      exporter.getFinishedSpans().map((s) => ({
        name: s.name,
        attributes: s.attributes,
        status: s.status.code,
        parentSpanId: s.parentSpanContext?.spanId,
        spanId: s.spanContext().spanId,
        traceId: s.spanContext().traceId,
      })),
    ));
  `;
  const proc = Bun.spawn(["bun", "-e", script], {
    // A target that refuses instantly, so the proxy path completes fast.
    env: { ...process.env, DOCZ_API_URL: "http://127.0.0.1:9" },
    stdout: "pipe",
    stderr: "pipe",
  });
  const [out, err] = await Promise.all([
    new Response(proc.stdout).text(),
    new Response(proc.stderr).text(),
  ]);
  await proc.exited;
  const line = out
    .split("\n")
    .find((candidate) => candidate.startsWith(MARKER));
  if (line === undefined) {
    throw new Error(`child produced no spans; stdout: ${out} stderr: ${err}`);
  }
  return JSON.parse(line.slice(MARKER.length)) as CapturedSpan[];
}

describe("server spans", () => {
  test("an SPA request produces one span named by METHOD and route class", async () => {
    const spans = await spansFor(["/donaldgifford/docz-site/design/DESIGN-1"]);
    expect(spans).toHaveLength(1);
    expect(spans[0]?.name).toBe("GET spa");
  });

  test("attributes are the allowlist, and nothing else", async () => {
    const spans = await spansFor(["/donaldgifford/docz-site"]);
    const attrs = spans[0]?.attributes ?? {};

    expect(Object.keys(attrs).sort()).toEqual([
      "http.request.method",
      "http.response.status_code",
      "http.route",
      "url.path",
    ]);
    expect(attrs["http.request.method"]).toBe("GET");
    expect(attrs["http.route"]).toBe("spa");
  });

  test("url.full is NEVER set — the attribute auto-instrumentation adds", async () => {
    const spans = await spansFor(["/donaldgifford/docz-site?token=abc"]);
    for (const span of spans) {
      expect(span.attributes).not.toHaveProperty("url.full");
      expect(span.attributes).not.toHaveProperty("url.query");
      expect(span.attributes).not.toHaveProperty("http.request.header.cookie");
    }
  });

  test("probe paths produce NO span at all", async () => {
    const spans = await spansFor(["/healthz", "/readyz", "/metrics"]);
    expect(spans).toEqual([]);
  });

  test("the span name is method + route class, never the raw path", async () => {
    // This previously sent a plain GET while claiming to exercise a
    // hostile method, and only asserted the path was absent — so it
    // passed without testing anything. A hostile method cannot reach
    // handleRequest through this door at all: Bun's Request constructor
    // silently rewrites an unrecognised method token to GET (verified:
    // "EVIL-METHOD" and "BREW" both arrive as GET). The collapse to
    // `other` is asserted directly in route-class.test.ts; what belongs
    // here is that the span NAME is built from the classifier, so an
    // unbounded path can never become a span name.
    const spans = await spansFor([{ path: "/some/spa/path", method: "PATCH" }]);
    expect(spans[0]?.name).toBe("PATCH spa");
    expect(spans[0]?.name).not.toContain("/some/spa/path");
  });
});

describe("proxy child span", () => {
  test("is a child of the server span in the same trace", async () => {
    const spans = await spansFor(["/api/v1/repos"]);
    const child = spans.find((s) => s.name === "proxy.upstream");
    const parent = spans.find((s) => s.name !== "proxy.upstream");

    expect(child).toBeDefined();
    expect(parent).toBeDefined();
    // Linkage, not just presence: two unparented roots would still be
    // "two spans" while producing no usable trace.
    expect(child?.parentSpanId).toBe(parent?.spanId);
    expect(child?.traceId).toBe(parent?.traceId);
  });

  test("carries the upstream HOST, never the proxied URL", async () => {
    const spans = await spansFor([
      `/auth/callback?code=${CODE}&state=${STATE}`,
    ]);
    const child = spans.find((s) => s.name === "proxy.upstream");

    expect(child?.attributes["server.address"]).toBe("127.0.0.1:9");
    expect(child?.attributes).not.toHaveProperty("url.full");
  });

  test("an unreachable upstream sets span status ERROR", async () => {
    const spans = await spansFor(["/api/v1/repos"]);
    const child = spans.find((s) => s.name === "proxy.upstream");
    // SpanStatusCode.ERROR === 2
    expect(child?.status).toBe(2);
  });

  test("the 502 sets ERROR on the server span too, since 5xx", async () => {
    const spans = await spansFor(["/api/v1/repos"]);
    const parent = spans.find((s) => s.name !== "proxy.upstream");
    expect(parent?.attributes["http.response.status_code"]).toBe(502);
    expect(parent?.status).toBe(2);
  });

  test("a 4xx does NOT set ERROR — client fault, matching docz-api", async () => {
    // A POST to a non-proxied path is rejected by the router itself, so
    // this is a 4xx whether or not `dist/` was built. The previous
    // version used a traversal path expecting 400 but actually got the
    // SPA fallback's 200 — which flipped to a 500 on CI, where there is
    // no dist yet, and only then failed.
    const spans = await spansFor([{ path: "/some/spa/path", method: "POST" }]);
    const parent = spans.find((s) => s.name !== "proxy.upstream");

    expect(parent?.attributes["http.response.status_code"]).toBe(405);
    expect(parent?.status).not.toBe(2);
  });
});

describe("traceparent injection", () => {
  test("a well-formed W3C traceparent reaches the upstream", async () => {
    // The end-to-end claim rests on this header: docz-api installs a
    // TraceContext propagator and Extracts on every request, so if this
    // is well-formed and carries our trace id, its spans join our trace
    // with ZERO upstream change (INV-0006 F5). Verified against a real
    // listening socket rather than by inspecting our own span.
    const script = `
      let captured = null;
      const upstream = Bun.serve({
        port: 8141,
        fetch(req) {
          captured = req.headers.get("traceparent");
          return new Response("ok");
        },
      });

      const { NodeTracerProvider, InMemorySpanExporter, SimpleSpanProcessor } =
        await import("@opentelemetry/sdk-trace-node");
      const exporter = new InMemorySpanExporter();
      const provider = new NodeTracerProvider({
        spanProcessors: [new SimpleSpanProcessor(exporter)],
      });
      provider.register();

      const { handleRequest } = await import(${JSON.stringify(SERVE)});
      await handleRequest(new Request("http://localhost/api/v1/repos"));
      await provider.forceFlush();
      upstream.stop();

      const server = exporter.getFinishedSpans()
        .find((s) => s.name !== "proxy.upstream");
      console.log(${JSON.stringify(MARKER)} + JSON.stringify({
        traceparent: captured,
        serverTraceId: server?.spanContext().traceId,
      }));
    `;
    const proc = Bun.spawn(["bun", "-e", script], {
      env: { ...process.env, DOCZ_API_URL: "http://127.0.0.1:8141" },
      stdout: "pipe",
      stderr: "pipe",
    });
    const [out, err] = await Promise.all([
      new Response(proc.stdout).text(),
      new Response(proc.stderr).text(),
    ]);
    await proc.exited;
    const line = out.split("\n").find((c) => c.startsWith(MARKER));
    if (line === undefined) {
      throw new Error(`no result; stdout: ${out} stderr: ${err}`);
    }
    const result = JSON.parse(line.slice(MARKER.length)) as {
      traceparent: string | null;
      serverTraceId: string | undefined;
    };

    expect(result.traceparent).not.toBeNull();
    // version-traceid-spanid-flags, per the W3C Trace Context spec.
    expect(result.traceparent).toMatch(/^00-[0-9a-f]{32}-[0-9a-f]{16}-0[0-3]$/);
    // Same trace, not merely a valid-looking header.
    expect(result.traceparent?.split("-")[1]).toBe(result.serverTraceId);
  });
});

describe("attribute redaction gate", () => {
  test("no span attribute anywhere carries a code or state VALUE", async () => {
    // The tracing counterpart to the log redaction gate. A span
    // attribute reaches a collector, which is no more allowed to hold
    // an OAuth code than a log line is.
    const spans = await spansFor([
      `/auth/callback?code=${CODE}&state=${STATE}`,
      `/api/v1/search?q=hello&token=${CODE}`,
    ]);
    expect(spans.length).toBeGreaterThan(0);

    const serialized = JSON.stringify(spans);
    expect(serialized).not.toContain(CODE);
    expect(serialized).not.toContain(STATE);
    expect(serialized).not.toContain(encodeURIComponent(CODE));
  });

  test("the redacted url.path keeps its KEYS, so the span stays useful", async () => {
    const spans = await spansFor([`/auth/callback?code=${CODE}`]);
    const parent = spans.find((s) => s.name !== "proxy.upstream");
    const raw = parent?.attributes["url.path"];
    const path = typeof raw === "string" ? raw : "";

    expect(path).toContain("/auth/callback");
    expect(path).toContain("code=");
    expect(path).toContain("<redacted>");
  });

  test("the gate would FAIL if a raw URL were ever set as an attribute", () => {
    // Verify the guard fires before trusting it green: the assertion
    // above is a substring search, so prove that search actually
    // catches the secret when it IS present.
    const leaked = JSON.stringify([
      { attributes: { "url.full": `/auth/callback?code=${CODE}` } },
    ]);
    expect(leaked).toContain(CODE);
  });
});
