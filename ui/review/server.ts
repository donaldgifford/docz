/*
 * docz-review's local server (DESIGN-0024 section 6), in the shape of
 * server/serve.ts without the proxy, metrics, or tracing.
 *
 * One canonical origin, http://127.0.0.1:<port>: any other Host is 421.
 * The credential is a random token the page reads from its bootstrap URL
 * and sends as a bearer header; there is no cookie, so nothing ambient
 * reaches this port from another page. Mutations also need the exact
 * Origin, Sec-Fetch-Site same-origin when the browser sends it, and a
 * JSON body. No CORS headers are ever sent, and every response carries
 * the CSP, no-referrer, and no-store.
 *
 * The page and its assets are static and served without the token; what
 * a review contains is only ever behind /api. A submission becomes the
 * agent's next instruction, which is why a forged one is a prompt
 * injection and none of these checks is optional.
 */
import { createHash, randomBytes, timingSafeEqual } from "node:crypto";

import type { Lease } from "./lease";
import type { Action, Reply, Review } from "./round";

/** Why a review run ended. */
export type EndReason =
  "settled" | "done" | "deadline" | "grace" | "lease" | "signal";

/** Serves a static path, or null when there is no such asset. */
export type AssetSource = (path: string) => Promise<Response | null>;

/** One embedded asset: the file Bun embedded and its MIME type. */
export interface Asset {
  file: string;
  type: string;
}

/** Assets from a path-to-file map, as the compiled binary embeds them. */
export function mapAssets(
  assets: Readonly<Record<string, Asset>>,
): AssetSource {
  return (path) => {
    const a = Object.hasOwn(assets, path) ? assets[path] : undefined;
    return Promise.resolve(
      a === undefined
        ? null
        : new Response(Bun.file(a.file), {
            headers: { "Content-Type": a.type },
          }),
    );
  };
}

/** Assets from a built directory (dist-review), for a checkout. */
export function diskAssets(dir: string): AssetSource {
  return async (path) => {
    const rel = path.replace(/^\/+/, "");
    if (rel.split("/").some((seg) => seg === ".." || seg === "")) {
      return null;
    }
    const file = Bun.file(`${dir}/${rel}`);
    return (await file.exists()) ? new Response(file) : null;
  };
}

export interface ServerOptions {
  review: Review;
  assets: AssetSource;
  /** 0 for a random port. */
  port?: number;
  token?: string;
  /** Held while the server lives; losing it ends the run. */
  lease?: Lease;
  /** How long a disconnected page may take to come back. */
  graceMs: number;
  /** When the run ends whatever the page does. */
  deadline: Date;
  /** Lease renewal and event-stream ping period. */
  heartbeatMs?: number;
}

export interface RunningServer {
  url: string;
  port: number;
  token: string;
  /** Resolves once, with why the run ended. */
  done: Promise<EndReason>;
  /** Ends the run (a signal, say) and stops the server. */
  end(reason: EndReason): void;
}

export const SECURITY_HEADERS: Record<string, string> = {
  "Content-Security-Policy":
    "default-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'",
  "Referrer-Policy": "no-referrer",
  "Cache-Control": "no-store",
  "X-Content-Type-Options": "nosniff",
};

const ROUTE = /^\/api\/review\/([^/]+)\/(submit|approve|dismiss|draft)$/;

function withHeaders(res: Response): Response {
  for (const [k, v] of Object.entries(SECURITY_HEADERS)) {
    res.headers.set(k, v);
  }
  return res;
}

const json = (status: number, body: unknown) => Response.json(body, { status });
const error = (status: number, message: string) =>
  json(status, { error: message });

/** Compares two secrets in time independent of where they differ. */
export function sameSecret(a: string, b: string): boolean {
  const h = (s: string) => createHash("sha256").update(s).digest();
  return timingSafeEqual(h(a), h(b));
}

/** Reads a stream to EOF; the hook's stdin is read whole, then ignored. */
export async function readAll(
  stream: ReadableStream<Uint8Array>,
): Promise<string> {
  return new Response(stream).text();
}

