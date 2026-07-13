import { expect, test } from "@playwright/test";

/*
 * Rendering-pipeline journeys (IMPL-0002 Phases 2–4) against the MSW
 * preview build. The DESIGN-0777 fixture is served by a browser-worker
 * override behind the docz:e2e:rendering-doc flag (see
 * src/mocks/browser.ts) — it carries an alert, a captioned go fence,
 * and a mermaid diagram whose node label is a hostile <img> payload,
 * so this is also where the REAL mermaid strict-mode render gets its
 * security assertion.
 */

test("alerts, code chrome, and mermaid render on one doc", async ({ page }) => {
  await page.addInitScript(() => {
    sessionStorage.setItem("docz:e2e:rendering-doc", "1");
  });
  const mermaidRequests: string[] = [];
  page.on("request", (request) => {
    if (/mermaid/i.test(request.url())) {
      mermaidRequests.push(request.url());
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

  // Mermaid: the lazy chunk loads and the diagram lands as SVG.
  await expect(page.locator("figure.mermaid-figure svg")).toBeVisible({
    timeout: 15_000,
  });
  await expect(page.locator("figure.mermaid-figure figcaption")).toHaveText(
    "fig 1 - order flow",
  );
  expect(mermaidRequests.length).toBeGreaterThan(0);

  // strict + htmlLabels:false — the hostile node label stays literal
  // SVG text: no element (not even a purified <img src>) materializes
  // from document text, no foreignObject HTML islands, nothing runs.
  expect(await page.locator(".doc-prose img").count()).toBe(0);
  expect(await page.locator(".doc-prose foreignObject").count()).toBe(0);
  expect(await page.locator(".doc-prose script").count()).toBe(0);
  expect(dialogFired).toBe(false);
});

test("the mermaid chunk stays off diagram-free docs", async ({ page }) => {
  const mermaidRequests: string[] = [];
  page.on("request", (request) => {
    if (/mermaid/i.test(request.url())) {
      mermaidRequests.push(request.url());
    }
  });

  await page.goto("/donaldgifford/docz-site/design/DESIGN-0001");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  // The reader (and Shiki) are fully loaded; mermaid never was.
  await expect(page.locator(".doc-prose pre").first()).toBeVisible();
  expect(mermaidRequests).toHaveLength(0);
});
