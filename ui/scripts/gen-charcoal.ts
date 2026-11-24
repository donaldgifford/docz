/*
 * Generates ../pkg/theme/tokens/charcoal.css from the design system
 * (DESIGN-0028 §11): pkg/theme's charcoal theme is the package's dark
 * set under pkg/theme's token names, so it is written from the package
 * and not by hand.
 *
 * It reads three things:
 *   - tokens.json from the installed package, its dark set. Values there
 *     are oklch or a {name} reference; src/theme/oklch.ts resolves and
 *     converts them, so what is written is sRGB hex and pkg/theme, its
 *     tests, and the export's goldens keep reading hex.
 *   - src/theme/docz-tokens.css's dark block, for the names the package
 *     has no value for.
 *   - scripts/charcoal.template.css, which keeps the hand-written header
 *     prose and font block.
 *
 * The derivation (which name comes from where) is src/theme/charcoal.ts,
 * where vitest holds it. This file only reads and writes.
 *
 * Run it with `just gen-charcoal` from the repository root, after
 * `just ui install`. `--out <path>` writes somewhere else, which is how
 * scripts/gen-charcoal-check.sh compares without touching the tree.
 * The header records the package's version, and a Go test in pkg/theme
 * holds it to the one bun.lock pins.
 */
import { existsSync } from "node:fs";
import { readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import {
  charcoalTokens,
  doczDarkTokens,
  PACKAGE,
  renderCharcoal,
  type TokenFile,
} from "../src/theme/charcoal";

const here = fileURLToPath(new URL(".", import.meta.url));
const ui = join(here, "..");
const packageDir = join(ui, "node_modules", ...PACKAGE.split("/"));
const defaultOut = join(ui, "..", "pkg", "theme", "tokens", "charcoal.css");

function outPath(args: readonly string[]): string {
  const flag = args.indexOf("--out");
  if (flag < 0) {
    return defaultOut;
  }
  const path = args[flag + 1];
  if (path === undefined) {
    throw new Error("gen-charcoal: --out needs a path");
  }
  return path;
}

async function main(): Promise<void> {
  const tokensPath = join(packageDir, "tokens.json");
  if (!existsSync(tokensPath)) {
    throw new Error(
      `gen-charcoal: ${tokensPath} is missing; run: just ui install`,
    );
  }

  const [tokensJson, packageJson, doczTokens, template] = await Promise.all([
    readFile(tokensPath, "utf8"),
    readFile(join(packageDir, "package.json"), "utf8"),
    readFile(join(ui, "src", "theme", "docz-tokens.css"), "utf8"),
    readFile(join(here, "charcoal.template.css"), "utf8"),
  ]);

  const { version } = JSON.parse(packageJson) as { version: string };
  const tokens = charcoalTokens(
    JSON.parse(tokensJson) as TokenFile,
    doczDarkTokens(doczTokens),
  );

  const out = outPath(process.argv.slice(2));
  await writeFile(out, renderCharcoal(template, version, tokens));
  console.log(
    `gen-charcoal: wrote ${String(tokens.size)} tokens from ${PACKAGE} ${version} to ${out}`,
  );
}

await main();
