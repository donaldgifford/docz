import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Snippet } from "@/components/snippet";

function renderSnippet(snippet: string): HTMLElement {
  const { container } = render(<Snippet snippet={snippet} />);
  return container;
}

describe("Snippet", () => {
  it("re-emits <em> markers as <mark> and nothing else", () => {
    const container = renderSnippet("before <em>match</em> after");

    const marks = container.querySelectorAll("mark");
    expect(marks).toHaveLength(1);
    expect(marks[0]?.textContent).toBe("match");
    expect(container.textContent).toBe("before match after");
  });

  it("handles multiple markers", () => {
    const container = renderSnippet("<em>a</em> mid <em>b</em>");
    const marks = [...container.querySelectorAll("mark")];
    expect(marks.map((m) => m.textContent)).toEqual(["a", "b"]);
  });

  it("survives unbalanced markers without crashing", () => {
    const container = renderSnippet("</em></em>loose <em>open");
    expect(container.textContent).toBe("loose open");
    expect(container.querySelectorAll("mark")).toHaveLength(1);
  });

  describe("hostile snippets render inert", () => {
    it.each([
      ["<script>alert(1)</script>", "script"],
      ['<img src=x onerror="alert(1)">', "img"],
      ['<a href="javascript:alert(1)">click</a>', "a"],
      ["<iframe src=//evil.example></iframe>", "iframe"],
      ['<svg onload="alert(1)"></svg>', "svg"],
      ["<style>*{display:none}</style>", "style"],
      ["<mark onmouseover=alert(1)>fake</mark>", "*[onmouseover]"],
    ])("%s creates no live element", (payload, selector) => {
      const container = renderSnippet(`hit <em>x</em> ${payload}`);

      expect(container.querySelector(selector)).toBeNull();
      // The payload survives as visible literal text, not markup.
      expect(container.textContent).toContain(payload);
    });

    it("does not honor <em> variants carrying attributes", () => {
      const container = renderSnippet('<em onmouseover="alert(1)">styled</em>');

      expect(container.querySelector("em")).toBeNull();
      expect(container.querySelector("[onmouseover]")).toBeNull();
      // The bogus opener is literal text; the bare </em> closer is a
      // legitimate marker and is consumed.
      expect(container.textContent).toContain(
        '<em onmouseover="alert(1)">styled',
      );
    });

    it("keeps a forged </em> from eating following markup", () => {
      const container = renderSnippet(
        "</em><img src=x onerror=alert(1)><em>real</em>",
      );

      expect(container.querySelector("img")).toBeNull();
      expect(container.querySelectorAll("mark")).toHaveLength(1);
    });
  });
});
