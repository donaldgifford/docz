import { expect, type Page, test } from "@playwright/test";

/*
 * docz-flow, docz-stepper, and docz-scorecard in the reader and on the
 * review page (IMPL-0030 Phase 5). The reader's blocks are the markdown
 * specimen's Widgets section; the review page's are DESIGN-0004 in the
 * review fixture repository. Each valid block draws as a figure a reader
 * can drive; the invalid one shows its errors, then its YAML.
 */

// The lazy half of a widget block: the validator, the YAML parser, and
// the three components (src/markdown/widget-block.tsx).
const WIDGET_CHUNK = "widget-view";

const REVIEW = "http://127.0.0.1:4174/review.html?t=dev-review-token";

async function driveWidgets(page: Page) {
  const flow = page.locator('figure.widget[data-widget="flow"]');
  const stepper = page.locator('figure.widget[data-widget="stepper"]');
  const scorecard = page.locator('figure.widget[data-widget="scorecard"]');
  await expect(flow).toHaveCount(1);
  await expect(stepper).toHaveCount(1);
  await expect(scorecard).toHaveCount(1);

  // A flow stage shows what it does when selected.
  const stage = flow.getByRole("listitem").nth(1).getByRole("button");
  await stage.click();
  await expect(stage).toHaveAttribute("aria-pressed", "true");
  await expect(flow.locator("[aria-live]")).not.toContainText(
    "Select a stage.",
  );

  // The failing scenario marks its step; a step past it is never reached.
  await stepper
    .getByRole("group", { name: "Scenario" })
    .getByRole("button")
    .nth(1)
    .click();
  await stepper.getByRole("button", { name: "Next" }).click();
  await stepper.getByRole("button", { name: "Next" }).click();
  await expect(stepper).toContainText("Never reached");

  // The example scores; pasted JSON that is not JSON says so.
  await scorecard.getByRole("button", { name: "Load the example" }).click();
  await expect(
    scorecard.getByRole("progressbar", { name: "Score" }),
  ).toBeVisible();
  await expect(scorecard).toContainText("example, not real results");
  await scorecard.getByRole("button", { name: "Clear" }).click();
  await scorecard.getByLabel("Results as JSON").fill("{");
  await scorecard.getByRole("button", { name: "Score" }).click();
  await expect(scorecard).toContainText("Not valid JSON");
}

async function expectInvalidFlow(page: Page) {
  const invalid = page.locator(".widget-invalid");
  await expect(invalid).toHaveCount(1);
  await expect(invalid.locator(".adm-label")).toHaveText(
    "Invalid docz-flow block",
  );
  await expect(invalid.locator("li").first()).toContainText(/line \d+/);
  await expect(invalid).toContainText("colour");
  await expect(invalid.locator("pre code")).toContainText("colour:");
}

test("the reader draws each widget and shows an invalid block's errors", async ({
  page,
}) => {
  const chunks: string[] = [];
  page.on("request", (r) => {
    if (r.url().includes(WIDGET_CHUNK)) {
      chunks.push(r.url());
    }
  });

  // A document with no widget never loads the chunk.
  await page.goto("/donaldgifford/docz-site/design/DESIGN-0001");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await expect(page.locator("article")).toBeVisible();
  expect(chunks).toEqual([]);

  await page.goto("/donaldgifford/docz-site/pages/guides/markdown-specimen.md");
  await expect(
    page.getByRole("heading", {
      level: 1,
      name: "Markdown rendering specimen",
    }),
  ).toBeVisible();
  await driveWidgets(page);
  await expectInvalidFlow(page);
  expect(chunks.length).toBeGreaterThan(0);
});

test("the review page draws each widget at its document line", async ({
  page,
}) => {
  await page.goto(REVIEW);
  await page
    .getByRole("navigation", { name: "Documents" })
    .getByRole("button", { name: /^DESIGN-0004/ })
    .click();
  await expect(
    page.getByRole("heading", { level: 1, name: "Widget blocks", exact: true }),
  ).toBeVisible();
  await driveWidgets(page);
  await expectInvalidFlow(page);

  // The review render's line stamps survive the widget, so an
  // annotation on one still has a coordinate.
  for (const sel of [
    'figure.widget[data-widget="flow"]',
    'figure.widget[data-widget="stepper"]',
    'figure.widget[data-widget="scorecard"]',
    ".widget-invalid",
  ]) {
    await expect(page.locator(sel)).toHaveAttribute("data-line", /^\d+$/);
  }
});
