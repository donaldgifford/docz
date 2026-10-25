import { describe, expect, test } from "bun:test";

import type { ReviewSession } from "@/review/api";
import { INSPECT } from "@/review/testing";

import { Lease } from "./lease";
import { Review } from "./round";
import {
  type AssetSource,
  MAX_BODY_BYTES,
  sameSecret,
  startServer,
} from "./server";
import { docAs, reviewFixture } from "./testdata/review-fixture";

const A = docAs(INSPECT.id, INSPECT.path);
const B = docAs("DESIGN-0043", "docs/design/0043-b.md");

const assets: AssetSource = (path) =>
  Promise.resolve(
    path === "/review.html"
      ? new Response("<html>review</html>", {
          headers: { "Content-Type": "text/html" },
        })
      : null,
  );

async function serve(
  over: { graceMs?: number; deadlineMs?: number; heartbeatMs?: number } = {},
  docs = [A],
) {
  const fx = reviewFixture(docs);
  const deadline = new Date(Date.now() + (over.deadlineMs ?? 60_000));
  const { review } = await Review.open(fx.entries, {
    session: fx.session,
    docz: fx.docz,
    reviewId: "sess-1",
    deadline,
    maxRounds: 5,
  });
  const lease = Lease.acquire(fx.session, { ttlMs: 60_000 });
  const srv = startServer({
    review,
    assets,
    lease,
    graceMs: over.graceMs ?? 60_000,
    deadline,
    heartbeatMs: over.heartbeatMs,
  });
  const origin = `http://127.0.0.1:${String(srv.port)}`;
  const api = (path: string, init: RequestInit = {}) =>
    fetch(`${origin}${path}`, {
      ...init,
      headers: {
        Authorization: `Bearer ${srv.token}`,
        ...(init.body === undefined
          ? {}
          : {
              Origin: origin,
              "Content-Type": "application/json",
              "Sec-Fetch-Site": "same-origin",
            }),
        ...(init.headers as Record<string, string> | undefined),
      },
    });
  const session = async () =>
    (await (await api("/api/review")).json()) as ReviewSession;
  const keyOf = async (i: number, submission_id: string) => {
    const d = (await session()).documents[i];
    return {
      round: d?.round,
      content_sha256: d?.inspect.content_sha256,
      submission_id,
    };
  };
  return { ...fx, srv, origin, api, keyOf, review };
}

const post = (body: unknown) => ({
  method: "POST",
  body: JSON.stringify(body),
});

