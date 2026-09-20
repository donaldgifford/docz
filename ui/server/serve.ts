/*
 * Production static server (IMPL-0001 Phase 4 deploy artifact): a small
 * Bun.serve that ships in the Docker image and does exactly four things:
 *
 *   1. serves dist/ with sane cache headers (hashed /assets/ immutable,
 *      index.html always revalidated), preferring precompressed .gz
 *      variants when the client accepts them;
 *   2. falls back to index.html for SPA routes;
 *   3. answers the operational probes — /healthz (liveness,
 *      unconditional) and /readyz (readiness, checks dist/ and config);
 *   4. proxies /api, /auth, /webhooks and /openapi.yaml to docz-api
 *      (DOCZ_API_URL) so browser and API share one origin — no CORS,
 *      and the httpOnly session cookie just works.
 *
 * Runs under Bun only (Bun.serve/Bun.file) — deliberately not part of
 * the Vite/browser graph. See tsconfig.server.json.
 */

import {
  createLogger,
  LOG_FORMATS,
  LOG_LEVELS,
  type LogFormat,
  type LogLevel,
} from "./logger";
import { hasCookie, redactLocation, redactUrl } from "./redact";
import {
  classifyRoute,
  IMMUTABLE_PREFIX,
  isProxiedPath,
  normalizeMethod,
  PROBE_PATHS,
  type RouteClass,
} from "./route-class";

const PORT = Number(process.env.PORT ?? "8080");
const DIST = process.env.DOCZ_SITE_DIST ?? "dist";
const DOCZ_API_URL = process.env.DOCZ_API_URL;

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

// Runtime nav pins (DESIGN-0002 Component 1): DOCZ_NAV_LINKS is a JSON
// array of {label, href}, chosen per-deployment like the auth
// providers. Validation here is what stands between env text and the
// inline <script>, so it's a whitelist: tight label charset, app-path
// hrefs in printable ASCII with no HTML-significant characters, hard
// cap. Invalid entries (or an invalid payload) drop silently — bad
// config degrades to fewer pins, never a broken page.
export interface NavLink {
  label: string;
  href: string;
}

const NAV_LINK_CAP = 6;
const NAV_LABEL = /^[\w .&+-]{1,24}$/;
const NAV_HREF_MAX = 200;

/**
 * Same-origin app paths only — the authReturn.ts stash rule (leading
 * "/", never "//") plus: printable ASCII, no whitespace/control, none
 * of <>"'`\ — so a href can never terminate the inline script or
 * smuggle markup.
 */
function isValidNavHref(href: string): boolean {
  if (
    href.length > NAV_HREF_MAX ||
    !href.startsWith("/") ||
    href.startsWith("//")
  ) {
    return false;
  }
  for (const char of href) {
    const code = char.codePointAt(0) ?? 0;
    if (code <= 0x20 || code >= 0x7f || "<>\"'`\\".includes(char)) {
      return false;
    }
  }
  return true;
}

/** Parse + validate DOCZ_NAV_LINKS; anything invalid → fewer/no pins. */
export function resolveNavLinks(raw: string | undefined): NavLink[] {
  if (raw === undefined || raw.trim() === "") {
    return [];
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return [];
  }
  if (!Array.isArray(parsed)) {
    return [];
  }
  const links: NavLink[] = [];
  for (const entry of parsed) {
    if (links.length === NAV_LINK_CAP) {
      break;
    }
    if (typeof entry !== "object" || entry === null) {
      continue;
    }
    const { label, href } = entry as Record<string, unknown>;
    if (typeof label !== "string" || typeof href !== "string") {
      continue;
    }
    if (!NAV_LABEL.test(label) || !isValidNavHref(href)) {
      continue;
    }
    links.push({ label, href });
  }
  return links;
}

// Runtime diagram layout (IMPL-0006 OQ-1): mermaid 12 makes ELK the
// default layout engine and so does this site, but a deployment that
// wants dagre back must be able to say so without rebuilding the image
// — the same bargain as auth providers and nav pins. A closed set, so
// the only strings that can ever reach the inline <script> are the two
// named here.
const KNOWN_MERMAID_LAYOUTS = new Set(["dagre", "elk"]);
const DEFAULT_MERMAID_LAYOUT = "elk";

