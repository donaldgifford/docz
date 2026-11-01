import { execFileSync } from "node:child_process";
import {
  appendFileSync,
  cpSync,
  existsSync,
  mkdtempSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";

import { expect, type Page, test } from "@playwright/test";

/*
 * docz export html end to end (IMPL-0028 Phase 5): a built docz exports
 * the parity fixtures, and chromium opens each page from file://, with no
 * server. Every tab, the mode toggle, the IMPL task filter, an open
 * question, and each widget are driven, and a page may log no error and
 * make no request. Needs `just build` (or DOCZ_EXPORT_DOCZ).
 */

declare global {
  interface Window {
    docz: { mode: () => string };
  }
}

const DOCZ = resolve(process.env.DOCZ_EXPORT_DOCZ ?? "../build/bin/docz");
const FIXTURES = resolve("../test/parity/fixtures");

const WIDGETS = `---
id: DESIGN-0099
title: Widgets
status: Draft
author: e2e
created: 2026-03-04
---

# DESIGN-0099: Widgets

## Overview

\`\`\`docz-flow
title: Request path
gates: [2]
stages:
  - {name: Ingress, does: Terminates TLS}
  - {name: Auth, does: Verifies the token}
\`\`\`

\`\`\`docz-stepper
steps:
  - {key: parse, name: Parse, does: Reads it}
  - {key: verify, name: Verify, does: Checks it}
scenarios:
  - name: Valid
  - {name: Expired, fails_at: verify}
\`\`\`

\`\`\`docz-scorecard
checks:
  - {id: C1, name: Signed, target: All signed}
\`\`\`
`;

/**
 * Exports a copy of a parity fixture and returns its output directory.
 * `config` is appended to the copy's .docz.yaml.
 */
function exportFixture(
  name: string,
  extra?: Record<string, string>,
  config?: string,
): string {
  const root = mkdtempSync(join(tmpdir(), `e2e-export-${name}-`));
  cpSync(join(FIXTURES, name), root, { recursive: true });
  for (const [path, body] of Object.entries(extra ?? {})) {
    writeFileSync(join(root, path), body);
  }
  if (config !== undefined) {
    appendFileSync(join(root, ".docz.yaml"), config);
  }
  const out = join(root, "out");
  execFileSync(DOCZ, ["export", "html", "--repo-root", root, "--out", out], {
    stdio: "pipe",
  });
  return out;
}

/** Opens a page from disk, recording console errors and requests. */
async function open(
  page: Page,
  file: string,
): Promise<{ errors: string[]; requests: string[] }> {
  const errors: string[] = [];
  const requests: string[] = [];
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(m.text());
  });
  page.on("pageerror", (e) => errors.push(String(e)));
  page.on("request", (r) => {
    const url = r.url();
    if (!url.startsWith("file:") && !url.startsWith("data:")) {
      requests.push(url);
    }
  });
  await page.goto(pathToFileURL(file).href);
  return { errors, requests };
}

/** Clicks every tab and checks its panel is the one shown. */
async function clickEveryTab(page: Page): Promise<void> {
  const tabs = await page
    .locator("nav.tabs button")
    .evaluateAll((bs) => bs.map((b) => b.getAttribute("data-tab") ?? ""));
  expect(tabs.length).toBeGreaterThan(0);
  for (const tab of tabs) {
    await page.click(`nav.tabs button[data-tab="${tab}"]`);
    await expect(page.locator(`#p-${tab}`)).toBeVisible();
  }
}

/** Toggles the mode and checks the page follows. */
async function toggleMode(page: Page): Promise<void> {
  const before = await page.evaluate(() => window.docz.mode());
  await page.click(".mode-toggle");
  const after = await page.evaluate(() => window.docz.mode());
  expect(after).not.toBe(before);
}

test.beforeAll(() => {
  if (!existsSync(DOCZ)) {
    throw new Error(
      `no docz at ${DOCZ}: run just build or set DOCZ_EXPORT_DOCZ`,
    );
  }
});

