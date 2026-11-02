import { expect, test } from "@playwright/test";

import { SHELL_MODES, useShellMode } from "./appearances";

/*
 * The reader's document pane in a browser (DESIGN-0026 §7, IMPL-0032
 * Phase 4): each document is its docz-html fragment, rendered by
 * docz-api's own renderer (fragments.json), with pkg/theme's runtime
 * mounted inside it. Runs once per shell mode; the pane follows the
 * shell's mode, since the runtime reads data-mode from <html>.
 */

const SITE = "/donaldgifford/docz-site";
const API = "/donaldgifford/docz-api";

for (const shell of SHELL_MODES) {
  test.describe(shell.label, () => {
    test.beforeEach(async ({ page }) => {
      await useShellMode(page, shell);
    });

    test("tabs move ?tab=, and the URL opens the tab it names", async ({
      page,
    }) => {
      await page.goto(`${API}/design/DESIGN-0001`);
      const doc = page.locator(".docz-page.js");
      await expect(doc).toBeVisible();
      await expect(page.locator("html")).toHaveAttribute(
        "data-mode",
        shell.mode,
      );
      // The tab the runtime opens on mount is not written to the URL.
      expect(new URL(page.url()).search).toBe("");

      await doc.getByRole("tab", { name: /Testing/ }).click();
      await expect(page).toHaveURL(/\?tab=testing$/);
      await expect(doc.locator("#p-testing")).toBeVisible();
      await expect(doc.locator("#p-overview")).toBeHidden();

      await page.goto(`${API}/design/DESIGN-0001?tab=rollout`);
      await expect(
        page.locator(".docz-page.js").getByRole("tab", { selected: true }),
      ).toHaveAttribute("data-tab", "rollout");
    });

    test("an open question opens and closes", async ({ page }) => {
      await page.goto(`${API}/design/DESIGN-0001?tab=open-questions`);
      const question = page.locator(".docz-page.js details.expand").first();
      await expect(question).toHaveAttribute("open", "");
      await question.locator("summary").click();
      await expect(question).not.toHaveAttribute("open");
      await expect(question.locator("li.option.recommended")).toBeHidden();
    });

    test("the IMPL filter hides the tasks it leaves out", async ({ page }) => {
      await page.goto(`${SITE}/impl/IMPL-0001?tab=phases`);
      const panel = page.locator(".docz-page.js #p-phases");
      const done = panel.locator('ul.tasks li[data-state="done"]').first();
      await expect(done).toBeVisible();

      await panel.getByRole("button", { name: "Open" }).click();
      await expect(panel).toHaveAttribute("data-task-filter", "open");
      await expect(done).toBeHidden();

      await panel.getByRole("button", { name: "All" }).click();
      await expect(done).toBeVisible();
    });

    test("a widget draws and a diagram renders with the site's mermaid", async ({
      page,
    }) => {
      await page.goto(`${SITE}/runbook/RUNBOOK-0001?tab=procedures`);
      const panel = page.locator(".docz-page.js #p-procedures");
      const flow = panel.locator('figure.widget[data-widget="flow"]');
      await expect(flow).toHaveClass(/\bdrawn\b/);
      await expect(flow.locator(".widget-mount")).toContainText("Restart");
      await expect(panel.locator("figure.mermaid-figure svg")).toHaveCount(1, {
        timeout: 15_000,
      });
      expect(await panel.locator("foreignObject").count()).toBe(0);
    });

    test("a server-resolved site link routes in-app", async ({ page }) => {
      await page.goto(`${API}/rfc/RFC-0001?tab=references`);
      // A full navigation would reload the page and drop this marker.
      await page.evaluate(() => {
        (window as unknown as { marker: boolean }).marker = true;
      });
      await page
        .locator(
          '.docz-page.js a[href="/donaldgifford/docz-api/design/DESIGN-0002"]',
        )
        .first()
        .click();
      await expect(page).toHaveURL(`${API}/design/DESIGN-0002`);
      await expect(page.locator(".docz-page.js")).toHaveAttribute(
        "data-doc",
        "DESIGN-0002",
      );
      expect(
        await page.evaluate(
          () => (window as unknown as { marker?: boolean }).marker,
        ),
      ).toBe(true);
    });
  });
}
