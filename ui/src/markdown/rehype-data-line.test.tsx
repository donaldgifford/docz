import { render } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { renderMarkdown } from "@/markdown/processor";

// mermaid can't render in jsdom; the fallback <pre> is what carries the
// stamp here, the figure carries it once a diagram renders.
vi.mock("mermaid", () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn(() => Promise.reject(new Error("jsdom"))),
  },
}));

async function reviewDom(md: string, review = true) {
  const { content } = await renderMarkdown(md, { review });
  const { container } = render(<MemoryRouter>{content}</MemoryRouter>);
  return container;
}

// The line a block's stamp names, read back from the DOM.
function lineOf(container: HTMLElement, selector: string): string | null {
  const el = container.querySelector(selector);
  if (el === null) {
    throw new Error(`no element for ${selector}`);
  }
  return el.getAttribute("data-line");
}

const DOC = [
  "---", // 1
  "id: DESIGN-0001",
  "status: Draft",
  "---",
  "", // 5
  "# Title",
  "",
  "<!--toc:start-->",
  "- [Goals](#goals)",
  "<!--toc:end-->", // 10
  "",
  "## Goals",
  "",
  "Plain paragraph.",
  "", // 15
  "- one",
  "  - nested",
  "",
  "> [!NOTE]",
  "> inside the alert", // 20
  "",
  "| a | b |",
  "| - | - |",
  "| 1 | 2 |",
  "", // 25
  "```go",
  "package main",
  "```",
  "",
  "```mermaid", // 30
  "graph TD; A-->B",
  "```",
  "",
  '<div data-line="99">',
  "", // 35
  "raw html block",
  "",
  "</div>",
].join("\n");

describe("rehypeDataLine in review mode", () => {
  it("stamps every block with its document line", async () => {
    const c = await reviewDom(DOC);
    expect(lineOf(c, "h1")).toBe("6");
    expect(lineOf(c, "h2")).toBe("12");
    expect(lineOf(c, "p")).toBe("14");
  });

  it("stamps nested list items on their own lines", async () => {
    const c = await reviewDom(DOC);
    const items = c.querySelectorAll("li");
    expect(items[0]?.getAttribute("data-line")).toBe("16");
    expect(items[1]?.getAttribute("data-line")).toBe("17");
    expect(lineOf(c, "ul ul")).toBe("17");
  });

  it("stamps a block inside an alert", async () => {
    const c = await reviewDom(DOC);
    expect(lineOf(c, ".admonition")).toBe("19");
    const para = Array.from(c.querySelectorAll(".admonition p")).find((p) =>
      p.textContent.includes("inside the alert"),
    );
    expect(para?.getAttribute("data-line")).toBe("19");
  });

  it("keeps the stamp on a table inside its wrapper", async () => {
    const c = await reviewDom(DOC);
    expect(c.querySelector(".table-wrap > table")).not.toBeNull();
    expect(lineOf(c, "table")).toBe("22");
    expect(lineOf(c, "tbody tr")).toBe("24");
  });

  it("keeps the stamp through Shiki and on a Mermaid block", async () => {
    const c = await reviewDom(DOC);
    expect(lineOf(c, 'pre[data-language="go"]')).toBe("26");
    expect(lineOf(c, "pre[data-mermaid-fallback]")).toBe("30");
  });

  it("replaces a forged data-line on raw HTML with the real one", async () => {
    const c = await reviewDom(DOC);
    const raw = Array.from(c.querySelectorAll("div")).find((d) =>
      d.textContent.trim().startsWith("raw html block"),
    );
    expect(raw?.getAttribute("data-line")).toBe("34");
    expect(c.querySelector('[data-line="99"]')).toBeNull();
  });

  it("strips a forged data-line outside review mode", async () => {
    const c = await reviewDom('<p data-line="7">forged</p>\n\nplain\n', false);
    expect(c.querySelector("[data-line]")).toBeNull();
  });

  it("renders neither frontmatter nor the ToC block", async () => {
    const c = await reviewDom(DOC);
    expect(c.textContent).not.toContain("DESIGN-0001");
    expect(c.querySelector('a[href="#goals"]')).toBeNull();
  });
});
