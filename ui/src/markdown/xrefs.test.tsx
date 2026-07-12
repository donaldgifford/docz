import { render } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { renderMarkdown } from "@/markdown/processor";

const XREFS = new Map([
  ["DESIGN-0002", "/acme/docs/design/DESIGN-0002"],
  ["RFC-0007", "/acme/docs/rfc/RFC-0007"],
]);

async function renderBody(md: string): Promise<HTMLElement> {
  const { content } = await renderMarkdown(md, { xrefs: XREFS });
  // MarkdownAnchor emits router Links for xrefs — give them a router.
  const { container } = render(<MemoryRouter>{content}</MemoryRouter>);
  return container;
}

function xrefAnchors(container: HTMLElement): HTMLAnchorElement[] {
  return [...container.querySelectorAll<HTMLAnchorElement>("a[data-xref]")];
}

describe("xref linking", () => {
  it("turns resolving doc-id tokens into router links", async () => {
    const container = await renderBody("See DESIGN-0002 for the contract.");

    const anchors = xrefAnchors(container);
    expect(anchors).toHaveLength(1);
    expect(anchors[0]?.getAttribute("href")).toBe(
      "/acme/docs/design/DESIGN-0002",
    );
    expect(anchors[0]?.textContent).toBe("DESIGN-0002");
    expect(container.textContent).toContain("See DESIGN-0002 for the");
  });

  it("resolves case-insensitively but keeps the written casing", async () => {
    const container = await renderBody("see design-0002.");

    const anchors = xrefAnchors(container);
    expect(anchors).toHaveLength(1);
    expect(anchors[0]?.textContent).toBe("design-0002");
  });

  it("links multiple tokens in one text node, preserving order", async () => {
    const container = await renderBody("DESIGN-0002 supersedes RFC-0007.");

    const anchors = xrefAnchors(container);
    expect(anchors.map((a) => a.textContent)).toEqual([
      "DESIGN-0002",
      "RFC-0007",
    ]);
    expect(container.textContent).toBe("DESIGN-0002 supersedes RFC-0007.");
  });

  it("leaves non-resolving ids as plain text", async () => {
    const container = await renderBody("RFC-9999 does not exist.");

    expect(xrefAnchors(container)).toHaveLength(0);
    expect(container.textContent).toContain("RFC-9999");
  });

  it("never links inside code spans or fenced blocks", async () => {
    const container = await renderBody(
      "Inline `DESIGN-0002` and:\n\n```text\nDESIGN-0002\n```\n",
    );

    expect(xrefAnchors(container)).toHaveLength(0);
  });

  it("never nests a link inside an author-written link", async () => {
    const container = await renderBody(
      "[DESIGN-0002](https://example.com/spec)",
    );

    const anchors = [...container.querySelectorAll("a")];
    expect(anchors).toHaveLength(1);
    expect(anchors[0]?.getAttribute("href")).toBe("https://example.com/spec");
    expect(anchors[0]?.hasAttribute("data-xref")).toBe(false);
  });

  it("renders untouched without a resolver", async () => {
    const { content } = await renderMarkdown("See DESIGN-0002.");
    const { container } = render(<MemoryRouter>{content}</MemoryRouter>);

    expect(xrefAnchors(container)).toHaveLength(0);
  });
});
