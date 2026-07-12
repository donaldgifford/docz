import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

/*
 * Full-rule axe in a real browser — including color-contrast, which
 * the jsdom sweep (src/a11y/axe.test.tsx) cannot compute. Same bar:
 * zero serious/critical violations.
 */

async function expectNoBlockingViolations(page: Page): Promise<void> {
  const results = await new AxeBuilder({ page }).analyze();
  const blocking = results.violations.filter(
    (violation) =>
      violation.impact === "serious" || violation.impact === "critical",
  );
  expect(
    blocking.map((violation) => ({
      id: violation.id,
      impact: violation.impact,
      nodes: violation.nodes.map((node) => node.html),
    })),
  ).toEqual([]);
}

test("directory passes full-rule axe", async ({ page }) => {
  await page.goto("/");
  await expect(
    page.getByText("docz-site: cross-repo docz reader and search UI"),
  ).toBeVisible();
  await expectNoBlockingViolations(page);
});

test("reader passes full-rule axe", async ({ page }) => {
  await page.goto("/donaldgifford/docz-site/design/DESIGN-0001");
  await expect(
    page.getByRole("heading", {
      level: 1,
      name: "docz-site: cross-repo docz reader and search UI",
    }),
  ).toBeVisible();
  await expectNoBlockingViolations(page);
});

test("repos and type page pass full-rule axe", async ({ page }) => {
  await page.goto("/repos");
  await expect(
    page.getByRole("heading", { name: "Repositories" }),
  ).toBeVisible();
  await expectNoBlockingViolations(page);

  await page.goto("/donaldgifford/docz-site/design");
  await expect(page.getByRole("table")).toBeVisible();
  await expectNoBlockingViolations(page);
});
