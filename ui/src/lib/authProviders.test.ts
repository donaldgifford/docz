import { describe, expect, it } from "vitest";

import {
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
