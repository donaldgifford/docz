/*
 * The XSS gate for search snippets (DESIGN-0001 "Snippets are
 * untrusted"; DESIGN-0028 §7). A snippet is a string from docz-api with
 * its matches between literal <em> and </em>, cut from a document's
 * body, so it can hold anything a document can. docz draws it through
 * the design system's Marked, in the palette and on the directory, and
 * this is docz's own check on that part: the two markers become <mark>
 * and every other character stays text, whatever the string holds. The
 * payloads are the ones the document pane's gate runs
 * (doc-pane.xss.test.tsx). If a change to the part breaks this suite,
 * the change is wrong, not the test.
 */
import { Marked } from "@donaldgifford/design-system/blocks/marked";
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import payloadList from "@/mocks/content/xss-payloads.json";

// [name, markdown] pairs, shared with the document pane's gate.
const PAYLOADS = payloadList as [name: string, payload: string][];

function renderSnippet(snippet: string): HTMLElement {
  const { container } = render(<Marked text={snippet} />);
  return container;
}

describe("a search snippet", () => {
  it("re-emits <em> markers as <mark> and nothing else", () => {
    const container = renderSnippet("before <em>match</em> after");

    const marks = container.querySelectorAll("mark");
    expect(marks).toHaveLength(1);
    expect(marks[0]?.textContent).toBe("match");
    expect(container.textContent).toBe("before match after");
  });

  it("survives unbalanced markers without crashing", () => {
    const container = renderSnippet("</em></em>loose <em>open");
    expect(container.textContent).toBe("loose open");
    expect(container.querySelectorAll("mark")).toHaveLength(1);
  });

  describe("renders every hostile payload as text", () => {
    it.each(PAYLOADS)("%s", (_, payload) => {
      const container = renderSnippet(`hit <em>x</em> ${payload}`);

      // The one element is the mark around the match; the payload made
      // none, in or out of it.
      const elements = [...container.querySelectorAll("*")];
      expect(elements.map((element) => element.tagName)).toEqual(["MARK"]);
      expect(elements[0]?.getAttributeNames().sort()).toEqual([
        "class",
        "data-slot",
      ]);
      // It survives as the characters it is.
      expect(container.textContent).toBe(`hit x ${payload}`);
    });

    it.each(PAYLOADS)("%s, inside the match", (_, payload) => {
      const container = renderSnippet(`<em>${payload}</em>`);

      expect(container.querySelectorAll("mark *")).toHaveLength(0);
      expect(container.querySelectorAll(":not(mark)")).toHaveLength(0);
    });
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
