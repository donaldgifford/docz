import { render, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { renderMarkdown } from "@/markdown/processor";

// mermaid can't render in jsdom (no SVG measurement); a rejecting mock
// pins MermaidBlock to its fallback so routing stays deterministic.
vi.mock("mermaid", () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn(() => Promise.reject(new Error("jsdom"))),
  },
}));

async function renderToDom(md: string) {
  const { content, toc } = await renderMarkdown(md);
  const { container } = render(<>{content}</>);
  return { container, toc };
}

describe("renderMarkdown", () => {
  it("slugs headings and collects an h2–h4 ToC", async () => {
    const { container, toc } = await renderToDom(
      [
        "# Title",
        "## First Section",
        "### Nested Detail",
        "#### Deep Point",
        "##### Too Deep",
        "## Second Section",
      ].join("\n\n"),
    );

    expect(toc).toEqual([
      { depth: 2, text: "First Section", id: "first-section" },
      { depth: 3, text: "Nested Detail", id: "nested-detail" },
      { depth: 4, text: "Deep Point", id: "deep-point" },
      { depth: 2, text: "Second Section", id: "second-section" },
    ]);
    expect(container.querySelector("h2#first-section")).not.toBeNull();
    // h1 gets an id from slug but stays out of the ToC.
    expect(container.querySelector("h1#title")).not.toBeNull();
  });

  it("highlights fenced code in a bundled grammar", async () => {
    const { container } = await renderToDom(
      '```go\nfunc main() { fmt.Println("hi") }\n```',
    );

    const pre = container.querySelector("pre.shiki");
    expect(pre).not.toBeNull();
    expect(pre?.querySelectorAll("span[style]").length).toBeGreaterThan(0);
  });

  it("makes code blocks keyboard-scrollable named regions", async () => {
    // axe scrollable-region-focusable: overflow depends on rendered
    // font metrics, so every block gets the treatment up front.
    const { container } = await renderToDom("```text\nwide code\n```");

    const pre = container.querySelector("pre");
    expect(pre?.getAttribute("tabindex")).toBe("0");
    expect(pre?.getAttribute("role")).toBe("region");
    expect(pre?.getAttribute("aria-label")).toBe("code block");
  });

  it("wraps highlighted blocks in codeblock chrome with a caption", async () => {
    const { container } = await renderToDom(
      "```go internal/ingest/parse.go\nfunc main() {}\n```",
    );

    const block = container.querySelector(".codeblock");
    expect(block).not.toBeNull();
    const header = block?.querySelector(".codeblock-header");
    expect(header?.querySelector(".lang")?.textContent).toBe("go");
    expect(header?.querySelector(".caption")?.textContent).toBe(
      "internal/ingest/parse.go",
    );
    const pre = block?.querySelector("pre.shiki");
    expect(pre).not.toBeNull();
    expect(pre?.getAttribute("aria-label")).toBe("go code block");
  });

  it("renders a lang-only header when the fence has no meta", async () => {
    const { container } = await renderToDom("```yaml\nkey: value\n```");

    expect(
      container.querySelector(".codeblock-header .lang")?.textContent,
    ).toBe("yaml");
    expect(container.querySelector(".codeblock-header .caption")).toBeNull();
  });

  it("leaves plain and unknown-language fences bare", async () => {
    const { container } = await renderToDom(
      "```\nplain text\n```\n\n```klingon\nqapla\n```",
    );

    expect(container.querySelector(".codeblock")).toBeNull();
    expect(container.querySelectorAll("pre")).toHaveLength(2);
    expect(container.querySelector("pre")?.getAttribute("aria-label")).toBe(
      "code block",
    );
  });

  it("routes mermaid fences to MermaidBlock, not the codeblock chrome", async () => {
    const { container } = await renderToDom(
      "```mermaid fig 1\nflowchart TD\n  A --> B\n```",
    );

    // Settle on the (mocked-to-fail) fallback before asserting.
    await waitFor(() => {
      expect(
        container.querySelector('[data-mermaid-fallback="failed"]'),
      ).not.toBeNull();
    });
    expect(container.querySelector(".codeblock")).toBeNull();
    expect(container.querySelector("pre.shiki")).toBeNull();
    const fallback = container.querySelector("pre[data-mermaid-fallback]");
    expect(fallback?.textContent).toContain("A --> B");
    expect(fallback?.getAttribute("aria-label")).toBe("mermaid diagram source");
  });

  it("falls back to plain text for unknown languages", async () => {
    const { container } = await renderToDom(
      "```klingon\nqaStaHvIS wa' ram\n```",
    );

    expect(container.querySelector("pre")?.textContent).toContain(
      "qaStaHvIS wa' ram",
    );
  });

  it.each([
    ["NOTE", "note", "Note"],
    ["TIP", "tip", "Tip"],
    ["IMPORTANT", "important", "Important"],
    ["WARNING", "warning", "Warning"],
    ["CAUTION", "caution", "Caution"],
  ])("renders [!%s] alerts as %s admonitions", async (marker, kind, label) => {
    const { container } = await renderToDom(
      `> [!${marker}]\n> Handle with care.`,
    );

    const box = container.querySelector(`div.admonition.${kind}`);
    expect(box).not.toBeNull();
    expect(box?.querySelector("span.adm-label")?.textContent).toBe(label);
    expect(box?.textContent).toContain("Handle with care.");
    expect(box?.textContent).not.toContain(`[!${marker}]`);
    // The blockquote itself was rewritten, not nested.
    expect(container.querySelector("blockquote")).toBeNull();
  });

  it("keeps multi-paragraph alert bodies with nested markdown", async () => {
    const { container } = await renderToDom(
      [
        "> [!WARNING]",
        "> First paragraph with **bold** and [a link](https://example.com).",
        ">",
        "> Second paragraph.",
      ].join("\n"),
    );

    const box = container.querySelector("div.admonition.warning");
    expect(box?.querySelectorAll("p")).toHaveLength(2);
    expect(box?.querySelector("strong")?.textContent).toBe("bold");
    expect(box?.querySelector("a")?.getAttribute("href")).toBe(
      "https://example.com",
    );
  });

  it("leaves plain blockquotes and mid-text markers alone", async () => {
    const { container } = await renderToDom(
      [
        "> An ordinary pull quote.",
        "",
        "> Some text before [!NOTE] stays literal.",
        "",
        "```text",
        "> [!CAUTION]",
        "```",
      ].join("\n"),
    );

    expect(container.querySelector(".admonition")).toBeNull();
    expect(container.querySelectorAll("blockquote")).toHaveLength(2);
    expect(container.textContent).toContain("[!NOTE]");
    expect(container.querySelector("pre")?.textContent).toContain("[!CAUTION]");
  });

  it("renders GFM tables", async () => {
    const { container } = await renderToDom(
      ["| a | b |", "| - | - |", "| 1 | 2 |"].join("\n"),
    );

    expect(container.querySelector("table")).not.toBeNull();
    expect(container.querySelectorAll("td")).toHaveLength(2);
  });

  it("keeps sanitizer-approved raw HTML", async () => {
    const { container } = await renderToDom(
      "<details><summary>More</summary>\n\nhidden body\n\n</details>",
    );

    expect(container.querySelector("details > summary")?.textContent).toBe(
      "More",
    );
  });

  it("round-trips GFM footnotes with working anchors", async () => {
    const { container } = await renderToDom(
      "A claim.[^1]\n\n[^1]: The evidence.",
    );

    const ref = container.querySelector('a[href^="#"]');
    expect(ref).not.toBeNull();
    const target = ref?.getAttribute("href")?.slice(1) ?? "";
    expect(container.querySelector(`[id="${target}"]`)).not.toBeNull();
  });
});
