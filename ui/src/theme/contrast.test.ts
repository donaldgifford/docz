/// <reference types="node" />
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

// Read the file directly: vitest's CSS stubbing swallows `?raw` imports
// of .css files (verified — they arrive as ""), and import.meta.url is
// not a file: URL under jsdom. The node types are referenced per-file;
// the app tsconfig deliberately stays browser-only.
const tokensCss = readFileSync(
  resolve(process.cwd(), "src/theme/tokens.css"),
  "utf8",
);

/*
 * Badge color contrast (IMPL-0001 Phase 4): status/type/hash tokens
 * render as ~11px badge text on the app surfaces, so they must hold
 * WCAG AA for normal text (4.5:1) against every background they can
 * sit on. jsdom's axe can't compute contrast — this checks the token
 * source mathematically instead; real-browser axe runs in e2e.
 */

const BADGE_TOKEN = /^(st|t|hash)-/;
const SURFACES = ["bg-base", "bg-raised", "bg-elevated"] as const;
const AA_NORMAL_TEXT = 4.5;

function parseTokens(css: string): Map<string, string> {
  const tokens = new Map<string, string>();
  for (const match of css.matchAll(
    /--color-([a-z0-9-]+):\s*(#[0-9a-fA-F]{6})/g,
  )) {
    tokens.set(match[1] ?? "", match[2] ?? "");
  }
  return tokens;
}

function luminance(hex: string): number {
  const [r, g, b] = [1, 3, 5]
    .map((i) => parseInt(hex.slice(i, i + 2), 16) / 255)
    .map((v) =>
      v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4),
    );
  return 0.2126 * (r ?? 0) + 0.7152 * (g ?? 0) + 0.0722 * (b ?? 0);
}

function contrastRatio(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return ((hi ?? 0) + 0.05) / ((lo ?? 0) + 0.05);
}

describe("badge token contrast", () => {
  const tokens = parseTokens(tokensCss);
  const badgeTokens = [...tokens.keys()].filter((name) =>
    BADGE_TOKEN.test(name),
  );

  it("finds the token families it means to check", () => {
    // Guard against a refactor renaming families and silently skipping.
    expect(badgeTokens.length).toBeGreaterThanOrEqual(20);
    for (const surface of SURFACES) {
      expect(tokens.has(surface)).toBe(true);
    }
  });

  it.each(badgeTokens)("--color-%s holds 4.5:1 on every surface", (name) => {
    const hex = tokens.get(name);
    if (hex === undefined) {
      throw new Error(`token ${name} vanished mid-test`);
    }
    for (const surface of SURFACES) {
      const bg = tokens.get(surface);
      if (bg === undefined) {
        throw new Error(`surface ${surface} missing from tokens.css`);
      }
      const ratio = contrastRatio(hex, bg);
      expect(
        ratio,
        `--color-${name} (${hex}) vs --color-${surface} (${bg}) is ${ratio.toFixed(2)}:1`,
      ).toBeGreaterThanOrEqual(AA_NORMAL_TEXT);
    }
  });
});
