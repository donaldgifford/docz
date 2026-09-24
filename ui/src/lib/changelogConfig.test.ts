import { describe, expect, it } from "vitest";

import { changelogBasename, changelogConfig } from "@/lib/changelogConfig";

/*
 * The config_snapshot is untyped repo-owner YAML round-tripped to JSON
 * — every wrong shape must read as "no changelog", because the RepoNav
 * row and the changelog route both gate on this.
 */
describe("changelogConfig", () => {
  it("returns the configured file when the block is enabled", () => {
    expect(
      changelogConfig({
        docs_dir: "docs",
        changelog: { enabled: true, file: "charts/site/CHANGELOG.md" },
      }),
    ).toEqual({ file: "charts/site/CHANGELOG.md" });
  });

  it("defaults the file to CHANGELOG.md when unset or blank", () => {
    expect(changelogConfig({ changelog: { enabled: true } })).toEqual({
      file: "CHANGELOG.md",
    });
    expect(
      changelogConfig({ changelog: { enabled: true, file: "  " } }),
    ).toEqual({ file: "CHANGELOG.md" });
  });

  it("gates on enabled === true exactly", () => {
    expect(changelogConfig({ changelog: { enabled: false } })).toBeUndefined();
    expect(changelogConfig({ changelog: { enabled: "true" } })).toBeUndefined();
    expect(changelogConfig({ changelog: { enabled: 1 } })).toBeUndefined();
    expect(
      changelogConfig({ changelog: { file: "CHANGELOG.md" } }),
    ).toBeUndefined();
  });

  it("reads every malformed snapshot shape as no changelog", () => {
    expect(changelogConfig(undefined)).toBeUndefined();
    expect(changelogConfig(null)).toBeUndefined();
    expect(changelogConfig("changelog")).toBeUndefined();
    expect(changelogConfig(42)).toBeUndefined();
    expect(changelogConfig({})).toBeUndefined();
    expect(changelogConfig({ changelog: "yes" })).toBeUndefined();
    expect(changelogConfig({ changelog: null })).toBeUndefined();
  });
});

describe("changelogBasename", () => {
  it("returns the last path segment", () => {
    expect(changelogBasename("CHANGELOG.md")).toBe("CHANGELOG.md");
    expect(changelogBasename("charts/docz-site/CHANGELOG.md")).toBe(
      "CHANGELOG.md",
    );
  });
});
