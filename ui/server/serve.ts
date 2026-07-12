/*
 * Production static server (IMPL-0001 Phase 4 deploy artifact): a small
 * Bun.serve that ships in the Docker image and does exactly four things:
 *
 *   1. serves dist/ with sane cache headers (hashed /assets/ immutable,
 *      index.html always revalidated), preferring precompressed .gz
 *      variants when the client accepts them;
 *   2. falls back to index.html for SPA routes;
 *   3. answers /healthz for probes;
 *   4. proxies /api, /auth, /webhooks and /openapi.yaml to docz-api
 *      (DOCZ_API_URL) so browser and API share one origin — no CORS,
 *      and the httpOnly session cookie just works.
 *
 * Runs under Bun only (Bun.serve/Bun.file) — deliberately not part of
 * the Vite/browser graph. See tsconfig.server.json.
 */

const PORT = Number(process.env.PORT ?? "8080");
const DIST = process.env.DOCZ_SITE_DIST ?? "dist";
const DOCZ_API_URL = process.env.DOCZ_API_URL;

const PROXY_PREFIXES = ["/api/", "/auth/", "/webhooks/"];
const PROXY_EXACT = new Set(["/api", "/auth", "/webhooks", "/openapi.yaml"]);

// Text-ish assets get a build-time .gz sibling (see Dockerfile); fonts
// and images are already compressed.
const IMMUTABLE_PREFIX = "/assets/";

function isProxied(pathname: string): boolean {
  return (
    PROXY_EXACT.has(pathname) ||
    PROXY_PREFIXES.some((prefix) => pathname.startsWith(prefix))
  );
}

async function proxy(req: Request, url: URL): Promise<Response> {
  if (DOCZ_API_URL === undefined) {
    return new Response("DOCZ_API_URL is not configured", { status: 502 });
  }
  const target = new URL(url.pathname + url.search, DOCZ_API_URL);
  const headers = new Headers(req.headers);
  headers.set("host", target.host);
  headers.set("x-forwarded-host", url.host);
  headers.set("x-forwarded-proto", url.protocol.replace(":", ""));
  try {
    const upstream = await fetch(target, {
      method: req.method,
      headers,
      body: req.body,
      // OAuth flows redirect the BROWSER (302 to GitHub) — pass
      // redirects through instead of following them server-side.
      redirect: "manual",
    });
    return new Response(upstream.body, {
      status: upstream.status,
      headers: upstream.headers,
    });
  } catch {
    return new Response("upstream unreachable", { status: 502 });
  }
}

/** Reject traversal and null bytes; return a dist-relative file path. */
function safeDistPath(pathname: string): string | null {
  if (pathname.includes("\0") || pathname.split("/").includes("..")) {
    return null;
  }
  return `${DIST}${pathname}`;
}

function cacheControl(pathname: string): string {
  if (pathname.startsWith(IMMUTABLE_PREFIX)) {
    // Vite content-hashes everything under assets/.
    return "public, max-age=31536000, immutable";
  }
  return "public, max-age=3600";
}

async function serveFile(
  req: Request,
  filePath: string,
  cache: string,
): Promise<Response | null> {
  const file = Bun.file(filePath);
  if (!(await file.exists())) {
    return null;
  }
  const headers = new Headers({
    "cache-control": cache,
    "content-type": file.type,
    vary: "accept-encoding",
  });
  const acceptsGzip = (req.headers.get("accept-encoding") ?? "").includes(
    "gzip",
  );
  if (acceptsGzip) {
    const gz = Bun.file(`${filePath}.gz`);
    if (await gz.exists()) {
      headers.set("content-encoding", "gzip");
      return new Response(gz, { headers });
    }
  }
  return new Response(file, { headers });
}

async function serveIndex(req: Request): Promise<Response> {
  const index = await serveFile(
    req,
    `${DIST}/index.html`,
    // The HTML references hashed assets — always revalidate it.
    "no-cache",
  );
  return index ?? new Response("dist/index.html missing", { status: 500 });
}

const server = Bun.serve({
  port: PORT,
  async fetch(req) {
    const url = new URL(req.url);
    const { pathname } = url;

    if (pathname === "/healthz") {
      return new Response("ok", {
        headers: { "cache-control": "no-store" },
      });
    }
    if (isProxied(pathname)) {
      return proxy(req, url);
    }
    if (req.method !== "GET" && req.method !== "HEAD") {
      return new Response("method not allowed", { status: 405 });
    }

    if (pathname !== "/") {
      const filePath = safeDistPath(pathname);
      if (filePath === null) {
        return new Response("bad request", { status: 400 });
      }
      const file = await serveFile(req, filePath, cacheControl(pathname));
      if (file !== null) {
        return file;
      }
    }
    // SPA fallback: the router owns every other path.
    return serveIndex(req);
  },
});

console.log(
  `docz-site serving ${DIST}/ on :${String(server.port)}` +
    (DOCZ_API_URL === undefined
      ? " (no DOCZ_API_URL — API proxy disabled)"
      : ` (proxying to ${DOCZ_API_URL})`),
);
