/*
 * The compiled docz-review smoke test (IMPL-0027 Phase 4): copy a binary
 * into an empty directory with no dist-review anywhere near it, review a
 * document through a fake docz, and fetch the page and one lazy chunk.
 * Proves every asset the page loads was embedded, which a run from the
 * checkout cannot, since it would read dist-review from disk.
 *
 *   bun review/smoke.ts <binary>
 */
import { copyFileSync, mkdtempSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { fakeDocz } from "./testdata/fake-docz";
import { docAs, doczConfig } from "./testdata/review-fixture";

interface Chunk {
  file: string;
  isDynamicEntry?: boolean;
}

const bin = process.argv[2];
if (bin === undefined) {
  throw new Error("usage: bun review/smoke.ts <binary>");
}

const empty = mkdtempSync(join(tmpdir(), "docz-review-bin-"));
const exe = join(empty, "docz-review");
copyFileSync(bin, exe);

const doc = docAs("DESIGN-0042", "docs/design/0042-a-cache-for-the-reader.md");
const fake = fakeDocz(doczConfig([doc]));
const proc = Bun.spawn(
  [exe, doc.inspect.id, "--no-open", "--timeout", "60", "--repo-root", empty],
  {
    cwd: empty,
    env: {
      PATH: process.env.PATH ?? "",
      HOME: empty,
      DOCZ_REVIEW_DOCZ: fake.bin,
      DOCZ_REVIEW_STATE_DIR: join(empty, "state"),
    },
    stdout: "pipe",
    stderr: "pipe",
  },
);

let err = "";
const reader = proc.stderr.getReader();
let url: string | undefined;
while (url === undefined) {
  const { done, value } = await reader.read();
  if (done) {
    throw new Error(`docz-review exited before serving:\n${err}`);
  }
  err += new TextDecoder().decode(value);
  url = /open (http:\/\/127\.0\.0\.1:\d+)\//.exec(err)?.[1];
}

async function fetchOk(path: string, type: string): Promise<void> {
  const res = await fetch(`${url ?? ""}${path}`);
  const got = res.headers.get("content-type") ?? "";
  if (res.status !== 200 || !got.startsWith(type)) {
    throw new Error(`${path}: ${String(res.status)} ${got}`);
  }
  console.log(`docz-review smoke: ${path} ${String(res.status)} ${got}`);
}

const manifest = JSON.parse(
  readFileSync(
    join(import.meta.dir, "..", "dist-review", ".vite", "manifest.json"),
    "utf8",
  ),
) as Record<string, Chunk>;
const lazy = Object.values(manifest).find((c) => c.isDynamicEntry === true);
try {
  await fetchOk("/", "text/html");
  if (lazy === undefined) {
    throw new Error("the manifest names no lazy chunk");
  }
  await fetchOk(`/${lazy.file}`, "text/javascript");
} finally {
  proc.kill("SIGTERM");
  await proc.exited;
}
