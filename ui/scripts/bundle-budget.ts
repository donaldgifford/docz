/*
 * CI bundle-size budget (IMPL-0001 Phase 4): the entry chunk must stay
 * small enough that first paint never waits on the heavy lazy chunks.
 * The real regression this guards against is the markdown pipeline
 * (~150 KB gz) or a Shiki grammar leaking into the eager graph — that
 * blows straight through the headroom. Run after `bun run build`:
 *
 *   bun scripts/bundle-budget.ts
 *
 * Budget rationale: the entry (react + router + query + generated
 * client + shell/palette) gzips to ~117 KB today; 130 KB allows normal
 * growth while any eager-import regression fails loudly.
 */

import { readFileSync } from "node:fs";
import { gzipSync } from "node:zlib";

const BUDGET_GZIP_BYTES = 130 * 1024;

function read(path: string): Buffer {
  try {
    return readFileSync(path);
  } catch {
    console.error(`${path} missing — run \`bun run build\` first`);
    process.exit(1);
  }
}

const html = read("dist/index.html").toString("utf8");

// The entry chunk is the module script Vite injects into index.html.
const entryMatch = /<script type="module"[^>]*src="\/(assets\/[^"]+\.js)"/.exec(
  html,
);
if (entryMatch?.[1] === undefined) {
  console.error("no module script found in dist/index.html");
  process.exit(1);
}
const entryPath = `dist/${entryMatch[1]}`;
const gzipped = gzipSync(read(entryPath)).byteLength;

const kb = (bytes: number): string => `${(bytes / 1024).toFixed(1)} KB`;
const verdict = gzipped <= BUDGET_GZIP_BYTES ? "OK" : "OVER BUDGET";
console.log(
  `${entryPath}: ${kb(gzipped)} gzipped (budget ${kb(BUDGET_GZIP_BYTES)}) — ${verdict}`,
);
if (gzipped > BUDGET_GZIP_BYTES) {
  console.error(
    "Entry chunk exceeds the budget. Check for eager imports of the " +
      "markdown pipeline, Shiki, or other lazy-only modules before " +
      "raising the number here.",
  );
  process.exit(1);
}
