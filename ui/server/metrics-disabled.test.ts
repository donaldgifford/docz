/*
 * The DOCZ_METRICS_ENABLED=false path (DESIGN-0006 OQ-4a).
 *
 * DOCZ_METRICS_ENABLED is read once at module import, and `bun test`
 * shares one module registry across the whole run — so by the time this
 * file executes, serve.ts has already been imported by a sibling suite
 * with metrics ON. Setting the env here and re-importing would test
 * nothing.
 *
 * A child process is therefore the only honest way to exercise the
 * disabled build: fresh registry, env set before the first import, and
 * it runs the same serve.ts that ships.
 */
import { describe, expect, test } from "bun:test";

const SERVE = new URL("./serve.ts", import.meta.url).pathname;

/** Distinguishes the payload from the server's own log lines. */
const MARKER = "__RESULT__";

/** Drive one request through a serve.ts imported with metrics off. */
async function requestWithMetricsDisabled(
  path: string,
): Promise<{ status: number; body: string; contentType: string }> {
  const script = `
    const { handleRequest } = await import(${JSON.stringify(SERVE)});
    const res = await handleRequest(new Request("http://localhost${path}"));
    // Marked, because the server's structured logs share this stdout —
    // a readyz.fail warn would otherwise be parsed as the payload.
    console.log(${JSON.stringify(MARKER)} + JSON.stringify({
      status: res.status,
      body: await res.text(),
      contentType: res.headers.get("content-type") ?? "",
    }));
  `;
  const proc = Bun.spawn(["bun", "-e", script], {
    env: { ...process.env, DOCZ_METRICS_ENABLED: "false" },
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
    throw new Error(`child produced no output; stdout: ${out} stderr: ${err}`);
  }
  return JSON.parse(line.slice(MARKER.length)) as {
    status: number;
    body: string;
    contentType: string;
  };
}

describe("metrics disabled", () => {
  test("/metrics returns an explicit 404, NOT the SPA shell", async () => {
    const res = await requestWithMetricsDisabled("/metrics");

    // The explicit 404 is the entire point. An unregistered route would
    // fall through to the SPA fallback and hand a scraper index.html
    // with a 200 — a dashboard full of parse errors instead of an
    // endpoint that honestly reports itself absent.
    expect(res.status).toBe(404);
    expect(res.body.toLowerCase()).not.toContain("<!doctype html");
    expect(res.body.toLowerCase()).not.toContain("<html");
    expect(res.contentType).toContain("text/plain");
  });

  test("no exposition leaks from the disabled endpoint", async () => {
    const res = await requestWithMetricsDisabled("/metrics");
    expect(res.body).not.toContain("docz_site_");
    expect(res.body).not.toContain("process_cpu");
  });

  test("the other probes still answer, so nothing else breaks", async () => {
    expect((await requestWithMetricsDisabled("/healthz")).status).toBe(200);
    expect([200, 503]).toContain(
      (await requestWithMetricsDisabled("/readyz")).status,
    );
  });

  test("the child really is running with metrics OFF", async () => {
    // Verify the harness before trusting its green: the same request in
    // THIS process (metrics on) must answer 200 exposition, so a 404
    // above can only come from the env the child was given.
    const { handleRequest } = await import("./serve");
    const here = await handleRequest(new Request("http://localhost/metrics"));
    expect(here.status).toBe(200);
    expect(await here.text()).toContain("docz_site_");
  });
});