/** Whitelist-validate DOCZ_MERMAID_LAYOUT; empty/garbage → "elk". */
export function resolveMermaidLayout(raw: string | undefined): string {
  const layout = (raw ?? "").trim().toLowerCase();
  return KNOWN_MERMAID_LAYOUTS.has(layout) ? layout : DEFAULT_MERMAID_LAYOUT;
}

// Logging config (DESIGN-0006). Same whitelist-with-fallback discipline
// as everything above: bad config degrades to the default rather than
// failing startup, because a typo in a log level must never be the
// reason a deployment will not boot.
const DEFAULT_LOG_LEVEL: LogLevel = "info";
const DEFAULT_LOG_FORMAT: LogFormat = "json";

/** Whitelist-validate DOCZ_LOG_LEVEL; empty/garbage → "info". */
export function resolveLogLevel(raw: string | undefined): LogLevel {
  const value = (raw ?? "").trim().toLowerCase();
  return LOG_LEVELS.find((level) => level === value) ?? DEFAULT_LOG_LEVEL;
}

/** Whitelist-validate DOCZ_LOG_FORMAT; empty/garbage → "json". */
export function resolveLogFormat(raw: string | undefined): LogFormat {
  const value = (raw ?? "").trim().toLowerCase();
  return LOG_FORMATS.find((format) => format === value) ?? DEFAULT_LOG_FORMAT;
}

// Readiness (DESIGN-0006 Component 4). Liveness and readiness answer
// different questions and Kubernetes responds to them differently: a
// failing liveness probe RESTARTS the container, a failing readiness
// probe HOLDS TRAFFIC. A missing dist/ is not fixed by restarting —
// that is CrashLoopBackOff — but it should absolutely stall the rollout
// and leave the previous ReplicaSet serving. Same detection, correct
// response. That asymmetry is the whole reason for a second endpoint.

/** The environment readiness inspects, injectable for tests. */
export type ConfigEnv = Readonly<Record<string, string | undefined>>;

/** Per-check status: `ok`, or the reason it is not. */
export interface ReadyChecks {
  dist: "ok" | "missing";
  config: "ok" | "invalid";
}

export interface ReadyReport {
  ready: boolean;
  checks: ReadyChecks;
  /** Names of vars that were set but produced nothing usable. */
  invalid: string[];
}

function isSet(raw: string | undefined): raw is string {
  return raw !== undefined && raw.trim() !== "";
}

/** True when every provider the resolver returned was actually asked for. */
function authProvidersHonoured(raw: string): boolean {
  const asked = new Set(
    raw
      .split(",")
      .map((key) => key.trim().toLowerCase())
      .filter((key) => key !== ""),
  );
  return resolveAuthProviders(raw).every((key) => asked.has(key));
}

/**
 * Config vars that are SET but produce nothing usable — an unambiguous
 * deployment mistake rather than a preference.
 *
 * Deliberately narrow, because readiness failure stalls a rollout and
 * that is too blunt an instrument for a cosmetic typo. An unset var is
 * a default, not a fault; a nav array where some entries validate is
 * honoured for the ones that did. Only a non-empty value that survives
 * validation empty-handed counts.
 *
 * No whitelist is restated here — each check runs the real resolver and
 * looks at whether it fell back, so this cannot drift from the rules it
 * is reporting on.
 */
export function invalidConfigVars(env: ConfigEnv): string[] {
  const invalid: string[] = [];

  // Closed-set vars echo a valid value back, so a mismatch with the
  // normalised input IS the fallback.
  const closedSet: readonly [string, (raw: string) => string][] = [
    ["DOCZ_MERMAID_LAYOUT", resolveMermaidLayout],
    ["DOCZ_LOG_LEVEL", resolveLogLevel],
    ["DOCZ_LOG_FORMAT", resolveLogFormat],
  ];
  for (const [name, resolve] of closedSet) {
    const raw = env[name];
    if (isSet(raw) && resolve(raw) !== raw.trim().toLowerCase()) {
      invalid.push(name);
    }
  }

  const providers = env.DOCZ_AUTH_PROVIDERS;
  if (isSet(providers) && !authProvidersHonoured(providers)) {
    invalid.push("DOCZ_AUTH_PROVIDERS");
  }
  const nav = env.DOCZ_NAV_LINKS;
  if (isSet(nav) && resolveNavLinks(nav).length === 0) {
    invalid.push("DOCZ_NAV_LINKS");
  }
  return invalid;
}

