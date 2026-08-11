import { afterEach, describe, expect, it, vi } from "vitest";

import {
  enabledNavLinks,
  navLinksFromJson,
  parseNavLinks,
} from "@/lib/navLinks";

afterEach(() => {
  delete window.__DOCZ_CONFIG__;
  vi.unstubAllEnvs();
});

describe("parseNavLinks", () => {
  it("keeps valid entries in order and caps at six", () => {
    const nine = Array.from({ length: 9 }, (_, i) => ({
      label: `Pin ${String(i)}`,
      href: `/pin/${String(i)}`,
    }));
    const links = parseNavLinks(nine);
    expect(links).toHaveLength(6);
    expect(links[0]).toEqual({ label: "Pin 0", href: "/pin/0" });
  });

  it("drops hostile and malformed entries but keeps the rest", () => {
    expect(
      parseNavLinks([
        { label: "RFCs", href: "/donaldgifford/rfcs" },
        { label: "<img onerror=alert(1)>", href: "/x" },
        { label: "a label far too long for the topbar row", href: "/x" },
        { label: "abs", href: "https://evil.example" },
        { label: "proto", href: "//evil.example" },
        { label: "js", href: "javascript:alert(1)" },
        { label: "space", href: "/has space" },
        { label: "markup", href: '/x"><script>alert(1)</script>' },
        { label: "rel", href: "relative/path" },
        "not an object",
        null,
        { label: "Docs", href: "/donaldgifford/docs/docs#top" },
      ]),
    ).toEqual([
      { label: "RFCs", href: "/donaldgifford/rfcs" },
      { label: "Docs", href: "/donaldgifford/docs/docs#top" },
    ]);
  });

  it("returns [] for non-array payloads", () => {
    expect(parseNavLinks(undefined)).toEqual([]);
    expect(parseNavLinks({ label: "x", href: "/x" })).toEqual([]);
    expect(parseNavLinks("[]")).toEqual([]);
  });
});

describe("navLinksFromJson", () => {
  it("parses the build-time JSON and survives garbage", () => {
    expect(navLinksFromJson('[{"label":"RFCs","href":"/r"}]')).toEqual([
      { label: "RFCs", href: "/r" },
    ]);
    expect(navLinksFromJson(undefined)).toEqual([]);
    expect(navLinksFromJson("   ")).toEqual([]);
    expect(navLinksFromJson("not json")).toEqual([]);
  });
});

describe("enabledNavLinks precedence", () => {
  it("prefers the injected runtime config over VITE_NAV_LINKS", () => {
    window.__DOCZ_CONFIG__ = { nav: [{ label: "Runtime", href: "/rt" }] };
    vi.stubEnv("VITE_NAV_LINKS", '[{"label":"Baked","href":"/baked"}]');
    expect(enabledNavLinks()).toEqual([{ label: "Runtime", href: "/rt" }]);
  });

  it("treats an injected empty array as authoritative", () => {
    // The deployment explicitly chose zero pins; don't resurrect the
    // build-time set.
    window.__DOCZ_CONFIG__ = { nav: [] };
    vi.stubEnv("VITE_NAV_LINKS", '[{"label":"Baked","href":"/baked"}]');
    expect(enabledNavLinks()).toEqual([]);
  });

  it("falls back to VITE_NAV_LINKS when nothing was injected", () => {
    vi.stubEnv("VITE_NAV_LINKS", '[{"label":"Baked","href":"/baked"}]');
    expect(enabledNavLinks()).toEqual([{ label: "Baked", href: "/baked" }]);
  });

  it("re-validates the injected config (both-ends rule)", () => {
    window.__DOCZ_CONFIG__ = {
      nav: [
        { label: "ok", href: "/fine" },
        { label: "evil", href: "javascript:alert(1)" },
      ],
    };
    expect(enabledNavLinks()).toEqual([{ label: "ok", href: "/fine" }]);
  });

  it("returns [] with neither source configured", () => {
    expect(enabledNavLinks()).toEqual([]);
  });
});
