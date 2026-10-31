import { expect, test, type Page } from "@playwright/test";

/*
 * docz-review's page in a real browser (DESIGN-0024 section 7: the
 * proof that anchors survive formatting, wraps, Unicode, repetition,
 * and block boundaries), against the fake server the review dev server
 * runs over e2e/fixtures/review-session.json. The fake keeps state, so
 * the specs run serially and reset it first.
 */

const ORIGIN = "http://127.0.0.1:4174";
const TOKEN = "dev-review-token";

test.describe.configure({ mode: "serial" });

test.beforeEach(async ({ request }) => {
  await request.post(`${ORIGIN}/__fake/reset`);
});

async function open(page: Page) {
  await page.goto(`${ORIGIN}/review.html?t=${TOKEN}`);
  await expect(
    page.getByRole("heading", { name: "Review cache", level: 1 }),
  ).toBeVisible();
}

// Selects from the first occurrence of `from` (the `nth` one) through the
// end of `to` in the rendered document, then releases the mouse, which is
// what opens the popover.
async function select(page: Page, from: string, to = from, nth = 0) {
  await page.evaluate(
    ([start, end, n]) => {
      const article = document.querySelector("article");
      if (article === null) {
        throw new Error("no article");
      }
      const walker = document.createTreeWalker(article, NodeFilter.SHOW_TEXT);
      const nodes: Text[] = [];
      for (let t = walker.nextNode(); t !== null; t = walker.nextNode()) {
        nodes.push(t as Text);
      }
      const all = nodes.map((t) => t.data).join("");
      let at = -1;
      for (let i = 0; i <= n; i += 1) {
        at = all.indexOf(start, at + 1);
      }
      const stop = all.indexOf(end, at) + end.length;
      if (at === -1 || stop < at) {
        throw new Error(`cannot select ${start}…${end}`);
      }
      const range = document.createRange();
      let acc = 0;
      for (const t of nodes) {
        const next = acc + t.data.length;
        if (at >= acc && at < next) {
          range.setStart(t, at - acc);
        }
        if (stop > acc && stop <= next) {
          range.setEnd(t, stop - acc);
          break;
        }
        acc = next;
      }
      // A person selects what is on screen: bring it there first.
      range.startContainer.parentElement?.scrollIntoView({ block: "center" });
      const sel = document.getSelection();
      sel?.removeAllRanges();
      sel?.addRange(range);
      document.dispatchEvent(new MouseEvent("mouseup", { bubbles: true }));
    },
    [from, to, nth] as const,
  );
  return page.getByRole("dialog", { name: "Annotate selection" });
}

async function comment(page: Page, text: string) {
  const popover = page.getByRole("dialog", { name: "Annotate selection" });
  await popover.getByRole("button", { name: "Comment" }).click();
  await popover.getByLabel("Comment").fill(text);
  await popover.getByRole("button", { name: "Add" }).click();
  await expect(popover).toBeHidden();
}

