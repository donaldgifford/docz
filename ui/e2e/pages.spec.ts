import { expect, test } from "@playwright/test";

/*
 * Published-pages journey (IMPL-0005 Phase 3, DESIGN-0004): the
 * sidebar's Pages tree opens a nested file page, the body renders
 * through the one sanitizing pipeline (admonition + code chrome from
 * the guide fixture), and the mermaid chunk never loads for a
 * diagram-free page.
 */

test("the sidebar's pages tree opens a rendered page", async ({ page }) => {
  const mermaidRequests: string[] = [];
  page.on("request", (request) => {
    if (/mermaid/i.test(request.url())) {
      mermaidRequests.push(request.url());
    }
  });

  await page.goto("/donaldgifford/docz-site");
  const sidebar = page.getByRole("navigation", { name: "docz navigation" });
  await expect(sidebar.getByText("Pages", { exact: true })).toBeVisible();

  // guides/ has no page of its own — its row opens without
  // navigating, revealing the leaf.
  await sidebar.getByRole("button", { name: "guides/" }).click();
  await expect(page).toHaveURL("/donaldgifford/docz-site");

  await sidebar
    .getByRole("link", { name: "Local development against a real docz-api" })
    .click();
  await expect(page).toHaveURL(
    "/donaldgifford/docz-site/pages/guides/local-dev.md",
  );

  // The page renders through the reader pipeline: h1 kept, admonition
  // labeled with no leaked marker, code fence with chrome.
  await expect(
    page.getByRole("heading", {
      level: 1,
      name: "Local development against a real docz-api",
    }),
  ).toBeVisible();
  await expect(page.locator(".admonition.note .adm-label")).toHaveText("Note");
  await expect(page.locator(".doc-prose")).not.toContainText("[!NOTE]");
  await expect(page.locator(".codeblock-header .lang")).toHaveText("sh");

  // Source meta: reconstructed repo path + short sha.
  await expect(page.getByTestId("page-meta")).toContainText(
    "docs/guides/local-dev.md",
  );

  // Diagram-free page — the ~700 KB mermaid chunk stays off.
  expect(mermaidRequests).toHaveLength(0);
});
