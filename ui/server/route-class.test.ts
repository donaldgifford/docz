import { describe, expect, test } from "bun:test";

import {
  classifyRoute,
  isProxiedPath,
  normalizeMethod,
  OTHER_METHOD,
  PROBE_PATHS,
  type RouteClass,
} from "./route-class";

describe("classifyRoute", () => {
  test.each([
    ["/healthz", "probe"],
    ["/readyz", "probe"],
    ["/metrics", "probe"],
  ] as const)("%s is a probe", (path, expected) => {
    // Probes classify the same either way: they never reach the disk.
    expect(classifyRoute(path, false)).toBe(expected);
    expect(classifyRoute(path, true)).toBe(expected);
  });

  test.each([
    ["/api", "proxy:api"],
    ["/api/v1/repos", "proxy:api"],
    ["/auth", "proxy:auth"],
    ["/auth/login", "proxy:auth"],
    ["/webhooks", "proxy:webhooks"],
    ["/webhooks/github", "proxy:webhooks"],
    ["/openapi.yaml", "proxy:openapi"],
  ] as const)("%s proxies to %s", (path, expected) => {
    expect(classifyRoute(path, false)).toBe(expected);
  });

  test("a hashed asset served from disk is an asset", () => {
    expect(classifyRoute("/assets/index-a1b2c3.js", true)).toBe("asset");
  });

  test("another real file is static", () => {
    expect(classifyRoute("/favicon.ico", true)).toBe("static");
  });

  test("anything not on disk is spa, including a missing asset", () => {
    // A missing hashed asset falls through to the index.html fallback,
    // so `spa` is the honest label — it describes what was served.
    expect(classifyRoute("/assets/gone-000000.js", false)).toBe("spa");
    expect(
      classifyRoute("/donaldgifford/docz-site/design/DESIGN-0001", false),
    ).toBe("spa");
    expect(classifyRoute("/", false)).toBe("spa");
  });

  test("proxy prefixes do not match a same-named SPA path", () => {
    // "/apiary" must not be mistaken for the "/api/" prefix.
    expect(classifyRoute("/apiary", false)).toBe("spa");
    expect(classifyRoute("/authors", false)).toBe("spa");
  });

  const VALID: ReadonlySet<RouteClass> = new Set([
    "asset",
    "static",
    "spa",
    "proxy:api",
    "proxy:auth",
    "proxy:webhooks",
    "proxy:openapi",
    "probe",
  ]);

  test("hostile paths stay inside the closed set and are never echoed", () => {
    const hostile = [
      "/a/../../etc/passwd",
      `/${"x".repeat(2048)}`,
      // Built rather than written literally: a raw control byte in
      // source silently turns the file binary in git (PR #28).
      `/${String.fromCharCode(0)}/null-byte`,
      `/nl${String.fromCharCode(10)}forged-log-line`,
      "/<script>alert(1)</script>",
      "/%2e%2e%2f%2e%2e%2f",
      "/api/../auth/login",
      "/../../../",
      "/assets/../../secret",
    ];
    for (const path of hostile) {
      for (const served of [true, false]) {
        // The whole point: every label comes from the closed set, so no
        // request — however crafted — can mint a new metric series.
        expect(VALID.has(classifyRoute(path, served))).toBe(true);
      }
    }
  });

  test("the label set never grows beyond the eight declared values", () => {
    // Guards the cardinality property directly: 2000 distinct paths
    // must still collapse to no more than the eight declared labels.
    const seen = new Set<RouteClass>();
    for (let i = 0; i < 1000; i++) {
      seen.add(classifyRoute(`/owner/repo/design/DESIGN-${String(i)}`, false));
      seen.add(classifyRoute(`/assets/chunk-${String(i)}.js`, true));
    }
    expect(seen.size).toBeLessThanOrEqual(VALID.size);
    for (const label of seen) {
      expect(VALID.has(label)).toBe(true);
    }
  });

  test("every probe path is covered by PROBE_PATHS", () => {
    for (const path of PROBE_PATHS) {
      expect(classifyRoute(path, false)).toBe("probe");
    }
  });
});

describe("isProxiedPath", () => {
  test("matches exactly the proxied surface", () => {
    for (const path of [
      "/api",
      "/api/v1",
      "/auth",
      "/auth/login",
      "/webhooks",
      "/webhooks/github",
      "/openapi.yaml",
    ]) {
      expect(isProxiedPath(path)).toBe(true);
    }
    for (const path of ["/", "/login", "/apiary", "/healthz", "/assets/a.js"]) {
      expect(isProxiedPath(path)).toBe(false);
    }
  });
});

describe("normalizeMethod", () => {
  test.each(["GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"])(
    "keeps %s",
    (method) => {
      expect(normalizeMethod(method)).toBe(method);
    },
  );

  test("normalizes case", () => {
    expect(normalizeMethod("get")).toBe("GET");
    expect(normalizeMethod("Post")).toBe("POST");
  });

  test("collapses anything else — fetch() accepts arbitrary tokens", () => {
    for (const method of [
      "TRACE",
      "CONNECT",
      "PROPFIND",
      "FOOBAR",
      "",
      "x".repeat(500),
      "GET\r\nX-Injected: 1",
    ]) {
      expect(normalizeMethod(method)).toBe(OTHER_METHOD);
    }
  });
});
