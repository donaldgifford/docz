import { render } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { renderMarkdown } from "@/markdown/processor";

async function renderToDom(md: string) {
  const { content } = await renderMarkdown(md);
  return render(<MemoryRouter>{content}</MemoryRouter>).container;
}

describe("table wrapping", () => {
  it("wraps every GFM table in a scroll container", async () => {
    const container = await renderToDom(
      [
        "| a | b |",
        "| - | - |",
        "| 1 | 2 |",
        "",
        "text",
        "",
        "| c |",
        "| - |",
        "| 3 |",
      ].join("\n"),
    );
    const wraps = container.querySelectorAll("div.table-wrap");
    expect(wraps).toHaveLength(2);
    for (const wrap of wraps) {
      expect(wrap.children).toHaveLength(1);
      expect(wrap.firstElementChild?.tagName).toBe("TABLE");
      // Focusable named region: the scroll is reachable by keyboard.
      expect(wrap.getAttribute("tabindex")).toBe("0");
      expect(wrap.getAttribute("role")).toBe("region");
      expect(wrap.getAttribute("aria-label")).toBe("table");
    }
    expect(container.querySelectorAll("table")).toHaveLength(2);
  });

  it("cannot be forged from document HTML", async () => {
    const container = await renderToDom(
      '<div class="table-wrap"><p>not a table</p></div>\n\n<div class="table-wrap">\n\n| a |\n| - |\n| 1 |\n\n</div>',
    );
    // The authored class is stripped by the schema; the real table still
    // gets exactly one generated wrapper.
    expect(container.querySelectorAll("div.table-wrap")).toHaveLength(1);
    expect(container.querySelector("div.table-wrap > table")).not.toBeNull();
    expect(container.querySelector("div.table-wrap div.table-wrap")).toBeNull();
  });
});
