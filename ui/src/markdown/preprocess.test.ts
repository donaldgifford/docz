import { describe, expect, it } from "vitest";

import { preprocessDoczMarkdown } from "@/markdown/preprocess";

describe("preprocessDoczMarkdown", () => {
  it("strips a leading YAML frontmatter block", () => {
    const raw = [
      "---",
      "id: DESIGN-0001",
      'title: "docz-site: reader"',
      "status: Draft",
      "---",
      "",
      "# Heading",
    ].join("\n");

    expect(preprocessDoczMarkdown(raw)).toBe("\n# Heading");
  });

  it("only treats frontmatter at the very start as frontmatter", () => {
    const raw = "# Heading\n\n---\nid: not-frontmatter\n---\n";
    expect(preprocessDoczMarkdown(raw)).toBe(raw);
  });

  it("leaves thematic breaks alone when there is no closing fence", () => {
    const raw = "---\n\nJust a horizontal rule opener, no yaml fence close";
    expect(preprocessDoczMarkdown(raw)).toBe(raw);
  });

  it("strips the docz toc marker block and its contents", () => {
    const raw = [
      "# Title",
      "",
      "<!--toc:start-->",
      "- [Section](#section)",
      "  - [Nested](#nested)",
      "<!--toc:end-->",
      "",
      "Body text.",
    ].join("\n");

    expect(preprocessDoczMarkdown(raw)).toBe("# Title\n\n\nBody text.");
  });

  it("strips frontmatter and toc together (real docz shape)", () => {
    const raw = [
      "---",
      "id: IMPL-0001",
      "status: Draft",
      "---",
      "",
      "# IMPL 0001",
      "",
      "<!--toc:start-->",
      "- [Phase 0](#phase-0)",
      "<!--toc:end-->",
      "",
      "## Phase 0",
    ].join("\n");

    expect(preprocessDoczMarkdown(raw)).toBe("\n# IMPL 0001\n\n\n## Phase 0");
  });

  it("handles CRLF line endings", () => {
    const raw = "---\r\nid: X\r\n---\r\nBody";
    expect(preprocessDoczMarkdown(raw)).toBe("Body");
  });

  it("is the identity for markdown without either block", () => {
    const raw = "# Plain\n\nSome **markdown** with a [link](https://x).\n";
    expect(preprocessDoczMarkdown(raw)).toBe(raw);
  });

  it("strips the leading h1 only when asked", () => {
    const raw = "---\nid: X\n---\n\n# DESIGN 0001: title\n\nBody.";
    expect(preprocessDoczMarkdown(raw)).toContain("# DESIGN 0001");
    expect(preprocessDoczMarkdown(raw, { stripLeadingH1: true })).toBe(
      "\n\nBody.",
    );
  });

  it("does not strip an h1 that isn't the opening content", () => {
    const raw = "Intro paragraph.\n\n# Late Heading\n";
    expect(preprocessDoczMarkdown(raw, { stripLeadingH1: true })).toBe(raw);
  });

  it("strips the leading h1 past markdownlint pragma comments", () => {
    const raw = [
      "---",
      "id: DESIGN-0001",
      "---",
      "<!-- markdownlint-disable-file MD025 MD041 -->",
      "",
      "# DESIGN 0001: real docz shape",
      "",
      "Body.",
    ].join("\n");

    const out = preprocessDoczMarkdown(raw, { stripLeadingH1: true });
    expect(out).not.toContain("# DESIGN 0001");
    expect(out).toContain("markdownlint-disable-file");
    expect(out).toContain("Body.");
  });

  it("handles dashes inside and between leading comments", () => {
    const raw = "<!-- a - b -- c --->\n<!----->\n# Title\nBody.";

    const out = preprocessDoczMarkdown(raw, { stripLeadingH1: true });
    expect(out).not.toContain("# Title");
    expect(out).toContain("a - b -- c");
    expect(out).toContain("Body.");
  });

  it("survives a comment-bomb without backtracking (js/redos)", () => {
    // "<!--" + "--><!--"×N with no h1 made the old lazy comment body
    // backtrack exponentially — this input would hang the suite if the
    // regex regressed. raw_md is untrusted; linear or bust.
    const raw = "<!--" + "--><!--".repeat(10_000);

    const out = preprocessDoczMarkdown(raw, { stripLeadingH1: true });
    expect(out).toBe(raw); // no h1 → nothing stripped
  });
});
