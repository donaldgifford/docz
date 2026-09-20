/*
 * Split probes (DESIGN-0006 Component 4).
 *
 * Drives the real request path via the exported `handleRequest` rather
 * than testing the helpers in isolation (OQ-1a) — `import.meta.main`
 * keeps the import from binding a port, so the assertions are about
 * what a kubelet would actually receive.
 */
import { afterAll, describe, expect, test } from "bun:test";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { checkReady, handleRequest, invalidConfigVars } from "./serve";

const temps: string[] = [];

/** A directory that looks like a built dist/, or an empty one. */
async function distDir(withIndex: boolean): Promise<string> {
  const dir = await mkdtemp(join(tmpdir(), "docz-readyz-"));
  temps.push(dir);
  if (withIndex) {
    await writeFile(join(dir, "index.html"), "<!doctype html><html></html>");
  }
  return dir;
}

afterAll(async () => {
  await Promise.all(
    temps.map((dir) => rm(dir, { recursive: true, force: true })),
  );
});

describe("/healthz", () => {
  test("is unconditional 200 ok — liveness, not readiness", async () => {
    const res = await handleRequest(new Request("http://localhost/healthz"));
    expect(res.status).toBe(200);
    expect(await res.text()).toBe("ok");
    expect(res.headers.get("cache-control")).toBe("no-store");
  });

  test("answers even when the dist check would fail", async () => {
    // The distinction that justifies a second endpoint: restarting the
    // container cannot conjure a dist/, so liveness must not fail on it.
    const report = await checkReady(await distDir(false), {});
    expect(report.ready).toBe(false);

    const res = await handleRequest(new Request("http://localhost/healthz"));
    expect(res.status).toBe(200);
  });
});

describe("checkReady", () => {
  test("ready when index.html is present and config is clean", async () => {
    const report = await checkReady(await distDir(true), {});
    expect(report).toEqual({
      ready: true,
      checks: { dist: "ok", config: "ok" },
      invalid: [],
    });
  });

  test("names dist as the offender when index.html is absent", async () => {
    const report = await checkReady(await distDir(false), {});
    expect(report.ready).toBe(false);
    expect(report.checks.dist).toBe("missing");
    expect(report.checks.config).toBe("ok");
  });

  test("a nonexistent dist directory is missing, not a throw", async () => {
    const report = await checkReady("/nonexistent/docz/dist", {});
    expect(report.checks.dist).toBe("missing");
  });

  test("makes no network call — no fetch is issued", async () => {
    // Gating readiness on docz-api would evict every pod from the
    // Service when the API blipped (INV-0006 F3). Proven, not assumed.
    const realFetch = globalThis.fetch;
    let calls = 0;
    globalThis.fetch = ((...args: Parameters<typeof fetch>) => {
      calls += 1;
      return realFetch(...args);
    }) as typeof fetch;
    try {
      await checkReady(await distDir(true), {
        DOCZ_API_URL: "http://docz-api:8080",
      });
      await handleRequest(new Request("http://localhost/readyz"));
    } finally {
      globalThis.fetch = realFetch;
    }
    expect(calls).toBe(0);
  });
});

describe("invalidConfigVars", () => {
  test("an unset var is a default, not a fault", () => {
    expect(invalidConfigVars({})).toEqual([]);
    expect(
      invalidConfigVars({ DOCZ_LOG_LEVEL: "", DOCZ_NAV_LINKS: "  " }),
    ).toEqual([]);
  });

  test("valid values in any casing are clean", () => {
    expect(
      invalidConfigVars({
        DOCZ_LOG_LEVEL: " DEBUG ",
        DOCZ_LOG_FORMAT: "text",
        DOCZ_MERMAID_LAYOUT: "Dagre",
        DOCZ_AUTH_PROVIDERS: "github,okta",
        DOCZ_NAV_LINKS:
          '[{"label":"RFCs","href":"/donaldgifford/docz-api/rfc"}]',
      }),
    ).toEqual([]);
  });

  test("a set-but-unusable closed-set value is named", () => {
    expect(invalidConfigVars({ DOCZ_LOG_LEVEL: "trace" })).toEqual([
      "DOCZ_LOG_LEVEL",
    ]);
    expect(invalidConfigVars({ DOCZ_LOG_FORMAT: "logfmt" })).toEqual([
      "DOCZ_LOG_FORMAT",
    ]);
    expect(invalidConfigVars({ DOCZ_MERMAID_LAYOUT: "cytoscape" })).toEqual([
      "DOCZ_MERMAID_LAYOUT",
    ]);
  });

  test("auth providers: a typo is named, a partial drop is honoured", () => {
    expect(invalidConfigVars({ DOCZ_AUTH_PROVIDERS: "okat" })).toEqual([
      "DOCZ_AUTH_PROVIDERS",
    ]);
    // "github" survived, so the operator got something they asked for.
    expect(invalidConfigVars({ DOCZ_AUTH_PROVIDERS: "github,okat" })).toEqual(
      [],
    );
  });

  test("nav links: only an empty-handed payload counts", () => {
    expect(invalidConfigVars({ DOCZ_NAV_LINKS: "not json" })).toEqual([
      "DOCZ_NAV_LINKS",
    ]);
    expect(invalidConfigVars({ DOCZ_NAV_LINKS: "[]" })).toEqual([
      "DOCZ_NAV_LINKS",
    ]);
    // One of two entries validates — cosmetic, and a rollout must not
    // stall for it.
    expect(
      invalidConfigVars({
        DOCZ_NAV_LINKS:
          '[{"label":"ok","href":"/repos"},{"label":"!!","href":"x"}]',
      }),
    ).toEqual([]);
  });

  test("reports every offender, not just the first", () => {
    expect(
      invalidConfigVars({ DOCZ_LOG_LEVEL: "trace", DOCZ_MERMAID_LAYOUT: "d3" }),
    ).toEqual(["DOCZ_MERMAID_LAYOUT", "DOCZ_LOG_LEVEL"]);
  });
});

describe("/readyz response", () => {
  test("reports the per-check shape DESIGN-0006 specifies", async () => {
    const res = await handleRequest(new Request("http://localhost/readyz"));
    const body = (await res.json()) as {
      status: string;
      checks: Record<string, string>;
    };
    expect(Object.keys(body.checks).sort()).toEqual(["config", "dist"]);
    expect([200, 503]).toContain(res.status);
    expect(res.headers.get("cache-control")).toBe("no-store");
    // The response names WHICH check failed, never which env var — the
    // body is reachable by anything that can reach the pod.
    expect(JSON.stringify(body)).not.toContain("DOCZ_");
  });

  test("status and HTTP code agree", async () => {
    const res = await handleRequest(new Request("http://localhost/readyz"));
    const body = (await res.json()) as { status: string };
    expect(body.status).toBe(res.status === 200 ? "ready" : "not ready");
  });
});

describe("reserved paths", () => {
  test("probe paths never fall through to the SPA", async () => {
    // A probe path that reached the SPA fallback would answer 200
    // text/html — silently poisoning whatever scraped it.
    for (const path of ["/healthz", "/readyz", "/metrics"]) {
      const res = await handleRequest(new Request(`http://localhost${path}`));
      expect(res.headers.get("content-type") ?? "").not.toContain("text/html");
    }
  });

  test("/metrics is 404 until its phase lands, not an SPA page", async () => {
    const res = await handleRequest(new Request("http://localhost/metrics"));
    expect(res.status).toBe(404);
  });
});
