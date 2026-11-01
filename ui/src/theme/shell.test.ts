/// <reference types="node" />
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

/*
 * shell.css (DESIGN-0026 §2) has one light set and one dark set, and the
 * dark set appears twice: under data-mode="dark" and for the system's
 * dark preference with no data-mode. tokens.css's @theme block is the
 * same dark set, so a utility Tailwind generates from it names a value
 * the shell really uses.
 */

function read(file: string): string {
  return readFileSync(resolve(process.cwd(), "src/theme", file), "utf8");
}

function colors(css: string, open: string): Map<string, string> {
  const start = css.indexOf(open);
  if (start < 0) {
    throw new Error(`no block ${open}`);
  }
  const body = css.slice(start, css.indexOf("}", start));
  return new Map(
    [...body.matchAll(/--color-([a-z0-9-]+):\s*([^;]+);/g)].map((m) => [
      m[1] ?? "",
      (m[2] ?? "").trim(),
    ]),
  );
}

const shell = read("shell.css");
const light = colors(shell, "\n:root {");
const dark = colors(shell, ':root[data-mode="dark"] {');

describe("shell.css", () => {
  it("gives light and dark the same tokens", () => {
    expect([...light.keys()].sort()).toEqual([...dark.keys()].sort());
    expect(light.size).toBeGreaterThanOrEqual(55);
  });

  it("repeats the dark set for the system's preference", () => {
    expect(colors(shell, ":root:not([data-mode]) {")).toEqual(dark);
  });

  it("is what tokens.css's @theme block declares", () => {
    const theme = colors(read("tokens.css"), "@theme static {");
    for (const [name, value] of theme) {
      expect(dark.get(name), `--color-${name}`).toBe(value);
    }
  });
});