describe("the server", () => {
  test("serves the page with the security headers and no CORS", async () => {
    const { srv, origin } = await serve();
    const res = await fetch(`${origin}/?t=${srv.token}`);
    expect(await res.text()).toBe("<html>review</html>");
    expect(res.headers.get("content-security-policy")).toContain(
      "frame-ancestors 'none'",
    );
    expect(res.headers.get("referrer-policy")).toBe("no-referrer");
    expect(res.headers.get("cache-control")).toBe("no-store");
    expect(res.headers.get("access-control-allow-origin")).toBeNull();
    expect(srv.url).toBe(`${origin}/?t=${srv.token}`);
    srv.end("signal");
  });

  test("a localhost Host is 421", async () => {
    const { srv, origin } = await serve();
    const res = await fetch(`${origin}/api/review`, {
      headers: {
        Host: `localhost:${String(srv.port)}`,
        Authorization: `Bearer ${srv.token}`,
      },
    });
    expect(res.status).toBe(421);
    srv.end("signal");
  });

  test("a missing or wrong bearer is 401", async () => {
    const { srv, origin, api } = await serve();
    expect((await fetch(`${origin}/api/review`)).status).toBe(401);
    expect(
      (await api("/api/review", { headers: { Authorization: "Bearer nope" } }))
        .status,
    ).toBe(401);
    expect((await api("/api/review")).status).toBe(200);
    srv.end("signal");
  });

  test("a mutation needs the exact Origin, same-origin fetch, and JSON", async () => {
    const { srv, api, keyOf } = await serve();
    const body = post(await keyOf(0, "s_1"));
    const path = `/api/review/${A.inspect.id}/approve`;
    expect(
      (await api(path, { ...body, headers: { Origin: "http://evil.example" } }))
        .status,
    ).toBe(403);
    expect(
      (await api(path, { ...body, headers: { Origin: "http://localhost:1" } }))
        .status,
    ).toBe(403);
    expect(
      (
        await api(path, {
          ...body,
          headers: { "Sec-Fetch-Site": "cross-site" },
        })
      ).status,
    ).toBe(403);
    expect(
      (await api(path, { ...body, headers: { "Content-Type": "text/plain" } }))
        .status,
    ).toBe(415);
    expect((await api(path, body)).status).toBe(200);
    srv.end("signal");
  });

  test("an oversized body is refused before it is read", async () => {
    const { srv, api, keyOf } = await serve();
    const big = { ...(await keyOf(0, "s_1")), pad: "x".repeat(MAX_BODY_BYTES) };
    const res = await api(`/api/review/${A.inspect.id}/approve`, post(big));
    expect(res.status).toBe(413);
    srv.end("signal");
  });

  test("a replayed submission id is acknowledged, not recorded twice", async () => {
    const { srv, api, keyOf, review } = await serve({}, [A, B]);
    const key = await keyOf(0, "s_1");
    const path = `/api/review/${A.inspect.id}/approve`;
    expect((await api(path, post(key))).status).toBe(200);
    expect((await api(path, post(key))).status).toBe(200);
    expect(review.records()).toHaveLength(1);
    srv.end("signal");
  });

  test("the run ends settled once every document has an outcome", async () => {
    const { srv, api, keyOf, session } = await serve({}, [A, B]);
    await api(
      `/api/review/${A.inspect.id}/approve`,
      post(await keyOf(0, "s_a")),
    );
    await api(
      `/api/review/${B.inspect.id}/dismiss`,
      post(await keyOf(1, "s_b")),
    );
    expect(await srv.done).toBe("settled");
    expect(session.read().entries.map((e) => e.state)).toEqual([
      "approved",
      "dismissed",
    ]);
  });

  test("Done dismisses the rest", async () => {
    const { srv, api, keyOf, session } = await serve({}, [A, B]);
    await api(
      `/api/review/${A.inspect.id}/approve`,
      post(await keyOf(0, "s_a")),
    );
    expect((await api("/api/review/done", post({}))).status).toBe(200);
    expect(await srv.done).toBe("done");
    expect(session.read().entries.map((e) => e.state)).toEqual([
      "approved",
      "dismissed",
    ]);
  });

  test("a draft is autosaved through PUT", async () => {
    const { srv, api, keyOf, session, entries } = await serve();
    const { round } = await keyOf(0, "x");
    const draft = {
      questions: {},
      annotations: [],
      status: "Draft",
      findings: [],
    };
    const res = await api(`/api/review/${A.inspect.id}/draft`, {
      method: "PUT",
      body: JSON.stringify({ round, draft }),
    });
    expect(res.status).toBe(200);
    expect(session.readDraft(entries[0]?.key ?? "", 1)).toEqual(draft);
    srv.end("signal");
  });

  test("a page that leaves and does not come back ends the run after the grace", async () => {
    const { srv, api, session } = await serve({ graceMs: 50 });
    const ctl = new AbortController();
    const res = await api("/api/events", { signal: ctl.signal });
    const reader = res.body?.getReader();
    await reader?.read();
    ctl.abort();
    expect(await srv.done).toBe("grace");
    expect(session.read().entries[0]?.state).toBe("dismissed");
  });

  test("the deadline ends the run as a dismissal", async () => {
    const { srv, session } = await serve({ deadlineMs: 50 });
    expect(await srv.done).toBe("deadline");
    expect(session.read().entries[0]?.state).toBe("dismissed");
    expect(Lease.read(session)).toBeNull();
  });

  test("losing the lease ends the run", async () => {
    const { srv, session } = await serve({ heartbeatMs: 20 });
    // Another server took the session over.
    await Bun.write(
      `${session.dir}/lease.json`,
      JSON.stringify({
        pid: 1,
        owner: "other",
        expires_at: "2999-01-01T00:00:00Z",
      }),
    );
    expect(await srv.done).toBe("lease");
  });

  test("the event stream says why the run closed", async () => {
    const { srv, api } = await serve();
    const res = await api("/api/events");
    const text = res.text();
    srv.end("signal");
    expect(await text).toContain("event: closed\ndata: signal");
  });
});

describe("sameSecret", () => {
  test("compares whole values of any length", () => {
    expect(sameSecret("abc", "abc")).toBe(true);
    expect(sameSecret("abc", "abd")).toBe(false);
    expect(sameSecret("abc", "abcd")).toBe(false);
  });
});
