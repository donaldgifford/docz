import { afterEach, describe, expect, it, vi } from "vitest";

import {
  enabledProviders,
  lastUsedProvider,
  parseProviders,
  promoteLastUsed,
  rememberProvider,
} from "@/lib/authProviders";

describe("parseProviders", () => {
  it("defaults to GitHub alone", () => {
    expect(parseProviders(undefined)).toEqual([
      { key: "github", label: "GitHub" },
    ]);
  });

  it("parses a configured list, preserving order", () => {
    expect(parseProviders("github, okta,keycloak")).toEqual([
      { key: "github", label: "GitHub" },
      { key: "okta", label: "Okta" },
      { key: "keycloak", label: "Keycloak" },
    ]);
  });

  it("normalizes case, trims, and dedupes", () => {
    expect(parseProviders(" Okta ,OKTA, github ")).toEqual([
      { key: "okta", label: "Okta" },
      { key: "github", label: "GitHub" },
    ]);
  });

  it("drops unknown providers rather than rendering dead buttons", () => {
    expect(parseProviders("github,facebook")).toEqual([
      { key: "github", label: "GitHub" },
    ]);
  });

  it("never yields an empty page — misconfig falls back to GitHub", () => {
    expect(parseProviders("facebook,,")).toEqual([
      { key: "github", label: "GitHub" },
    ]);
  });
});

describe("enabledProviders — runtime config precedence", () => {
  afterEach(() => {
    delete window.__DOCZ_CONFIG__;
    vi.unstubAllEnvs();
  });

  it("prefers the injected runtime config over the build-time env", () => {
    vi.stubEnv("VITE_AUTH_PROVIDERS", "github");
    window.__DOCZ_CONFIG__ = { authProviders: ["keycloak", "github"] };
    expect(enabledProviders().map((p) => p.key)).toEqual([
      "keycloak",
      "github",
    ]);
  });

  it("falls back to the build-time env when no runtime config is present", () => {
    vi.stubEnv("VITE_AUTH_PROVIDERS", "okta");
    expect(enabledProviders().map((p) => p.key)).toEqual(["okta"]);
  });

  it("ignores an empty runtime list and falls back", () => {
    vi.stubEnv("VITE_AUTH_PROVIDERS", "okta");
    window.__DOCZ_CONFIG__ = { authProviders: [] };
    expect(enabledProviders().map((p) => p.key)).toEqual(["okta"]);
  });

  it("still whitelists a hostile runtime list down to GitHub", () => {
    window.__DOCZ_CONFIG__ = { authProviders: ["facebook", "google"] };
    expect(enabledProviders()).toEqual([{ key: "github", label: "GitHub" }]);
  });
});

describe("last-used provider", () => {
  const three = parseProviders("github,okta,keycloak");

  it("round-trips through localStorage", () => {
    expect(lastUsedProvider()).toBeNull();
    rememberProvider("okta");
    expect(lastUsedProvider()).toBe("okta");
  });

  it("promotes the remembered provider to the primary slot", () => {
    expect(promoteLastUsed(three, "keycloak").map((p) => p.key)).toEqual([
      "keycloak",
      "github",
      "okta",
    ]);
  });

  it("leaves the order alone when nothing (or garbage) is remembered", () => {
    expect(promoteLastUsed(three, null)).toEqual(three);
    // e.g. a provider that has since been disabled
    expect(promoteLastUsed(three, "facebook")).toEqual(three);
  });
});
