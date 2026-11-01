import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

import { SHELL_MODES, useShellMode } from "./appearances";

/*
 * Full-rule axe in a real browser — including color-contrast, which
 * the jsdom sweep (src/a11y/axe.test.tsx) cannot compute. Same bar:
 * zero serious/critical violations. Every view runs under every theme
 * and, on a theme with modes, both of them (IMPL-0030 Phase 3).
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

for (const appearance of SHELL_MODES) {
  test.describe(appearance.label, () => {
    test("directory passes full-rule axe", async ({ page }) => {
      await useShellMode(page, appearance);
      await page.goto("/");
      await expect(
        page.getByText("docz-site: cross-repo docz reader and search UI"),
      ).toBeVisible();
      await expectNoBlockingViolations(page);
    });

    test("reader passes full-rule axe", async ({ page }) => {
      await useShellMode(page, appearance);
      await page.goto("/donaldgifford/docz-site/design/DESIGN-0001");
      await expect(
        page.getByRole("heading", {
          level: 1,
          name: "docz-site: cross-repo docz reader and search UI",
        }),
      ).toBeVisible();
      await expectNoBlockingViolations(page);
    });

    test("markdown specimen page passes full-rule axe", async ({ page }) => {
      await useShellMode(page, appearance);
      // docs/guides/markdown-specimen.md renders every pipeline construct
      // on one page; this is the only sweep that sees real mermaid output
      // and real color-contrast for all of them.
      await page.goto(
        "/donaldgifford/docz-site/pages/guides/markdown-specimen.md",
      );
      await expect(
        page.getByRole("heading", {
          level: 1,
          name: "Markdown rendering specimen",
        }),
      ).toBeVisible();
      await expect(page.locator("figure.mermaid-figure svg")).toHaveCount(3, {
        timeout: 15_000,
      });
      await expectNoBlockingViolations(page);
    });

    test("repos and type page pass full-rule axe", async ({ page }) => {
      await useShellMode(page, appearance);
      await page.goto("/repos");
      await expect(
        page.getByRole("heading", { name: "Repositories" }),
      ).toBeVisible();
      await expectNoBlockingViolations(page);

      await page.goto("/donaldgifford/docz-site/design");
      await expect(page.getByRole("table")).toBeVisible();
      await expectNoBlockingViolations(page);
    });
  });
}
