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

// Runtime login-provider config. The set of login buttons the SPA
// renders is chosen per-DEPLOYMENT (DOCZ_AUTH_PROVIDERS), not baked at
// build time — so one signed image serves any provider combo. We inject
// it into index.html as window.__DOCZ_CONFIG__; src/lib/authProviders.ts
// reads that at runtime (falling back to the build-time
// VITE_AUTH_PROVIDERS, then GitHub).
//
// SECURITY: the only values that ever reach the injected <script> come
// from this closed whitelist — never raw env text — so there is no HTML/
// JS injection surface. docz-api's /auth/login rejects anything else.
const KNOWN_AUTH_PROVIDERS = new Set(["github", "okta", "keycloak"]);

/** Whitelist-validate DOCZ_AUTH_PROVIDERS; empty/garbage → ["github"]. */
export function resolveAuthProviders(raw: string | undefined): string[] {
  const keys = [
    ...new Set(
      (raw ?? "")
        .split(",")
        .map((key) => key.trim().toLowerCase())
        .filter((key) => KNOWN_AUTH_PROVIDERS.has(key)),
    ),
  ];
  return keys.length > 0 ? keys : ["github"];
}

/** The inline <script> that publishes the runtime config to the SPA. */
export function runtimeConfigScript(providers: string[]): string {
  const json = JSON.stringify({ authProviders: providers });
  return `<script>window.__DOCZ_CONFIG__=${json};</script>`;
}

/**
 * Insert the config script before the app's entry <script> (falling back
 * to </head>) so window.__DOCZ_CONFIG__ is set first — textually ahead of
 * the bundle, not merely relying on module-defer semantics.
 */
export function injectRuntimeConfig(html: string, script: string): string {
  const marker = html.includes("<script")
    ? "<script"
    : html.includes("</head>")
      ? "</head>"
      : null;
  if (marker === null) {
    // No <head>/<script> (unexpected for a Vite build) — prepend.
    return `${script}${html}`;
  }
  return html.replace(marker, `${script}\n    ${marker}`);
}

const AUTH_PROVIDERS = resolveAuthProviders(process.env.DOCZ_AUTH_PROVIDERS);
const CONFIG_SCRIPT = runtimeConfigScript(AUTH_PROVIDERS);

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

async function serveIndex(): Promise<Response> {
  const file = Bun.file(`${DIST}/index.html`);
  if (!(await file.exists())) {
    return new Response("dist/index.html missing", { status: 500 });
  }
  // Inject runtime config, so index.html is served fresh (not the static
  // .gz sibling) and always revalidated — it references hashed assets.
  const html = injectRuntimeConfig(await file.text(), CONFIG_SCRIPT);
  return new Response(html, {
    headers: {
      "cache-control": "no-cache",
      "content-type": "text/html; charset=utf-8",
    },
  });
}

function startServer() {
  return Bun.serve({
    port: PORT,
    fetch: handleRequest,
  });
}

async function handleRequest(req: Request): Promise<Response> {
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
  return serveIndex();
}

// Guard startup so tests can import the pure helpers above without
// binding a port (import.meta.main is true only for the entrypoint).
if (import.meta.main) {
  const server = startServer();
  console.log(
    `docz-site serving ${DIST}/ on :${String(server.port)} ` +
      `(auth: ${AUTH_PROVIDERS.join(",")})` +
      (DOCZ_API_URL === undefined
        ? " (no DOCZ_API_URL — API proxy disabled)"
        : ` (proxying to ${DOCZ_API_URL})`),
  );
}
