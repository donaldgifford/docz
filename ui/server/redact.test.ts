import { describe, expect, test } from "bun:test";

import { hasCookie, REDACTED, redactLocation, redactUrl } from "./redact";

/** A realistic OAuth callback — the request this module exists for. */
const CALLBACK = new URL(
  "https://docs.example.com/auth/callback" +
    "?code=4%2F0AY0e-g7pK9xLmQ2vR8sT1uV3wX5yZ&state=abc123xyz",
);

describe("redactUrl", () => {
  test("keeps the path", () => {
    expect(redactUrl(new URL("https://x.test/auth/login"))).toBe("/auth/login");
  });

  test("a path with no query is returned unchanged", () => {
    expect(redactUrl(new URL("https://x.test/"))).toBe("/");
  });

  test("keeps query KEYS but never their values", () => {
    const out = redactUrl(CALLBACK);
    expect(out).toBe(`/auth/callback?code=${REDACTED}&state=${REDACTED}`);
    // The keys are the diagnostic: "the callback arrived without state".
    expect(out).toContain("code=");
    expect(out).toContain("state=");
  });

  test("the credential values are absent in every encoding", () => {
    const out = redactUrl(CALLBACK);
    for (const secret of [
      "4/0AY0e-g7pK9xLmQ2vR8sT1uV3wX5yZ", // decoded
      "4%2F0AY0e-g7pK9xLmQ2vR8sT1uV3wX5yZ", // as sent
      "abc123xyz",
    ]) {
      expect(out).not.toContain(secret);
    }
  });

  test("an UNKNOWN key is redacted too — allowlist, not denylist", () => {
    // The whole point: a provider adding a new credential-bearing
    // parameter tomorrow is redacted without anyone updating a list.
    const url = new URL("https://x.test/auth/cb?id_token=header.payload.sig");
    expect(redactUrl(url)).toBe(`/auth/cb?id_token=${REDACTED}`);
  });

  test("duplicate keys and ordering survive", () => {
    const url = new URL("https://x.test/s?a=1&b=2&a=3");
    expect(redactUrl(url)).toBe(`/s?a=${REDACTED}&b=${REDACTED}&a=${REDACTED}`);
  });

  test("control characters cannot forge a log line", () => {
    // A newline in text-mode output would let a crafted request write
    // what looks like a second log entry.
    const nl = String.fromCharCode(10);
    const url = new URL(`https://x.test/p${encodeURIComponent(nl)}x?q=1`);
    const out = redactUrl(url);
    expect(out).not.toContain(nl);
    expect(out).not.toContain(String.fromCharCode(0));
  });
});

describe("redactLocation", () => {
  test("reduces an OAuth authorize URL to its host", () => {
    const location =
      "https://login.okta.example.com/oauth2/v1/authorize" +
      "?client_id=0oa1b2c3&state=abc123xyz&scope=openid";
    expect(redactLocation(location)).toBe("login.okta.example.com");
  });

  test("the query — client id and state — never survives", () => {
    const out = redactLocation(
      "https://idp.test/authorize?client_id=0oa1b2c3&state=abc123xyz",
    );
    expect(out).not.toContain("client_id");
    expect(out).not.toContain("abc123xyz");
  });

  test("keeps a non-default port, which is diagnostic", () => {
    expect(redactLocation("http://keycloak.test:8443/realms/x")).toBe(
      "keycloak.test:8443",
    );
  });

  test("undefined for absent, empty, relative, or malformed", () => {
    expect(redactLocation(null)).toBeUndefined();
    expect(redactLocation("")).toBeUndefined();
    expect(redactLocation("/relative/path")).toBeUndefined();
    expect(redactLocation("::::")).toBeUndefined();
  });
});

describe("hasCookie", () => {
  test("reports presence only, never the value", () => {
    const withCookie = new Headers({ cookie: "docz_session=opaque-value" });
    expect(hasCookie(withCookie)).toBe(true);
    // The function's return type is the guarantee: a boolean cannot
    // carry the session id.
    expect(typeof hasCookie(withCookie)).toBe("boolean");
    expect(hasCookie(new Headers())).toBe(false);
  });
});
