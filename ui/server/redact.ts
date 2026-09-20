/*
 * Redaction (DESIGN-0006 Component 2).
 *
 * Issue #18 calls redaction non-negotiable. This module is what makes
 * it structural rather than a habit: nothing else in the server is
 * allowed to put a URL, a header, or a redirect target into a log line
 * or a span attribute without coming through here first.
 *
 * The proxy forwards `/auth/*` verbatim — including the OAuth callback
 * carrying `?code=…&state=…` — and copies every inbound header,
 * including the `docz_session` cookie. So the hottest path in the app
 * is also the one carrying credentials.
 *
 * Pure: no I/O, no globals.
 */

/** Stand-in for any value we refuse to record. */
export const REDACTED = "<redacted>";

/**
 * Query parameters whose VALUES may be recorded verbatim.
 *
 * Deliberately empty. This is an allowlist, not a denylist: a denylist
 * of `code`/`state` fails open the first time an identity provider
 * introduces a new credential-bearing parameter, and we would not find
 * out. Add a key here only with a stated reason.
 */
const SAFE_QUERY_VALUE_KEYS = new Set<string>();

/**
 * Replace control characters, which would otherwise let a crafted path
 * forge or split a line in text-mode log output. JSON mode escapes
 * them anyway; this makes both modes safe by construction.
 */
function stripControl(value: string): string {
  // eslint-disable-next-line no-control-regex -- the point is control chars
  return value.replace(/[\u0000-\u001f\u007f]/g, "\uFFFD");
}

/**
 * A URL safe to record: path kept, every query value replaced.
 *
 * Query KEYS survive, because "the callback arrived without `state`" is
 * exactly the misconfiguration an operator is hunting, while the value
 * is a credential. Duplicate keys and ordering are preserved so the
 * shape of the real request is still legible.
 */
export function redactUrl(url: URL): string {
  const path = stripControl(url.pathname);
  if (url.search === "") {
    return path;
  }
  const parts: string[] = [];
  for (const [key, value] of url.searchParams) {
    const safeKey = stripControl(key);
    parts.push(
      SAFE_QUERY_VALUE_KEYS.has(key)
        ? `${safeKey}=${stripControl(value)}`
        : `${safeKey}=${REDACTED}`,
    );
  }
  return `${path}?${parts.join("&")}`;
}

/**
 * Reduce a redirect target to its host.
 *
 * The OAuth 302 carries the full authorize URL, with client id and
 * state in the query. "Did we send them to the right identity
 * provider?" is answerable from the host alone; the rest is not ours to
 * log. Returns undefined when there is no header or it does not parse,
 * so a caller simply omits the field.
 */
export function redactLocation(location: string | null): string | undefined {
  if (location === null || location === "") {
    return undefined;
  }
  try {
    return stripControl(new URL(location).host);
  } catch {
    // Relative redirect (no host to report) or malformed input.
    return undefined;
  }
}

/**
 * Whether the request carried a cookie — the only header fact recorded
 * anywhere. Header VALUES are never logged or attached to spans, with
 * no allowlist and no exceptions.
 */
export function hasCookie(headers: Headers): boolean {
  return headers.has("cookie");
}
