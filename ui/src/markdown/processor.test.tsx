import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { renderMarkdown } from "@/markdown/processor";

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

  it("falls back to plain text for unknown languages", async () => {
    const { container } = await renderToDom(
      "```klingon\nqaStaHvIS wa' ram\n```",
    );

    expect(container.querySelector("pre")?.textContent).toContain(
      "qaStaHvIS wa' ram",
    );
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
