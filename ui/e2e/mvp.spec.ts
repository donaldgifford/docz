import { expect, test } from "@playwright/test";

/*
 * MVP journeys (IMPL-0001 Phase 4) against the MSW-enabled preview
 * build — the same curated demo-org fixtures the unit tests use.
 */

const SITE_DESIGN_TITLE = "docz-site: cross-repo docz reader and search UI";
const SITE_IMPL_TITLE =
  "docz-site MVP: phased build of the reader, directory, and repo pages";
const API_DESIGN_TITLE =
  "docz-api cross-repo docz registry and ingestion service";

test("directory → filter → open doc", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText(SITE_DESIGN_TITLE)).toBeVisible();

  // Filter by type chip: non-impl rows disappear.
  await page.getByRole("button", { name: "impl", exact: true }).click();
  await expect(page.getByText(SITE_DESIGN_TITLE)).toBeHidden();
  await expect(page).toHaveURL(/\?type=impl/);

  // Open the surviving row in the reader.
  await page.getByText(SITE_IMPL_TITLE).click();
  await expect(
    page.getByRole("heading", { level: 1, name: SITE_IMPL_TITLE }),
  ).toBeVisible();
  await expect(page).toHaveURL(/\/donaldgifford\/docz-site\/impl\/IMPL-0001/);
});

test("palette search → open doc", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText(SITE_DESIGN_TITLE)).toBeVisible();

  await page.keyboard.press("/");
  const dialog = page.getByRole("dialog", { name: "Search documents" });
  await expect(dialog).toBeVisible();

  await dialog
    .getByPlaceholder("search docs, rfcs, authors…")
    .fill("ingestion service");
  // Wait for the FILTERED result set (the unfiltered list also contains
  // the docz-api title, so waiting on the title alone races the query).
  await expect(
    dialog.getByText("donaldgifford/docz-api — 1 match"),
  ).toBeVisible();
  await page.keyboard.press("Enter");

  await expect(
    page.getByRole("heading", { level: 1, name: API_DESIGN_TITLE }),
  ).toBeVisible();
  await expect(page).toHaveURL(
    /\/donaldgifford\/docz-api\/design\/DESIGN-0001/,
  );
});

test("cold deep-link into the reader", async ({ page }) => {
  await page.goto("/donaldgifford/docz-site/design/DESIGN-0001");
  await expect(
    page.getByRole("heading", { level: 1, name: SITE_DESIGN_TITLE }),
  ).toBeVisible();
  // Reader chrome came with it: breadcrumbs and the repo nav.
  await expect(
    page.getByRole("navigation", { name: "Breadcrumb" }),
  ).toBeVisible();
});

test("unknown doc renders the neutral 404 panel", async ({ page }) => {
  await page.goto("/donaldgifford/docz-site/design/DESIGN-9999");
  await expect(
    page.getByText("Not found — or not visible to you"),
  ).toBeVisible();
});

test("401 renders the bare session panel", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText(SITE_DESIGN_TITLE)).toBeVisible();

  // Flip the fixture override (see src/mocks/browser.ts) and reload.
  await page.evaluate(() => {
    sessionStorage.setItem("docz:e2e:force-401", "1");
  });
  await page.reload();

  await expect(page.getByText("Session required")).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Sign in with GitHub" }),
  ).toHaveAttribute("href", "/auth/login?provider=github");
});