export function startServer(opts: ServerOptions): RunningServer {
  const token = opts.token ?? randomBytes(32).toString("hex");
  const heartbeat = opts.heartbeatMs ?? 15_000;
  const streams = new Set<ReadableStreamDefaultController<string>>();
  const timers: ReturnType<typeof setTimeout>[] = [];
  let graceTimer: ReturnType<typeof setTimeout> | undefined;
  let ended = false;
  let resolveDone: (r: EndReason) => void = () => undefined;
  const done = new Promise<EndReason>((r) => {
    resolveDone = r;
  });

  const server = Bun.serve({
    hostname: "127.0.0.1",
    port: opts.port ?? 0,
    idleTimeout: 0,
    fetch: async (req) => withHeaders(await handle(req)),
  });
  const port = server.port ?? 0;
  const origin = `http://127.0.0.1:${String(port)}`;

  function end(reason: EndReason): void {
    if (ended) {
      return;
    }
    ended = true;
    if (reason !== "settled") {
      opts.review.dismissRest();
    }
    for (const t of timers) {
      clearTimeout(t);
      clearInterval(t);
    }
    if (graceTimer !== undefined) {
      clearTimeout(graceTimer);
    }
    for (const c of streams) {
      try {
        c.enqueue(`event: closed\ndata: ${reason}\n\n`);
        c.close();
      } catch {
        // the page went away first
      }
    }
    streams.clear();
    opts.lease?.release();
    resolveDone(reason);
    // Let the last reply and the closed event flush before closing.
    setTimeout(() => void server.stop(true), 50);
  }

  timers.push(
    setTimeout(
      () => {
        end("deadline");
      },
      Math.max(0, opts.deadline.getTime() - Date.now()),
    ),
  );
  timers.push(
    setInterval(() => {
      if (opts.lease !== undefined && !opts.lease.renew(heartbeat * 6)) {
        end("lease");
        return;
      }
      for (const c of streams) {
        c.enqueue(": ping\n\n");
      }
    }, heartbeat),
  );

  function events(): Response {
    let self: ReadableStreamDefaultController<string> | undefined;
    const stream = new ReadableStream<string>({
      start(c) {
        self = c;
        streams.add(c);
        if (graceTimer !== undefined) {
          clearTimeout(graceTimer);
          graceTimer = undefined;
        }
        c.enqueue(": connected\n\n");
      },
      cancel() {
        if (self !== undefined) {
          streams.delete(self);
        }
        if (streams.size === 0 && !ended) {
          graceTimer = setTimeout(() => {
            end("grace");
          }, opts.graceMs);
        }
      },
    });
    return new Response(stream.pipeThrough(new TextEncoderStream()), {
      headers: { "Content-Type": "text/event-stream" },
    });
  }

  async function handle(req: Request): Promise<Response> {
    if (req.headers.get("host") !== `127.0.0.1:${String(port)}`) {
      return error(421, "use the 127.0.0.1 address docz-review printed");
    }
    const { pathname } = new URL(req.url);
    if (!pathname.startsWith("/api/")) {
      if (req.method !== "GET" && req.method !== "HEAD") {
        return error(405, "method not allowed");
      }
      const asset = await opts.assets(
        pathname === "/" ? "/review.html" : pathname,
      );
      return asset ?? error(404, "not found");
    }

    const auth = req.headers.get("authorization") ?? "";
    if (!auth.startsWith("Bearer ") || !sameSecret(auth.slice(7), token)) {
      return error(401, "unauthorized");
    }
    if (req.method === "GET" && pathname === "/api/review") {
      return json(200, opts.review.view());
    }
    if (req.method === "GET" && pathname === "/api/events") {
      return events();
    }
    if (req.method !== "POST" && req.method !== "PUT") {
      return error(405, "method not allowed");
    }

    if (req.headers.get("origin") !== origin) {
      return error(403, "cross-origin request refused");
    }
    const site = req.headers.get("sec-fetch-site");
    if (site !== null && site !== "same-origin") {
      return error(403, "cross-site request refused");
    }
    if (
      !(req.headers.get("content-type") ?? "").startsWith("application/json")
    ) {
      return error(415, "the body must be application/json");
    }
    let body: unknown;
    try {
      body = await req.json();
    } catch {
      return error(400, "the body is not JSON");
    }
    if (ended) {
      return error(409, "the review has ended");
    }

    if (req.method === "POST" && pathname === "/api/review/done") {
      end("done");
      return json(200, {});
    }
    const m = ROUTE.exec(pathname);
    if (m === null) {
      return error(404, "not found");
    }
    const key = decodeURIComponent(m[1] ?? "");
    const action = m[2] ?? "";
    let reply: Reply;
    if (action === "draft") {
      if (req.method !== "PUT") {
        return error(405, "method not allowed");
      }
      reply = opts.review.saveDraft(key, body);
    } else {
      if (req.method !== "POST") {
        return error(405, "method not allowed");
      }
      reply = await opts.review.record(key, action as Action, body);
      if (reply.status === 200 && opts.review.settled()) {
        end("settled");
      }
    }
    return json(reply.status, reply.body);
  }

  return {
    url: `${origin}/?t=${token}`,
    port,
    token,
    done,
    end,
  };
}
