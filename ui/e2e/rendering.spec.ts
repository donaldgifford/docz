/// <reference lib="dom" />
// ^ e2e runs under tsconfig.node.json, whose lib is ES2023 only. The
// `page.evaluate` callback below runs in the browser and needs
// `getComputedStyle`; this is the same per-file escape hatch the unit
// tests use for node APIs, rather than widening the shared config.
import { expect, test } from "@playwright/test";

/*
 * Rendering-pipeline journeys (IMPL-0002 Phases 2–4) against the MSW
 * preview build. The DESIGN-0777 fixture is served by a browser-worker
 * override behind the docz:e2e:rendering-doc flag (see
 * src/mocks/browser.ts) — it carries an alert, a captioned go fence,
 * and two mermaid diagrams whose node labels are hostile <img>
 * payloads, the second of which also tries to disable the protection
 * through its own front matter, so this is also where the REAL mermaid
 * strict-mode render gets its security assertion.
 */

/*
 * Chunks that must only ever load for a document containing a diagram.
 * ELK is matched separately because mermaid 12 ships it as its own ESM
 * chunk whose filename contains no "mermaid" — a pattern looking only
 * for that word would sail straight past an eagerly imported layout
 * engine, which is the single mistake these assertions exist to catch.
 */
const ELK_CHUNK = /\belk\b/i;
const DIAGRAM_CHUNK = /mermaid|\belk\b/i;

test("alerts, code chrome, and mermaid render on one doc", async ({ page }) => {
  await page.addInitScript(() => {
    sessionStorage.setItem("docz:e2e:rendering-doc", "1");
  });
  const diagramRequests: string[] = [];
  page.on("request", (request) => {
    if (DIAGRAM_CHUNK.test(request.url())) {
      diagramRequests.push(request.url());
    }
  });
  let dialogFired = false;
  page.on("dialog", (dialog) => {
    dialogFired = true;
    void dialog.dismiss();
  });

  await page.goto("/donaldgifford/docz-site/design/DESIGN-0777");
  await expect(
    page.getByRole("heading", {
      level: 1,
      name: "Rendering pipeline e2e fixture",
    }),
  ).toBeVisible();

  // Admonition with label, no leaked marker.
  await expect(page.locator(".admonition.warning .adm-label")).toHaveText(
    "Warning",
  );
  await expect(page.locator(".doc-prose")).not.toContainText("[!WARNING]");

  // Codeblock chrome: badge + caption.
  await expect(page.locator(".codeblock-header .lang")).toHaveText("go");
  await expect(page.locator(".codeblock-header .caption")).toHaveText(
    "internal/ingest/parse.go",
  );

  // Mermaid: the lazy chunk loads and both diagrams land as SVG.
  await expect(page.locator("figure.mermaid-figure svg")).toHaveCount(2, {
    timeout: 15_000,
  });
  await expect(page.locator("figure.mermaid-figure figcaption")).toHaveText([
    "fig 1 - order flow",
    "fig 2 - hostile front matter",
  ]);
  expect(diagramRequests.length).toBeGreaterThan(0);
  // ELK is the configured layout, so its chunk must actually be fetched
  // — which also keeps the diagram-free assertion below honest by
  // proving this pattern matches something real.
  expect(diagramRequests.filter((url) => ELK_CHUNK.test(url))).not.toHaveLength(
    0,
  );

  // strict + htmlLabels:false — the hostile node label stays literal
  // SVG text: no element (not even a purified <img src>) materializes
  // from document text, no foreignObject HTML islands, nothing runs.
  // Figure 2 additionally carries front matter trying to set
  // htmlLabels/securityLevel itself; `secure` keeps it inert, so these
  // same counts cover it.
  expect(await page.locator(".doc-prose img").count()).toBe(0);
  expect(await page.locator(".doc-prose foreignObject").count()).toBe(0);
  expect(await page.locator(".doc-prose script").count()).toBe(0);
  expect(dialogFired).toBe(false);
});

