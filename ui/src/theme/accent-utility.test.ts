import { describe, expect, it } from "vitest";

import { sourceFiles } from "@/test/source-tree";

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

const SELF = "theme/accent-utility.test.ts";
// The bare utility only: text-accent-foreground is the package's own.
const ACCENT_UTILITY = /(?<![\w-])(bg|text|border)-accent(?![\w-])/g;

describe("the accent utility", () => {
  const scanned = sourceFiles(/\.(ts|tsx|css|html)$/).filter(
    (file) => file.path !== SELF,
  );

  it("scans the source tree", () => {
    expect(scanned.length).toBeGreaterThan(150);
    expect(scanned.some((file) => file.path === "theme/tokens.css")).toBe(true);
  });

  it("is written nowhere under src: use bg-brand, text-brand, border-brand", () => {
    const found = scanned.flatMap((file) =>
      file.text
        .split("\n")
        .flatMap((line, index) =>
          [...line.matchAll(ACCENT_UTILITY)].map(
            (match) => `${file.path}:${String(index + 1)} ${match[0]}`,
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
