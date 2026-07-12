/*
 * The XSS gate for the reader pipeline. Every payload below must come
 * out neutralized — no executable element, no event handler, no
 * scriptable URL — while the benign suite proves sanitization doesn't
 * maim real docz markdown. This suite gates CI; if a schema or
 * pipeline change breaks it, the change is wrong, not the test.
 */
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { renderMarkdown } from "@/markdown/processor";

async function renderToDom(md: string): Promise<HTMLElement> {
  const { content } = await renderMarkdown(md);
  const { container } = render(<>{content}</>);
  return container;
}

const FORBIDDEN_ELEMENTS =
  "script, iframe, object, embed, svg, math, style, form, link, meta, base";

const URL_ATTRIBUTES = ["href", "src", "xlink:href", "action", "formaction"];

function assertNeutralized(container: HTMLElement): void {
  expect(container.querySelectorAll(FORBIDDEN_ELEMENTS)).toHaveLength(0);

  for (const el of container.querySelectorAll("*")) {
    for (const attr of el.attributes) {
      expect(attr.name, `${el.tagName} carries ${attr.name}`).not.toMatch(
        /^on/i,
      );
      expect(attr.name).not.toBe("style");
      if (URL_ATTRIBUTES.includes(attr.name)) {
        expect(
          attr.value.trim().toLowerCase(),
          `${el.tagName} ${attr.name} keeps a scriptable URL`,
        ).not.toMatch(/^(javascript|data|vbscript):/);
      }
    }
  }
}

describe("XSS payloads are neutralized", () => {
  it.each([
    ["inline script", "<script>alert(1)</script>"],
    ["remote script", '<script src="https://evil.example/x.js"></script>'],
    ["img onerror", '<img src="x" onerror="alert(1)">'],
    ["javascript: markdown link", "[click me](javascript:alert(1))"],
    [
      "javascript: markdown link, entity-encoded",
      "[click me](javascript&#58;alert(1))",
    ],
    ["javascript: raw anchor", '<a href="javascript:alert(1)">x</a>'],
    ["javascript: mixed case", '<a href="JaVaScRiPt:alert(1)">x</a>'],
    ["javascript: with whitespace", '<a href=" \tjavascript:alert(1)">x</a>'],
    ["onclick handler", '<div onclick="alert(1)">content</div>'],
    [
      "onmouseover on a benign link",
      '<a href="https://ok.example" onmouseover="alert(1)">x</a>',
    ],
    ["iframe", '<iframe src="https://evil.example"></iframe>'],
    ["object", '<object data="https://evil.example/x.swf"></object>'],
    ["embed", '<embed src="https://evil.example/x.swf">'],
    ["svg onload", "<svg onload=alert(1)><circle r=1 /></svg>"],
    [
      "svg foreignObject smuggling",
      "<svg><foreignObject><script>alert(1)</script></foreignObject></svg>",
    ],
    [
      "mathml xlink",
      '<math><mi xlink:href="javascript:alert(1)">x</mi></math>',
    ],
    ["data: markdown link", "[x](data:text/html,<script>alert(1)</script>)"],
    [
      "data: image",
      '<img src="data:image/svg+xml;base64,PHN2Zy9vbmxvYWQ9YWxlcnQoMSk+">',
    ],
    ["vbscript: anchor", '<a href="vbscript:msgbox(1)">x</a>'],
    [
      "style attribute payload",
      "<div style=\"background:url('javascript:alert(1)')\">x</div>",
    ],
    [
      "form with formaction",
      '<form action="https://evil.example"><button formaction="javascript:alert(1)">go</button></form>',
    ],
    [
      "payload nested in benign markdown",
      '# Title\n\nSome text.\n\n<img src=x onerror=alert(1)>\n\n- list\n- items\n\n<a href="javascript:alert(1)">deep link</a>\n',
    ],
  ])("neutralizes %s", async (_name, payload) => {
    assertNeutralized(await renderToDom(payload));
  });
});

describe("benign markdown survives sanitization", () => {
  it("keeps GFM tables", async () => {
    const container = await renderToDom(
      ["| col a | col b |", "| ----- | ----- |", "| 1     | 2     |"].join(
        "\n",
      ),
    );
    expect(container.querySelector("table")).not.toBeNull();
    expect(container.querySelectorAll("td")).toHaveLength(2);
  });

  it("keeps fenced code with highlighting", async () => {
    const container = await renderToDom("```yaml\nkey: value\n```");
    expect(container.querySelector("pre.shiki code")).not.toBeNull();
    expect(container.textContent).toContain("key: value");
  });

  it("keeps footnotes with working anchors", async () => {
    const container = await renderToDom("Claim.[^n]\n\n[^n]: Evidence.");
    const ref = container.querySelector('a[href^="#"]');
    const target = ref?.getAttribute("href")?.slice(1) ?? "";
    expect(container.querySelector(`[id="${target}"]`)).not.toBeNull();
  });

  it("keeps https images with alt text", async () => {
    const container = await renderToDom(
      "![diagram](https://example.com/d.png)",
    );
    const img = container.querySelector("img");
    expect(img?.getAttribute("src")).toBe("https://example.com/d.png");
    expect(img?.getAttribute("alt")).toBe("diagram");
  });

  it("keeps blockquotes, task lists, and strikethrough", async () => {
    const container = await renderToDom(
      ["> quoted wisdom", "", "- [x] done", "- [ ] todo", "", "~~gone~~"].join(
        "\n",
      ),
    );
    expect(container.querySelector("blockquote")).not.toBeNull();
    expect(container.querySelectorAll('input[type="checkbox"]')).toHaveLength(
      2,
    );
    expect(container.querySelector("del")).not.toBeNull();
  });

  it("keeps https links", async () => {
    const container = await renderToDom("[docs](https://example.com/docs)");
    expect(container.querySelector("a")?.getAttribute("href")).toBe(
      "https://example.com/docs",
    );
  });
});

describe("slug stability", () => {
  const doc = [
    "# Doc Title",
    "## Getting Started",
    "### Install & Run",
    "## Getting Started",
    "## über section",
  ].join("\n\n");

  it("produces deterministic, repeat-safe ids", async () => {
    const first = await renderMarkdown(doc);
    const second = await renderMarkdown(doc);

    expect(first.toc).toEqual(second.toc);
    expect(first.toc.map((entry) => entry.id)).toEqual([
      "getting-started",
      "install--run",
      "getting-started-1",
      "über-section",
    ]);
  });
});
