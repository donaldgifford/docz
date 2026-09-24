/*
 * Route classification (DESIGN-0006 Component 1).
 *
 * One classifier shared by logs, metrics, and traces, so the three can
 * never disagree about what a request was, and so none of them can
 * independently blow up label cardinality.
 *
 * Both inputs here are attacker-controlled. SPA paths are unbounded —
 * `/donaldgifford/docz-site/design/DESIGN-0001` is a real one, and any
 * URL a browser requests would become a label value — and `fetch()`
 * accepts arbitrary method tokens. Everything this module returns is
 * therefore drawn from a closed set, never echoed from the request.
 *
 * Pure: no I/O, no globals, imports nothing from serve.ts. The proxy
 * constants live here rather than in serve.ts because this is the
 * module that has to reason about them exhaustively.
 */

/**
 * The complete set of route labels. Eight values, closed — they map
 * onto the branches `handleRequest` already takes, so classification
 * reads a decision that has already been made rather than re-parsing
 * the URL.
 */
export type RouteClass =
  | "asset"
  | "static"
  | "spa"
  | "proxy:api"
  | "proxy:auth"
  | "proxy:webhooks"
  | "proxy:openapi"
  | "probe";

/** Operational endpoints, excluded from every signal. */
export const PROBE_PATHS = new Set(["/healthz", "/readyz", "/metrics"]);

/** Prefixes proxied to docz-api, paired with their label. */
const PROXY_PREFIX_CLASSES: readonly (readonly [string, RouteClass])[] = [
  ["/api/", "proxy:api"],
  ["/auth/", "proxy:auth"],
  ["/webhooks/", "proxy:webhooks"],
];

/** Exact paths proxied to docz-api, paired with their label. */
const PROXY_EXACT_CLASSES = new Map<string, RouteClass>([
  ["/api", "proxy:api"],
  ["/auth", "proxy:auth"],
  ["/webhooks", "proxy:webhooks"],
  ["/openapi.yaml", "proxy:openapi"],
]);

/** Vite content-hashes everything under this prefix. */
export const IMMUTABLE_PREFIX = "/assets/";

/** Methods we label verbatim; anything else collapses to `other`. */
const KNOWN_METHODS = new Set([
  "GET",
  "HEAD",
  "POST",
  "PUT",
  "PATCH",
  "DELETE",
  "OPTIONS",
]);

/** The label used for any method outside {@link KNOWN_METHODS}. */
export const OTHER_METHOD = "other";

/**
 * Collapse a request method to a bounded label.
 *
 * `fetch()` accepts arbitrary method tokens, so this is the same class
 * of untrusted input as the path — not a formality.
 */
export function normalizeMethod(method: string): string {
  const upper = method.toUpperCase();
  return KNOWN_METHODS.has(upper) ? upper : OTHER_METHOD;
}

/** True when this path is proxied to docz-api. */
export function isProxiedPath(pathname: string): boolean {
  return proxyClass(pathname) !== undefined;
}

function proxyClass(pathname: string): RouteClass | undefined {
  const exact = PROXY_EXACT_CLASSES.get(pathname);
  if (exact !== undefined) {
    return exact;
  }
  for (const [prefix, routeClass] of PROXY_PREFIX_CLASSES) {
    if (pathname.startsWith(prefix)) {
      return routeClass;
    }
  }
  return undefined;
}

/**
 * Classify a request.
 *
 * `servedFromDisk` distinguishes `static`/`asset` from `spa`, and is
 * only knowable after the file lookup — which is why the class is
 * finalised at response time rather than on entry. A request for a
 * hashed asset that is NOT on disk falls through to the index.html
 * fallback, so it is honestly labelled `spa`: the label describes what
 * was served, not what was asked for.
 */
export function classifyRoute(
  pathname: string,
  servedFromDisk: boolean,
): RouteClass {
  if (PROBE_PATHS.has(pathname)) {
    return "probe";
  }
  const proxied = proxyClass(pathname);
  if (proxied !== undefined) {
    return proxied;
  }
  if (!servedFromDisk) {
    return "spa";
  }
  return pathname.startsWith(IMMUTABLE_PREFIX) ? "asset" : "static";
}
