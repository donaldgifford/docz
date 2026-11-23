/// <reference types="node" />
import { readdirSync, readFileSync } from "node:fs";
import { join, relative, resolve } from "node:path";

import { describe, expect, it } from "vitest";

/*
 * docz's brand colour is the `brand` utility (DESIGN-0028 question 2).
 * `accent` is the design system's name for its hover surface, and its
 * theme map is inline, so bg-accent, text-accent, and border-accent
 * compile to that surface wherever they are written. Before the rename
 * all three meant docz's brand colour, so one written from habit paints
 * a grey where a blue was meant, and nothing else fails. This does.
 *
 * It reads the files as text: a class name in a string, a template, or
 * an @apply is all the same mistake. CSS comments are skipped, since
 * tokens.css explains the rule by naming the utility.
 */

const SRC = resolve(process.cwd(), "src");
const SELF = "theme/accent-utility.test.ts";
const SCANNED = /\.(ts|tsx|css|html)$/;
// The bare utility only: text-accent-foreground is the package's own.
const ACCENT_UTILITY = /(?<![\w-])(bg|text|border)-accent(?![\w-])/g;

function files(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      return entry.name === "__generated__" ? [] : files(path);
    }
    return SCANNED.test(entry.name) ? [path] : [];
  });
}

function source(path: string): string {
  const text = readFileSync(path, "utf8");
  return path.endsWith(".css") ? text.replace(/\/\*[\s\S]*?\*\//g, "") : text;
}

describe("the accent utility", () => {
  const scanned = files(SRC).filter((path) => relative(SRC, path) !== SELF);

  it("scans the source tree", () => {
    expect(scanned.length).toBeGreaterThan(150);
    expect(scanned.some((path) => path.endsWith("theme/tokens.css"))).toBe(
      true,
    );
  });

  it("is written nowhere under src: use bg-brand, text-brand, border-brand", () => {
    const found = scanned.flatMap((path) =>
      source(path)
        .split("\n")
        .flatMap((line, index) =>
          [...line.matchAll(ACCENT_UTILITY)].map(
            (match) =>
              `${relative(SRC, path)}:${String(index + 1)} ${match[0]}`,
          ),
        ),
    );
    expect(found).toEqual([]);
  });

  it("catches each of the three, and leaves the package's own names", () => {
    const hits = (text: string) =>
      [...text.matchAll(ACCENT_UTILITY)].map((match) => match[0]);
    expect(hits("bg-accent text-accent hover:border-accent")).toEqual([
      "bg-accent",
      "text-accent",
      "border-accent",
    ]);
    expect(
      hits(
        "text-accent-foreground bg-brand border-brand-border var(--color-accent) bg-sidebar-accent",
      ),
    ).toEqual([]);
  });
});
