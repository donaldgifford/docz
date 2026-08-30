import { describe, expect, it } from "vitest";

import { apiConfig } from "@/lib/apiConfig";

describe("apiConfig", () => {
  it("reads an enabled block with the .docz.yaml spellings", () => {
    expect(
      apiConfig({
        docs_dir: "docs",
        api: {
          enabled: true,
          landing_page: "docs/index.md",
          exclude: ["docs/scratch"],
          additional_docs: ["CONTRIBUTING.md", "DEVELOPMENT.md"],
        },
      }),
    ).toEqual({
      landingPage: "docs/index.md",
      additionalDocs: ["CONTRIBUTING.md", "DEVELOPMENT.md"],
    });
  });

  it("treats null lists as empty (the nil-slice wire gotcha)", () => {
    expect(
      apiConfig({
        api: {
          enabled: true,
          landing_page: "docs/index.md",
          exclude: null,
          additional_docs: null,
        },
      }),
    ).toEqual({ landingPage: "docs/index.md", additionalDocs: [] });
  });

  it("returns undefined for a disabled or absent block", () => {
    expect(
      apiConfig({ api: { enabled: false, landing_page: "docs/index.md" } }),
    ).toBeUndefined();
    expect(apiConfig({ docs_dir: "docs" })).toBeUndefined();
    expect(apiConfig({ api: {} })).toBeUndefined();
  });

  it("reads a stale capitalized snapshot as no pages surface", () => {
    // Pre-docz-v1.2.2 rows keep Go field names until the repo
    // re-ingests; the gate must degrade to today's behavior.
    expect(
      apiConfig({
        DocsDir: "docs",
        API: { Enabled: true, LandingPage: "docs/index.md" },
      }),
    ).toBeUndefined();
  });

  it("dies gracefully on wrong shapes", () => {
    expect(apiConfig(undefined)).toBeUndefined();
    expect(apiConfig(null)).toBeUndefined();
    expect(apiConfig("api")).toBeUndefined();
    expect(apiConfig({ api: "enabled" })).toBeUndefined();
    expect(apiConfig({ api: { enabled: "true" } })).toBeUndefined();
    // Enabled but hostile inner shapes: strings dropped, landing "".
    expect(
      apiConfig({
        api: { enabled: true, landing_page: 7, additional_docs: [1, "a.md"] },
      }),
    ).toEqual({ landingPage: "", additionalDocs: ["a.md"] });
  });
});
