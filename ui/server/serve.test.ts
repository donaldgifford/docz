import { describe, expect, test } from "bun:test";

import {
  injectRuntimeConfig,
  resolveAuthProviders,
  runtimeConfigScript,
} from "./serve";

describe("resolveAuthProviders", () => {
  test("defaults to GitHub when unset or empty", () => {
    expect(resolveAuthProviders(undefined)).toEqual(["github"]);
    expect(resolveAuthProviders("")).toEqual(["github"]);
  });

  test("keeps whitelisted providers in order", () => {
    expect(resolveAuthProviders("keycloak,github")).toEqual([
      "keycloak",
      "github",
    ]);
  });

  test("normalizes case/space and dedupes", () => {
    expect(resolveAuthProviders(" Okta , OKTA ,github")).toEqual([
      "okta",
      "github",
    ]);
  });

  test("drops unknown keys and falls back to GitHub when all are unknown", () => {
    expect(resolveAuthProviders("okta,facebook")).toEqual(["okta"]);
    expect(resolveAuthProviders("facebook, google")).toEqual(["github"]);
  });
});

describe("runtimeConfigScript / injectRuntimeConfig", () => {
  test("emits a script that publishes the validated provider list", () => {
    const script = runtimeConfigScript(["keycloak", "github"]);
    expect(script).toBe(
      '<script>window.__DOCZ_CONFIG__={"authProviders":["keycloak","github"]};</script>',
    );
  });

  test("closed whitelist means no HTML/JS breakout is possible", () => {
    // Whatever the env, the script body only ever contains whitelist keys.
    const script = runtimeConfigScript(resolveAuthProviders("okta</script>"));
    expect(script).not.toContain("</script></script>");
    expect(script).toContain('{"authProviders":["github"]}');
  });

  test("injects the config ahead of the entry bundle (Vite head-script)", () => {
    // Vite emits the entry <script type="module"> inside <head>.
    const html =
      '<!doctype html><html><head><meta charset="UTF-8" />' +
      '<script type="module" crossorigin src="/assets/index.js"></script>' +
      '</head><body><div id="root"></div></body></html>';
    const out = injectRuntimeConfig(html, runtimeConfigScript(["okta"]));
    expect(out).toContain("__DOCZ_CONFIG__");
    // Textually before the entry bundle — not relying on module defer.
    expect(out.indexOf("__DOCZ_CONFIG__")).toBeLessThan(
      out.indexOf("/assets/index.js"),
    );
    // Inside <head>, and charset stays first.
    expect(out.indexOf("__DOCZ_CONFIG__")).toBeLessThan(out.indexOf("</head>"));
    expect(out.indexOf("charset")).toBeLessThan(out.indexOf("__DOCZ_CONFIG__"));
  });

  test("falls back to prepending when there is no <head>", () => {
    const out = injectRuntimeConfig("<body>x</body>", "<script>y</script>");
    expect(out.startsWith("<script>y</script>")).toBe(true);
  });
});