test("answers, annotates, submits, and the message carries it all", async ({
  page,
  request,
}) => {
  await open(page);

  const q1 = page.locator('[data-question="1"]');
  await q1.getByRole("radio", { name: /An in-process LRU/ }).click();
  await page
    .getByRole("button", { name: "Accept all recommendations" })
    .click();
  await expect(
    page
      .locator('[data-question="2"]')
      .getByRole("radio", { name: /One hour/ }),
  ).toHaveAttribute("aria-checked", "true");
  await expect(
    q1.getByRole("radio", { name: /An in-process LRU/ }),
  ).toHaveAttribute("aria-checked", "true");

  // Formatted: the quote starts inside a <strong>.
  await select(page, "Postgres for the store");
  await comment(page, "Why not SQLite?");

  // Multiline: across the soft wrap in the overview.
  await select(page, "We use", "Postgres");
  await comment(page, "Wrapped.");

  // Unicode: combining-free accents and two emoji.
  const uni = await select(page, "café notes ☕");
  await uni.getByRole("button", { name: "Replace" }).click();
  await uni.getByLabel("Replace with").fill("coffee notes");
  await uni.getByRole("button", { name: "Add" }).click();

  // Repeated: the second "The cache is warm." in the same paragraph.
  const rep = await select(page, "The cache is warm.", "The cache is warm.", 1);
  await rep.getByRole("button", { name: "Delete" }).click();

  // Cross-block: from a goal into the non-goal list; comment only.
  const cross = await select(page, "under 50 ms.", "Caching search");
  await expect(cross.getByRole("button", { name: "Replace" })).toBeDisabled();
  await expect(cross.getByRole("button", { name: "Delete" })).toBeDisabled();
  await comment(page, "Across the lists.");

  await expect(
    page.getByRole("button", { name: "Approve", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "Submit" }).click();
  await expect(page.getByText(/This round is closed: changes/)).toBeVisible();

  const message = await (await request.get(`${ORIGIN}/__fake/feedback`)).text();
  expect(message).toContain(
    "docz review: DESIGN-0001 Review cache, round 1: changes requested",
  );
  expect(message).toContain("- Q1 Which cache? → c");
  expect(message).toContain("- Q2 How long a TTL? → a");
  expect(message).toContain(
    '[overview › Overview, line 39] "Postgres for the store"\n   comment: Why not SQLite?',
  );
  expect(message).toContain('[overview › Overview, line 39] "We use Postgres"');
  expect(message).toContain(
    '[goals › Goals, line 53] replace "café notes ☕"\n   → "coffee notes"',
  );
  expect(message).toContain(
    '[overview › Overview, line 43] delete: "The cache is warm."',
  );
  expect(message).toContain('[goals › Goals, line 52] "under 50 ms."');
  expect(message).toContain("Resolve Q1 and Q2 in the document");
  expect(message).not.toContain("quote not found");
  expect(message).not.toContain("quote ambiguous");
});

test("a comment typed but not added is never lost", async ({ page }) => {
  await open(page);
  const submit = page.getByRole("button", { name: "Submit" });
  const approve = page.getByRole("button", { name: "Approve", exact: true });
  // An empty draft has nothing to submit; approving is its own button.
  await expect(submit).toBeDisabled();
  await expect(approve).toBeEnabled();

  const popover = await select(page, "Postgres for the store");
  await popover.getByRole("button", { name: "Comment" }).click();
  await popover.getByLabel("Comment").fill("Why not SQLite?");
  await expect(submit).toBeDisabled();
  await expect(approve).toBeDisabled();

  // A click on the page clears the selection; the popover and its words stay.
  await page
    .getByRole("heading", { name: "Review cache", exact: true })
    .click();
  await expect(popover.getByLabel("Comment")).toHaveValue("Why not SQLite?");

  await popover.getByRole("button", { name: "Add" }).click();
  await expect(submit).toBeEnabled();
  await submit.click();
  await expect(page.getByText(/This round is closed: changes/)).toBeVisible();
});

test("a forged data-line never becomes a coordinate", async ({
  page,
  request,
}) => {
  await open(page);
  const forged = page.getByText("A forged paragraph claiming line one.");
  await expect(forged).toHaveAttribute("data-line", "71");
  await expect(page.locator('article [data-line="1"]')).toHaveCount(0);

  await select(page, "A forged paragraph");
  await comment(page, "Remove this.");
  await page.getByRole("button", { name: "Submit" }).click();
  await expect(page.getByText(/This round is closed/)).toBeVisible();

  const records = (await (
    await request.get(`${ORIGIN}/__fake/records`)
  ).json()) as {
    annotations: { anchor?: { line: number; resolution: string } }[];
  }[];
  expect(records[0]?.annotations[0]?.anchor).toMatchObject({
    line: 71,
    resolution: "exact",
  });
});

test("Approve records an approval with nothing drafted", async ({
  page,
  request,
}) => {
  await open(page);
  await page.getByRole("button", { name: "Approve", exact: true }).click();
  await expect(page.getByText(/This round is closed: approved/)).toBeVisible();
  const message = await (await request.get(`${ORIGIN}/__fake/feedback`)).text();
  expect(message.trim()).toBe(
    "docz review: DESIGN-0001 Review cache, round 1: approved",
  );
});
