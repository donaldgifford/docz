import { spawn } from "node:child_process";
import { existsSync, mkdtempSync } from "node:fs";
import { connect } from "node:net";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

import { expect, test } from "@playwright/test";

/*
 * docz-review end to end through its real server (IMPL-0027 Phase 4): a
 * built docz reads the fixture repository, review/main.ts serves the
 * built page from dist-review, and chromium submits. Also the browser's
 * half of the server's defences: a hostile Origin, framing, and an
 * oversized body. Needs `just build` (or DOCZ_REVIEW_DOCZ) and
 * `bun run build:review`; `just ui e2e-review` does the second.
 */

const DOCZ = resolve(process.env.DOCZ_REVIEW_DOCZ ?? "../build/bin/docz");
const REPO = resolve("e2e/fixtures/review-repo");

/** Sends a request's head over a raw socket and returns the status. */
function rawStatus(port: number, head: string[]): Promise<number> {
  return new Promise((resolveStatus, reject) => {
    const sock = connect(port, "127.0.0.1", () => {
      sock.write(`${head.join("\r\n")}\r\n\r\n`);
    });
    let buf = "";
    sock.setEncoding("utf8");
    sock.on("data", (chunk: string) => {
      buf += chunk;
      const m = /^HTTP\/1\.1 (\d{3})/.exec(buf);
      if (m) {
        sock.destroy();
        resolveStatus(Number(m[1]));
      }
    });
    sock.on("error", reject);
    sock.setTimeout(5000, () => {
      sock.destroy();
      reject(new Error(`no status within 5s: ${JSON.stringify(buf)}`));
    });
  });
}

interface Run {
  url: string;
  origin: string;
  token: string;
  /** stdout once the process exits, and its exit code. */
  exited: Promise<{ code: number | null; stdout: string }>;
}

function startReview(args: string[]): Promise<Run> {
  if (!existsSync(DOCZ)) {
    throw new Error(
      `no docz at ${DOCZ}: run just build or set DOCZ_REVIEW_DOCZ`,
    );
  }
  const proc = spawn("bun", ["review/main.ts", ...args, "--repo-root", REPO], {
    env: {
      ...process.env,
      DOCZ_REVIEW_DOCZ: DOCZ,
      DOCZ_REVIEW_STATE_DIR: mkdtempSync(join(tmpdir(), "e2e-review-state-")),
      DOCZ_REVIEW_BROWSER: "/nonexistent/browser",
    },
    stdio: ["ignore", "pipe", "pipe"],
  });
  let stdout = "";
  proc.stdout.on("data", (b: Buffer) => {
    stdout += b.toString();
  });
  const exited = new Promise<{ code: number | null; stdout: string }>(
    (done) => {
      proc.on("exit", (code) => {
        done({ code, stdout });
      });
    },
  );
  return new Promise((ready, fail) => {
    let stderr = "";
    proc.stderr.on("data", (b: Buffer) => {
      stderr += b.toString();
      const m = /open (http:\/\/127\.0\.0\.1:\d+)\/\?t=([0-9a-f]{64})/.exec(
        stderr,
      );
      if (m?.[1] !== undefined && m[2] !== undefined) {
        ready({ url: m[0].slice(5), origin: m[1], token: m[2], exited });
      }
    });
    proc.on("exit", () => {
      fail(new Error(`docz-review exited before serving:\n${stderr}`));
    });
  });
}

test("a review through the real server prints its record", async ({ page }) => {
  const run = await startReview(["DESIGN-0001", "--json"]);
  await page.goto(run.url);
  await expect(
    page.getByRole("heading", { name: "Review cache", level: 1 }),
  ).toBeVisible();
  // The token left the location bar on load.
  expect(page.url()).not.toContain("t=");

  await page
    .locator('[data-question="1"]')
    .getByRole("radio", { name: /An in-process LRU/ })
    .click();
  await page.getByRole("button", { name: "Submit" }).click();
  // The only document has an outcome, so the run settles and the page
  // hears it close.
  await expect(
    page.getByText(
      "The review session ended (settled). You can close this tab.",
    ),
  ).toBeVisible();

  const { code, stdout } = await run.exited;
  expect(code).toBe(0);
  const records = JSON.parse(stdout) as {
    outcome: string;
    questions: { number: number; choice: string }[];
    document: { id: string; content_sha256: string };
  }[];
  expect(records).toHaveLength(1);
  expect(records[0]?.outcome).toBe("changes");
  expect(records[0]?.questions).toEqual([{ number: 1, choice: "c" }]);
  expect(records[0]?.document.id).toBe("DESIGN-0001");
  expect(records[0]?.document.content_sha256).toMatch(/^[0-9a-f]{64}$/);
});

test("a hostile Origin, framing, and an oversized body are refused", async ({
  page,
  request,
}) => {
  const run = await startReview(["DESIGN-0001", "--json"]);
  const auth = { Authorization: `Bearer ${run.token}` };
  const session = (await (
    await request.get(`${run.origin}/api/review`, { headers: auth })
  ).json()) as {
    documents: { round: string; inspect: { content_sha256: string } }[];
  };
  const doc = session.documents[0];
  const key = {
    round: doc?.round,
    content_sha256: doc?.inspect.content_sha256,
    submission_id: "s_e2e",
  };
  const approve = `${run.origin}/api/review/DESIGN-0001/approve`;

  const hostile = await request.post(approve, {
    headers: {
      ...auth,
      Origin: "http://evil.example",
      "Content-Type": "application/json",
    },
    data: key,
  });
  expect(hostile.status()).toBe(403);

  // An oversized body is refused from its Content-Length alone. Sent
  // through a client that uploads the body, the server's early close
  // races the upload and surfaces as EPIPE or a reset, so the request
  // goes over a raw socket with the headers only.
  const big = await rawStatus(Number(new URL(run.origin).port), [
    `POST /api/review/DESIGN-0001/approve HTTP/1.1`,
    `Host: ${new URL(run.origin).host}`,
    `Authorization: ${auth.Authorization}`,
    `Origin: ${run.origin}`,
    "Content-Type: application/json",
    `Content-Length: ${String(5 * 1024 * 1024)}`,
  ]);
  expect(big).toBe(413);

  // A page on another origin cannot frame the review.
  const res = await request.get(`${run.origin}/`);
  expect(res.headers()["content-security-policy"]).toContain(
    "frame-ancestors 'none'",
  );
  // Chromium refuses the frame under frame-ancestors 'none' and shows
  // its error page in its place.
  await page.setContent(`<iframe src="${run.url}"></iframe>`);
  await expect
    .poll(() => page.frames()[1]?.url())
    .toBe("chrome-error://chromewebdata/");

  // Nothing was recorded; Done ends the run with no records.
  const done = await request.post(`${run.origin}/api/review/done`, {
    headers: {
      ...auth,
      Origin: run.origin,
      "Content-Type": "application/json",
    },
    data: {},
  });
  expect(done.status()).toBe(200);
  const { code, stdout } = await run.exited;
  expect(code).toBe(0);
  expect(JSON.parse(stdout)).toEqual([]);
});