test("the diagram chunks stay off diagram-free docs", async ({ page }) => {
  const diagramRequests: string[] = [];
  page.on("request", (request) => {
    if (DIAGRAM_CHUNK.test(request.url())) {
      diagramRequests.push(request.url());
    }
  });

  await page.goto("/donaldgifford/docz-site/design/DESIGN-0001");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  // The reader (and Shiki) are fully loaded; mermaid and ELK never were.
  await expect(page.locator(".doc-prose pre").first()).toBeVisible();
  expect(diagramRequests).toHaveLength(0);
});

/*
 * The deployment layout override (IMPL-0006 Phase 5). `build:msw` bakes
 * no layout — the default is what e2e should exercise, and baking
 * `dagre` would quietly stop covering ELK — so the override is driven
 * through `window.__DOCZ_CONFIG__` directly. That is more faithful than
 * a build-time variable anyway: it is the exact channel
 * server/serve.ts injects on, so this covers the real path a deployment
 * takes rather than the fallback.
 *
 * The payload sets only `mermaidLayout`; `nav` and `authProviders` stay
 * absent so they keep falling back to the baked build-time values and
 * this test changes nothing else about the page.
 */
test("the dagre override renders without fetching ELK", async ({ page }) => {
  await page.addInitScript(() => {
    (window as unknown as Record<string, unknown>).__DOCZ_CONFIG__ = {
      mermaidLayout: "dagre",
    };
  });
  const diagramRequests: string[] = [];
  page.on("request", (request) => {
    if (DIAGRAM_CHUNK.test(request.url())) {
      diagramRequests.push(request.url());
    }
  });

  await page.goto("/donaldgifford/docz-site/pages/guides/markdown-specimen.md");
  await expect(page.locator("figure.mermaid-figure svg")).toHaveCount(3, {
    timeout: 20_000,
  });

  // mermaid still loads; ELK is never fetched. This is what makes the
  // 436 KB gzipped layout chunk a cost only ELK deployments pay.
  expect(diagramRequests.length).toBeGreaterThan(0);
  expect(diagramRequests.filter((url) => ELK_CHUNK.test(url))).toHaveLength(0);
});

/*
 * The specimen's Mermaid section carries all three diagram kinds this
 * pipeline renders, and Figure 2 is the one place the "monochrome
 * unless the document says otherwise" policy is exercised: a diagram's
 * own `classDef` has to outrank the stylesheet, which works only
 * because mermaid scopes those rules by render id. A layout or theme
 * change can break that without breaking anything that throws, so it is
 * asserted on computed style rather than left to the eye.
 */
test("the specimen's diagrams render with their own colors", async ({
  page,
}) => {
  await page.goto("/donaldgifford/docz-site/pages/guides/markdown-specimen.md");
  const figures = page.locator("figure.mermaid-figure");
  await expect(figures.locator("svg")).toHaveCount(3, { timeout: 20_000 });
  // The third fence takes no caption, so only two figcaptions exist.
  await expect(page.locator("figure.mermaid-figure figcaption")).toHaveText([
    "Figure 1: the sync pipeline",
    "Figure 2: coloring individual nodes",
  ]);

  const nodes = await figures.nth(2).evaluate((figure) =>
    // Array.from, not a spread: the e2e lib is ES2023 without
    // dom.iterable, so a NodeList has no iterator here.
    Array.from(figure.querySelectorAll("g.node"), (node) => {
      const shape = node.querySelector("rect, polygon, path, circle");
      return {
        className: node.getAttribute("class") ?? "",
        stroke: shape === null ? "" : getComputedStyle(shape).stroke,
      };
    }),
  );
  const strokeOf = (className: string): string | undefined =>
    nodes.find((node) => node.className.includes(className))?.stroke;

  // The three classDef colors from the document, verbatim.
  expect(strokeOf("begin")).toBe("rgb(158, 206, 106)");
  expect(strokeOf("decide")).toBe("rgb(224, 175, 104)");
  expect(strokeOf("work")).toBe("rgb(125, 207, 255)");

  // Everything the document did not color takes --color-border-strong
  // from the theme map. A flat rgb() rather than a url(#…-gradient) is
  // also what proves the neo look's node gradients stay off.
  const plain = nodes.filter(
    (node) => !/begin|decide|work/.test(node.className),
  );
  expect(plain.length).toBeGreaterThan(0);
  for (const node of plain) {
    expect(node.stroke).toBe("rgb(52, 64, 90)");
  }
});