/**
 * Readiness: can this pod serve? Takes `distDir` rather than reading the
 * module-level DIST so tests can vary it — DIST is read once at import
 * and cannot be changed afterwards.
 *
 * This makes NO network call. Gating readiness on docz-api would evict
 * every pod from the Service the moment the API blipped, converting a
 * degradation the SPA already handles into a total outage (INV-0006 F3).
 * Upstream health is a metric, not a probe.
 */
export async function checkReady(
  distDir: string,
  env: ConfigEnv = process.env,
): Promise<ReadyReport> {
  const distOk = await Bun.file(`${distDir}/index.html`).exists();
  const invalid = invalidConfigVars(env);
  return {
    ready: distOk && invalid.length === 0,
    checks: {
      dist: distOk ? "ok" : "missing",
      config: invalid.length === 0 ? "ok" : "invalid",
    },
    invalid,
  };
}

/**
 * Everything the SPA reads off window.__DOCZ_CONFIG__. Every field is
 * the output of a whitelist above, never raw env text.
 */
export interface RuntimeConfig {
  authProviders: string[];
  nav: NavLink[];
  mermaidLayout: string;
}

/** The inline <script> that publishes the runtime config to the SPA. */
export function runtimeConfigScript(config: RuntimeConfig): string {
  // Keys written out rather than stringifying the argument, so the
  // emitted order does not depend on how a caller built the object.
  // Validation already forbids "</" anywhere, but escape it anyway
  // (JSON-legal) so the script can't be terminated even if a rule
  // ever loosens.
  const json = JSON.stringify({
    authProviders: config.authProviders,
    nav: config.nav,
    mermaidLayout: config.mermaidLayout,
  }).replaceAll("</", "<\\/");
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
const NAV_LINKS = resolveNavLinks(process.env.DOCZ_NAV_LINKS);
const MERMAID_LAYOUT = resolveMermaidLayout(process.env.DOCZ_MERMAID_LAYOUT);
const LOG_LEVEL = resolveLogLevel(process.env.DOCZ_LOG_LEVEL);
const LOG_FORMAT = resolveLogFormat(process.env.DOCZ_LOG_FORMAT);

const log = createLogger({ level: LOG_LEVEL, format: LOG_FORMAT });
const CONFIG_SCRIPT = runtimeConfigScript({
  authProviders: AUTH_PROVIDERS,
  nav: NAV_LINKS,
  mermaidLayout: MERMAID_LAYOUT,
});

async function proxy(
  req: Request,
  url: URL,
  route: RouteClass,
): Promise<Response> {
  const method = normalizeMethod(req.method);
  if (DOCZ_API_URL === undefined) {
    // A config fault and a network fault are indistinguishable from
    // outside — both are a bare 502 — so they get distinct reasons here.
    log.error("proxy.error", {
      method,
      route,
      reason: "not_configured",
      err_message: "DOCZ_API_URL is not set",
    });
    return new Response("DOCZ_API_URL is not configured", { status: 502 });
  }
  const target = new URL(url.pathname + url.search, DOCZ_API_URL);
  const headers = new Headers(req.headers);
  headers.set("host", target.host);
  headers.set("x-forwarded-host", url.host);
  headers.set("x-forwarded-proto", url.protocol.replace(":", ""));
  const started = performance.now();
  try {
    const upstream = await fetch(target, {
      method: req.method,
      headers,
      body: req.body,
      // OAuth flows redirect the BROWSER (302 to GitHub) — pass
      // redirects through instead of following them server-side.
      redirect: "manual",
    });
    // The OAuth-debugging line: where we sent the request, what came
    // back, and which host the browser is about to be redirected to.
    // Never the Location query — that carries client id and state.
    log.debug("proxy.request", {
      method,
      route,
      target_host: target.host,
      upstream_status: upstream.status,
      location_host: redactLocation(upstream.headers.get("location")),
      has_cookie: hasCookie(req.headers),
      duration_ms: Math.round(performance.now() - started),
    });
    return new Response(upstream.body, {
      status: upstream.status,
      headers: upstream.headers,
    });
  } catch (err) {
    // Binding this was the single highest-value fix in DESIGN-0006: the
    // cause used to be discarded at the language level, leaving an
    // operator with a bare 502 and no way to learn whether it was DNS,
    // a refused connection, TLS, or a timeout.
    log.error("proxy.error", {
      method,
      route,
      reason: "unreachable",
      target_host: target.host,
      err_name: err instanceof Error ? err.name : "unknown",
      err_message: err instanceof Error ? err.message : String(err),
      duration_ms: Math.round(performance.now() - started),
    });
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

/** A response plus the fact only the serving path knows. */
interface Served {
  response: Response;
  servedFromDisk: boolean;
}

/** Today's routing, unchanged, reporting whether a file was found. */
async function route(req: Request, url: URL): Promise<Served> {
  const { pathname } = url;

  if (isProxiedPath(pathname)) {
    const routeClass = classifyRoute(pathname, false);
    return {
      response: await proxy(req, url, routeClass),
      servedFromDisk: false,
    };
  }
  if (req.method !== "GET" && req.method !== "HEAD") {
    return {
      response: new Response("method not allowed", { status: 405 }),
      servedFromDisk: false,
    };
  }

  if (pathname !== "/") {
    const filePath = safeDistPath(pathname);
    if (filePath === null) {
      return {
        response: new Response("bad request", { status: 400 }),
        servedFromDisk: false,
      };
    }
    const file = await serveFile(req, filePath, cacheControl(pathname));
    if (file !== null) {
      return { response: file, servedFromDisk: true };
    }
  }
  // SPA fallback: the router owns every other path.
  return { response: await serveIndex(), servedFromDisk: false };
}

export async function handleRequest(req: Request): Promise<Response> {
  const url = new URL(req.url);
  const { pathname } = url;

  // Probes short-circuit before any signal is recorded. A kubelet
  // hitting these every 10s would otherwise be most of the log volume
  // on a quiet docs site.
  if (PROBE_PATHS.has(pathname)) {
    return await probe(pathname);
  }

  const started = performance.now();
  const { response, servedFromDisk } = await route(req, url);
  // One emission point, so the log line can never disagree with the
  // metric and span that join it here in later phases.
  log.debug("http.request", {
    method: normalizeMethod(req.method),
    route: classifyRoute(pathname, servedFromDisk),
    path: redactUrl(url),
    status: response.status,
    duration_ms: Math.round(performance.now() - started),
  });
  return response;
}

/** Operational endpoints. `/metrics` arrives in a later phase. */
async function probe(pathname: string): Promise<Response> {
  if (pathname === "/healthz") {
    // Liveness only, and unconditional by design — see the readiness
    // comment above for why this one must never check anything.
    return new Response("ok", { headers: { "cache-control": "no-store" } });
  }
  if (pathname === "/readyz") {
    return await readyz();
  }
  return new Response("not found", { status: 404 });
}

async function readyz(): Promise<Response> {
  const report = await checkReady(DIST);
  if (!report.ready) {
    // The failing var NAMES go to the log, never to the response: the
    // body is reachable by anything that can reach the pod.
    log.warn("readyz.fail", {
      dist: report.checks.dist,
      config: report.checks.config,
      invalid: report.invalid.join(",") || undefined,
    });
  }
  return Response.json(
    {
      status: report.ready ? "ready" : "not ready",
      checks: report.checks,
    },
    {
      status: report.ready ? 200 : 503,
      headers: { "cache-control": "no-store" },
    },
  );
}

// Guard startup so tests can import the pure helpers above without
// binding a port (import.meta.main is true only for the entrypoint).
if (import.meta.main) {
  const server = startServer();
  log.info("server.start", {
    port: server.port,
    dist: DIST,
    auth_providers: AUTH_PROVIDERS.join(","),
    nav_links: NAV_LINKS.length,
    mermaid_layout: MERMAID_LAYOUT,
    proxy_target: DOCZ_API_URL ?? "(none — API proxy disabled)",
    log_level: LOG_LEVEL,
    log_format: LOG_FORMAT,
  });
}
