/// <reference types="node" />
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { afterEach, beforeAll, describe, expect, it } from "vitest";

/*
 * pkg/theme/runtime.js mounted on a fragment (DESIGN-0026 §6): the same
 * runtime an exported page runs, inside one div.docz-page, with the URL's
 * hash left to the host. The fragment is pkg/export/html's golden.
 */

interface Mounted {
  showTab: (name: string, keepScroll?: boolean) => boolean;
  unmount: () => void;
}

interface Docz {
  mount: (
    el: Element,
    opts?: { hash?: boolean; tab?: string; onTab?: (t: string) => void },
  ) => Mounted;
}

// Read from disk: import.meta.url is not a file: URL under jsdom.
const repo = resolve(process.cwd(), "..");
const runtime = readFileSync(resolve(repo, "pkg/theme/runtime.js"), "utf8");
const fragment = readFileSync(
  resolve(repo, "pkg/export/html/testdata/golden/fragment.html"),
  "utf8",
);

function docz(): Docz {
  return (window as unknown as { docz: Docz }).docz;
}

function page(): HTMLElement {
  document.body.innerHTML = fragment;
  const el = document.querySelector<HTMLElement>(".docz-page");
  if (el === null) {
    throw new Error("the fragment has no .docz-page");
  }
  return el;
}

const selected = (el: HTMLElement) =>
  el
    .querySelector('nav.tabs button[aria-selected="true"]')
    ?.getAttribute("data-tab");

describe("docz.mount", () => {
  beforeAll(() => {
    // The site's <html> carries no data-doc, so loading the runtime must
    // not start it.
    // The runtime is pkg/theme's own file, read from disk.
    // eslint-disable-next-line @typescript-eslint/no-implied-eval
    const run = new Function(runtime) as () => void;
    run();
  });

  afterEach(() => {
    window.localStorage.clear();
    document.body.innerHTML = "";
    window.location.hash = "";
  });

  it("does not start itself on a page without data-doc", () => {
    expect(document.documentElement.classList.contains("js")).toBe(false);
    expect(typeof docz().mount).toBe("function");
  });

  it("opens the first tab, or the one asked for, and reports each", () => {
    const el = page();
    const seen: string[] = [];
    docz().mount(el, { hash: false, onTab: (t) => seen.push(t) });
    expect(el.classList.contains("js")).toBe(true);
    expect(selected(el)).toBe("overview");
    expect(el.querySelector("#p-overview")?.classList.contains("on")).toBe(
      true,
    );

    el.querySelector<HTMLButtonElement>('button[data-tab="phases"]')?.click();
    expect(selected(el)).toBe("phases");
    expect(el.querySelector("#p-overview")?.classList.contains("on")).toBe(
      false,
    );
    expect(seen).toEqual(["overview", "phases"]);

    const again = page();
    docz().mount(again, { hash: false, tab: "testing" });
    expect(selected(again)).toBe("testing");
  });

  it("leaves the hash alone when the host owns it", () => {
    window.location.hash = "#p-phases";
    const el = page();
    docz().mount(el, { hash: false });
    expect(selected(el)).toBe("overview");
    expect(window.location.hash).toBe("#p-phases");
  });

  it("runs the IMPL task filter inside the element", () => {
    const el = page();
    docz().mount(el, { hash: false });
    const panel = el.querySelector("#p-phases");
    expect(panel?.getAttribute("data-task-filter")).toBe("all");
    el.querySelector<HTMLButtonElement>('button[data-filter="open"]')?.click();
    expect(panel?.getAttribute("data-task-filter")).toBe("open");
    expect(window.localStorage.getItem("docz:IMPL-0004:filter")).toBe("open");
  });

  it("stops listening once unmounted", () => {
    const el = page();
    const m = docz().mount(el, { hash: false });
    m.unmount();
    el.querySelector<HTMLButtonElement>('button[data-tab="phases"]')?.click();
    expect(selected(el)).toBe("overview");
  });
});
