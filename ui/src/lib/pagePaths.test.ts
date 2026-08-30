import { describe, expect, it } from "vitest";

import { pageSourceKeys, pageSourcePath } from "@/lib/pagePaths";

/*
 * The reconstruction table (DESIGN-0004 Component 3): published path →
 * candidate source paths, inverting the upstream publish mapping with
 * docs_dir + additional_docs.
 */

const ADDITIONAL = ["README.md"];

describe("pageSourceKeys", () => {
  it.each([
    [
      "additional_docs member publishes at its repo-relative path",
      "README.md",
      ["README.md"],
    ],
    ["file page joins docs_dir", "input.md", ["docs/input.md"]],
    [
      "nested file page joins docs_dir",
      "guides/local-dev.md",
      ["docs/guides/local-dev.md"],
    ],
    [
      "uppercase extension still reads as a file page",
      "NOTES.MD",
      ["docs/NOTES.MD"],
    ],
    [
      "extensionless directory page yields both README and index keys",
      "design",
      ["docs/design/README.md", "docs/design/index.md"],
    ],
    [
      "nested directory page yields both keys",
      "guides/setup",
      ["docs/guides/setup/README.md", "docs/guides/setup/index.md"],
    ],
  ])("%s", (_name, publishedPath, expected) => {
    expect(pageSourceKeys(publishedPath, "docs", ADDITIONAL)).toEqual(expected);
  });

  it("prefers the additional_docs identity over the docs_dir join", () => {
    // A member ends .md too — membership must be checked first, or the
    // root README would wrongly reconstruct under docs_dir.
    expect(pageSourceKeys("README.md", "docs", ADDITIONAL)).toEqual([
      "README.md",
    ]);
    expect(pageSourceKeys("README.md", "docs", [])).toEqual(["docs/README.md"]);
  });
});

describe("pageSourcePath", () => {
  it("returns the canonical (first) source key", () => {
    expect(pageSourcePath("design", "docs", ADDITIONAL)).toBe(
      "docs/design/README.md",
    );
    expect(pageSourcePath("guides/local-dev.md", "docs", ADDITIONAL)).toBe(
      "docs/guides/local-dev.md",
    );
    expect(pageSourcePath("README.md", "docs", ADDITIONAL)).toBe("README.md");
  });
});
