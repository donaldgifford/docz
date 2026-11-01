/*
 * The XSS gate for the fragment path (DESIGN-0026 §7, IMPL-0032 Phase 4).
 * The pane injects what docz-api sends, so the gate runs over what docz-api
 * really sends: hostile-fragments.json is each payload in
 * mocks/content/xss-payloads.json rendered as a document by docz-api's own
 * renderFragment (TestUIHostileFragments), mounted here in the pane with
 * pkg/theme's runtime. Nothing executable may survive: no script-capable
 * element, no event handler, no scriptable URL, no style attribute, and
 * the pane routes only same-origin paths in-app. If a renderer change
 * breaks this suite, the change is wrong, not the test.
 */
import { render, waitFor } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { DocPane } from "@/components/doc-pane";
import hostile from "@/mocks/__generated__/hostile-fragments.json";
import payloadList from "@/mocks/content/xss-payloads.json";

const pending = vi.hoisted(() => () => new Promise<never>(() => undefined));
vi.mock("mermaid", () => ({
  default: { initialize: vi.fn(), render: vi.fn(pending) },
}));

const FRAGMENTS: Readonly<Record<string, string | undefined>> = hostile;
// [name, markdown] pairs, shared with TestUIHostileFragments.
const NAMES = (payloadList as [string, string][]).map(([name]) => name);

const FORBIDDEN =
  "script, iframe, object, embed, svg, foreignObject, math, style, form, link, meta, base";

const URL_ATTRIBUTES = ["href", "src", "xlink:href", "action", "formaction"];

function mountPane(html: string) {
  const router = createMemoryRouter(
    [{ path: "*", element: <DocPane html={html} /> }],
    { initialEntries: ["/acme/hostile/rfc/RFC-0001"] },
  );
  const { container } = render(<RouterProvider router={router} />);
  return { container, router };
}

function assertNeutralized(container: HTMLElement): void {
  // A widget's data island is the one script element a fragment may
  // carry: type application/json, never run.
  const forbidden = [...container.querySelectorAll(FORBIDDEN)].filter(
    (el) =>
      !(
        el.tagName === "SCRIPT" &&
        el.getAttribute("type") === "application/json" &&
        el.hasAttribute("data-widget")
      ),
  );
  expect(forbidden.map((el) => el.outerHTML)).toEqual([]);

  for (const el of container.querySelectorAll("*")) {
    for (const attr of el.attributes) {
      expect(attr.name, `${el.tagName} carries ${attr.name}`).not.toMatch(
        /^on/i,
      );
      expect(attr.name, `${el.tagName} carries a style`).not.toBe("style");
      if (URL_ATTRIBUTES.includes(attr.name)) {
        const value = attr.value.trim().toLowerCase();
        expect(value, `${el.tagName} ${attr.name}`).not.toMatch(
          /^(javascript|vbscript):/,
        );
        if (value.startsWith("data:")) {
          // The renderer inlines images as data:image/ only.
          expect(el.tagName).toBe("IMG");
          expect(value).toMatch(/^data:image\//);
        }
      }
    }
  }
}

describe("hostile fragments stay inert in the pane", () => {
  it("has a fragment for every payload", () => {
    expect(Object.keys(FRAGMENTS).sort()).toEqual([...NAMES].sort());
  });

  it.each(NAMES)("neutralizes %s", async (name) => {
    const html = FRAGMENTS[name];
    if (html === undefined) {
      throw new Error(`no fragment for ${name}`);
    }
    const { container } = mountPane(html);
    await waitFor(() => {
      expect(container.querySelector(".docz-page.js")).not.toBeNull();
    });
    assertNeutralized(container);
  });
});

describe("the pane routes only same-origin paths", () => {
  it.each([
    ["protocol-relative", "//evil.example/x"],
    ["absolute", "https://evil.example/x"],
    ["new tab", "/acme/hostile/rfc/RFC-0002"],
  ])("leaves a %s link to the browser", (kind, href) => {
    const target = kind === "new tab" ? ' target="_blank"' : "";
    const { container, router } = mountPane(
      `<div class="docz-page" data-doc="RFC-0001"><a href="${href}"${target}>x</a></div>`,
    );
    const anchor = container.querySelector("a");
    if (anchor === null) {
      throw new Error("no anchor");
    }
    // jsdom does not navigate; a click the pane did not take keeps the
    // router where it was and is not prevented.
    const event = new MouseEvent("click", { bubbles: true, cancelable: true });
    anchor.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(false);
    expect(router.state.location.pathname).toBe("/acme/hostile/rfc/RFC-0001");
  });
});
