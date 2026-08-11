import { describe, expect, test } from "bun:test";

import {
  injectRuntimeConfig,
  resolveAuthProviders,
  resolveNavLinks,
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

describe("resolveNavLinks", () => {
  test("returns [] when unset, empty, or unparseable", () => {
    expect(resolveNavLinks(undefined)).toEqual([]);
    expect(resolveNavLinks("")).toEqual([]);
    expect(resolveNavLinks("   ")).toEqual([]);
    expect(resolveNavLinks("not json")).toEqual([]);
    expect(resolveNavLinks('{"label":"x","href":"/x"}')).toEqual([]);
    expect(resolveNavLinks('"a string"')).toEqual([]);
  });

  test("keeps valid entries in order", () => {
    expect(
      resolveNavLinks(
        '[{"label":"RFCs","href":"/donaldgifford/rfcs"},' +
          '{"label":"Team Docs","href":"/donaldgifford/docs/docs"}]',
      ),
    ).toEqual([
      { label: "RFCs", href: "/donaldgifford/rfcs" },
      { label: "Team Docs", href: "/donaldgifford/docs/docs" },
    ]);
  });

  test("caps at six entries", () => {
    const many = JSON.stringify(
      Array.from({ length: 9 }, (_, i) => ({
        label: `Pin ${String(i)}`,
        href: `/pin/${String(i)}`,
      })),
    );
    expect(resolveNavLinks(many)).toHaveLength(6);
  });

  test("drops invalid entries but keeps the rest", () => {
    const mixed = JSON.stringify([
      { label: "ok", href: "/fine" },
      { label: "<img onerror=alert(1)>", href: "/x" }, // label charset
      { label: "way too long a label for the topbar row", href: "/x" },
      { label: "no href" },
      { label: "rel", href: "relative/path" }, // no leading /
      { label: "proto", href: "//evil.example" }, // protocol-relative
      { label: "abs", href: "https://evil.example" },
      { label: "js", href: "javascript:alert(1)" },
      { label: "space", href: "/has space" },
      { label: "markup", href: '/x"><script>alert(1)</script>' },
      { label: "long", href: `/${"a".repeat(200)}` }, // > 200 total
      "not an object",
      null,
      { label: "also ok", href: "/fine?tab=1#frag" },
    ]);
    expect(resolveNavLinks(mixed)).toEqual([
      { label: "ok", href: "/fine" },
      { label: "also ok", href: "/fine?tab=1#frag" },
    ]);
  });
});

describe("runtimeConfigScript / injectRuntimeConfig", () => {
  test("emits a script that publishes the validated provider list", () => {
    const script = runtimeConfigScript(["keycloak", "github"], []);
    expect(script).toBe(
      '<script>window.__DOCZ_CONFIG__={"authProviders":["keycloak","github"],"nav":[]};</script>',
    );
  });

  test("closed whitelist means no HTML/JS breakout is possible", () => {
    // Whatever the env, the script body only ever contains whitelist keys.
    const script = runtimeConfigScript(
      resolveAuthProviders("okta</script>"),
      [],
    );
    expect(script).not.toContain("</script></script>");
    expect(script).toContain('{"authProviders":["github"]');
  });

  test("injects the config ahead of the entry bundle (Vite head-script)", () => {
    // Vite emits the entry <script type="module"> inside <head>.
    const html =
      '<!doctype html><html><head><meta charset="UTF-8" />' +
      '<script type="module" crossorigin src="/assets/index.js"></script>' +
      '</head><body><div id="root"></div></body></html>';
    const out = injectRuntimeConfig(html, runtimeConfigScript(["okta"], []));
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

  test("hostile DOCZ_NAV_LINKS can never break out of the script", () => {
    // Everything markup-shaped dies in validation…
    const hostile = JSON.stringify([
      { label: "x", href: "/x</script><script>alert(1)</script>" },
      { label: "</script>", href: "/y" },
    ]);
    const script = runtimeConfigScript(["github"], resolveNavLinks(hostile));
    expect(script).toBe(
      '<script>window.__DOCZ_CONFIG__={"authProviders":["github"],"nav":[]};</script>',
    );
    // …and even a value that somehow carried "</" is escaped so the
    // parser can't see a terminator mid-string.
    const belt = runtimeConfigScript(
      ["github"],
      [{ label: "x", href: "/x</script>" }],
    );
    expect(belt).not.toContain('href":"/x</script>');
    expect(belt).toContain("<\\/script>");
  });
});
