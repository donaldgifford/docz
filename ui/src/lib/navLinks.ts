/*
 * Topbar nav pins (DESIGN-0002 Component 1): a deployment-chosen list
 * of {label, href} links rendered between Repos and the session menu.
 * The runtime config injected by server/serve.ts (DOCZ_NAV_LINKS)
 * wins; the build-time VITE_NAV_LINKS JSON is the dev/e2e fallback;
 * else no pins. Both ends validate — the server sanitized before
 * injecting, and this module re-validates whatever it reads (the
 * authReturn stash discipline), so a href only ever reaches a router
 * NavLink as a same-origin app path.
 */

export interface NavLink {
  label: string;
  href: string;
}

const NAV_LINK_CAP = 6;
const NAV_LABEL = /^[\w .&+-]{1,24}$/;
const NAV_HREF_MAX = 200;

/**
 * App paths only: leading "/" but never "//", printable ASCII, no
 * whitespace/control, none of <>"'`\ — mirrors server/serve.ts.
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

/** Validate an unknown payload (injected array or parsed JSON). */
export function parseNavLinks(payload: unknown): NavLink[] {
  if (!Array.isArray(payload)) {
    return [];
  }
  const links: NavLink[] = [];
  for (const entry of payload) {
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

/** The VITE_NAV_LINKS shape: the same JSON array, baked at build time. */
export function navLinksFromJson(raw: string | undefined): NavLink[] {
  if (raw === undefined || raw.trim() === "") {
    return [];
  }
  try {
    return parseNavLinks(JSON.parse(raw));
  } catch {
    return [];
  }
}

function runtimeNavLinks(): NavLink[] | undefined {
  if (typeof window === "undefined") {
    return undefined;
  }
  const nav = window.__DOCZ_CONFIG__?.nav;
  // An injected array is authoritative even when it validates to [] —
  // the deployment explicitly chose its pin set (possibly none).
  return Array.isArray(nav) ? parseNavLinks(nav) : undefined;
}

export function enabledNavLinks(): NavLink[] {
  return (
    runtimeNavLinks() ??
    navLinksFromJson(import.meta.env.VITE_NAV_LINKS as string | undefined)
  );
}
