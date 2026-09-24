/*
 * The redaction gate (DESIGN-0006 Testing Strategy).
 *
 * Issue #18 calls redaction non-negotiable, so this is a security gate
 * in the spirit of the XSS suite rather than an ordinary unit test. It
 * drives a realistic OAuth callback through the SERIALISED logger
 * output at every level and asserts the credential values never appear.
 *
 * Two deliberate choices:
 *
 *   - It asserts on the finished line, not on the fields handed in. A
 *     test that checks its own inputs proves nothing about what lands
 *     in a log aggregator.
 *   - It is parameterised over every level in LOG_LEVELS, so adding a
 *     `trace` level later cannot quietly open a path around it.
 */
import { describe, expect, test } from "bun:test";

import { createLogger, LOG_FORMATS, LOG_LEVELS } from "./logger";
import { hasCookie, redactLocation, redactUrl } from "./redact";
import { classifyRoute, normalizeMethod } from "./route-class";

/** The authorization code and state a real provider would send back. */
const CODE = "4/0AY0e-g7pK9xLmQ2vR8sT1uV3wX5yZ";
const STATE = "n0nce-9f8e7d6c5b4a3210";
const SESSION = "docz_session=s%3AopaqueSessionIdentifier.signature";

const CALLBACK_URL = new URL(
  `https://docs.example.com/auth/callback?code=${encodeURIComponent(CODE)}` +
    `&state=${encodeURIComponent(STATE)}`,
);

/** The IdP redirect the proxy passes back to the browser. */
const AUTHORIZE_LOCATION =
  `https://login.okta.example.com/oauth2/v1/authorize` +
  `?client_id=0oa1b2c3d4e5f6&state=${encodeURIComponent(STATE)}&scope=openid`;

/** Every secret that must never reach a log, in every encoding. */
const SECRETS = [
  CODE,
  encodeURIComponent(CODE),
  STATE,
  encodeURIComponent(STATE),
  SESSION,
  "opaqueSessionIdentifier",
  "0oa1b2c3d4e5f6",
];

/**
 * Log the OAuth journey exactly as serve.ts does — same redact calls,
 * same field names — and return every line that would hit stdout.
 */
function journeyLines(
  level: (typeof LOG_LEVELS)[number],
  format: (typeof LOG_FORMATS)[number],
): string[] {
  const lines: string[] = [];
  const log = createLogger({
    level,
    format,
    write: (line) => lines.push(line),
  });

  const headers = new Headers({
    cookie: SESSION,
    authorization: "Bearer a-token-that-must-not-be-logged",
  });
  const route = classifyRoute(CALLBACK_URL.pathname, false);

  log.debug("http.request", {
    method: normalizeMethod("GET"),
    route,
    path: redactUrl(CALLBACK_URL),
    status: 302,
    duration_ms: 12,
  });
  log.debug("proxy.request", {
    method: normalizeMethod("GET"),
    route,
    target_host: "docz-api:8080",
    upstream_status: 302,
    location_host: redactLocation(AUTHORIZE_LOCATION),
    has_cookie: hasCookie(headers),
    duration_ms: 11,
  });
  log.error("proxy.error", {
    method: normalizeMethod("GET"),
    route,
    reason: "unreachable",
    target_host: "docz-api:8080",
    err_name: "ConnectionRefused",
    err_message: "Unable to connect",
    duration_ms: 9,
  });
  return lines;
}

describe("redaction gate: OAuth credentials never reach the log", () => {
  for (const level of LOG_LEVELS) {
    for (const format of LOG_FORMATS) {
      test(`level=${level} format=${format}`, () => {
        const output = journeyLines(level, format).join("\n");
        for (const secret of SECRETS) {
          expect(output).not.toContain(secret);
        }
      });
    }
  }

  test("the diagnostic shape survives — keys and hosts are still there", () => {
    // Redaction that removed everything useful would pass the checks
    // above while making the log worthless, so pin what must REMAIN.
    const output = journeyLines("debug", "json").join("\n");
    expect(output).toContain("code=");
    expect(output).toContain("state=");
    expect(output).toContain("login.okta.example.com");
    expect(output).toContain("docz-api:8080");
    expect(output).toContain("/auth/callback");
  });

  test("header values never appear, only cookie presence", () => {
    const output = journeyLines("debug", "json").join("\n");
    expect(output).not.toContain("Bearer");
    expect(output).not.toContain("a-token-that-must-not-be-logged");
    expect(output).toContain('"has_cookie":true');
  });

  test("the gate would FAIL if redaction were bypassed", () => {
    // Verify the guard fires before trusting it green: log the same URL
    // WITHOUT redactUrl and confirm the assertion above would catch it.
    const lines: string[] = [];
    const log = createLogger({
      level: "debug",
      format: "json",
      write: (line) => lines.push(line),
    });
    log.debug("http.request", { path: CALLBACK_URL.toString() });
    const leaked = lines.join("\n");
    expect(SECRETS.some((secret) => leaked.includes(secret))).toBe(true);
  });
});