const pages: { fixture: string; file: string }[] = [
  { fixture: "rfc", file: "rfc/0001-switch-the-ingest-queue-to-nats.html" },
  { fixture: "adr", file: "index.html" },
  { fixture: "design", file: "design/0002-wiki-nav-generation.html" },
  { fixture: "impl", file: "impl/0001-phase-out-the-legacy-scanner.html" },
  { fixture: "investigation", file: "index.html" },
  { fixture: "custom", file: "index.html" },
];

for (const { fixture, file } of pages) {
  test(`${fixture}: ${file} opens offline and every tab works`, async ({
    page,
  }) => {
    const out = exportFixture(fixture);
    const { errors, requests } = await open(page, join(out, file));
    await clickEveryTab(page);
    await toggleMode(page);
    expect(errors).toEqual([]);
    expect(requests).toEqual([]);
  });
}

test("each type's first page opens offline", async ({ page }) => {
  for (const fixture of ["adr", "investigation", "custom"]) {
    const out = exportFixture(fixture);
    const { errors, requests } = await open(page, join(out, "index.html"));
    const first = await page.locator("table a").first().getAttribute("href");
    expect(first).toBeTruthy();
    await page.goto(pathToFileURL(join(out, first ?? "")).href);
    await clickEveryTab(page);
    expect(errors).toEqual([]);
    expect(requests).toEqual([]);
  }
});

test("the IMPL task filter hides what it should", async ({ page }) => {
  const out = exportFixture("impl");
  const { errors } = await open(
    page,
    join(out, "impl/0002-registry-store-migration.html"),
  );
  await page.click('nav.tabs button[data-tab="phases"]');
  const open_ = page.locator('#p-phases li[data-state="open"]').first();
  const done = page.locator('#p-phases li[data-state="done"]').first();
  await page.click('button[data-filter="done"]');
  await expect(open_).toBeHidden();
  await expect(done).toBeVisible();
  await page.click('button[data-filter="open"]');
  await expect(open_).toBeVisible();
  await expect(done).toBeHidden();
  await page.click('button[data-filter="all"]');
  await expect(done).toBeVisible();
  expect(errors).toEqual([]);
});

test("an open question expands", async ({ page }) => {
  const out = exportFixture("design");
  const { errors } = await open(
    page,
    join(out, "design/0002-wiki-nav-generation.html"),
  );
  await page.click('nav.tabs button[data-tab="open-questions"]');
  const question = page.locator("#p-open-questions details.expand").first();
  const wasOpen = await question.evaluate(
    (d) => (d as HTMLDetailsElement).open,
  );
  await question.locator("summary").click();
  expect(await question.evaluate((d) => (d as HTMLDetailsElement).open)).toBe(
    !wasOpen,
  );
  expect(errors).toEqual([]);
});

test("each widget draws", async ({ page }) => {
  const out = exportFixture("design", {
    "docs/design/0099-widgets.md": WIDGETS,
  });
  const { errors, requests } = await open(
    page,
    join(out, "design/0099-widgets.html"),
  );
  for (const name of ["flow", "stepper", "scorecard"]) {
    const fig = page.locator(`figure.widget[data-widget="${name}"]`);
    await expect(fig).toHaveClass(/\bdrawn\b/);
    await expect(fig.locator(".widget-fallback")).toBeHidden();
    await expect(fig.locator(".widget-mount > *").first()).toBeVisible();
  }
  await toggleMode(page);
  await expect(page.locator("figure.widget.drawn")).toHaveCount(3);
  expect(errors).toEqual([]);
  expect(requests).toEqual([]);
});

test("the repository's html.theme picks the theme with no --theme", async ({
  page,
}) => {
  const out = exportFixture("design", {}, "\nhtml:\n  theme: classic\n");
  const { errors, requests } = await open(
    page,
    join(out, "design/0002-wiki-nav-generation.html"),
  );
  await expect(page.locator("html")).toHaveAttribute("data-theme", "classic");
  const bg = await page.evaluate(() =>
    getComputedStyle(document.documentElement)
      .getPropertyValue("--color-bg-base")
      .trim(),
  );
  expect(bg).toBe("#0c1017");
  expect(errors).toEqual([]);
  expect(requests).toEqual([]);
});
