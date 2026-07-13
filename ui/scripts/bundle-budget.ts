/*
 * CI bundle-size budget (IMPL-0001 Phase 4): the eager JS must stay
 * small enough that first paint never waits on the heavy lazy chunks.
 * The real regression this guards against is the markdown pipeline
 * (~150 KB gz) or a Shiki grammar leaking into the eager graph — that
 * blows straight through the headroom. Run after `bun run build`:
 *
 *   bun scripts/bundle-budget.ts
 *
 * Measured: the entry chunk PLUS every modulepreload'd chunk in
 * index.html — Rollup splits shared statics (generated client, auth
 * helpers) out of index-*.js as the graph shifts, and those load
 * before first paint just the same. Measuring only the entry file
 * would let an eager import hide in a preloaded chunk.
 *
 * Budget rationale: the eager set (react + router + query + generated
 * client + shell/palette) gzips to ~123 KB today; 130 KB allows normal
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

// The entry chunk is the module script Vite injects into index.html;
// its static import closure is exactly the modulepreload link set.
const entryMatch = /<script type="module"[^>]*src="\/(assets\/[^"]+\.js)"/.exec(
  html,
);
if (entryMatch?.[1] === undefined) {
  console.error("no module script found in dist/index.html");
  process.exit(1);
}
const preloads = [
  ...html.matchAll(/rel="modulepreload"[^>]*href="\/(assets\/[^"]+\.js)"/g),
]
  .map((match) => match[1])
  .filter((path): path is string => path !== undefined);

const kb = (bytes: number): string => `${(bytes / 1024).toFixed(1)} KB`;
let total = 0;
for (const asset of [entryMatch[1], ...preloads]) {
  const gzipped = gzipSync(read(`dist/${asset}`)).byteLength;
  total += gzipped;
  console.log(`  dist/${asset}: ${kb(gzipped)} gzipped`);
}

const verdict = total <= BUDGET_GZIP_BYTES ? "OK" : "OVER BUDGET";
console.log(
  `eager total: ${kb(total)} gzipped (budget ${kb(BUDGET_GZIP_BYTES)}) — ${verdict}`,
);
if (total > BUDGET_GZIP_BYTES) {
  console.error(
    "Eager JS exceeds the budget. Check for eager imports of the " +
      "markdown pipeline, Shiki, or other lazy-only modules before " +
      "raising the number here.",
  );
  process.exit(1);
}
