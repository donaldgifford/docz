/// <reference types="node" />
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import { shellLook } from "@/test/shell-look";

/*
 * docz's 58 token names in two files (DESIGN-0028 §4): shell.css aliases
 * the 32 whose value is the design system's, and docz-tokens.css holds
 * docz's own 26 in light and dark. tokens.css's @theme block registers
 * the names Tailwind writes utilities for and has to agree with both.
 * Before the package arrived every value was a hex in shell.css, so
 * these also hold each name to the value it had then.
 */

function read(file: string): string {
  return readFileSync(resolve(process.cwd(), "src/theme", file), "utf8");
}

function colors(css: string, open: string): Map<string, string> {
  const start = css.indexOf(open);
  if (start < 0) {
    throw new Error(`no block ${open}`);
  }
  const body = css.slice(start, css.indexOf("\n}", start));
  return new Map(
    [...body.matchAll(/--color-([a-z0-9-]+):\s*([^;]+);/g)].map((m) => [
      m[1] ?? "",
      (m[2] ?? "").trim(),
    ]),
  );
}

const aliases = colors(read("shell.css"), "\n:root {");
const own = read("docz-tokens.css");
const ownLight = colors(own, "\n:root {");
const ownDark = colors(own, "\n.dark {");

describe("docz's token names", () => {
  it("are 32 shared and 26 own, 58 in all, with none in both", () => {
    expect(aliases.size).toBe(32);
    expect(ownLight.size).toBe(26);
    expect(new Set([...aliases.keys(), ...ownLight.keys()]).size).toBe(58);
  });

  it("give docz's own tokens the same names in light and dark", () => {
    expect([...ownDark.keys()]).toEqual([...ownLight.keys()]);
  });

  it("alias a shared name to a package token and nothing else", () => {
    for (const [name, value] of aliases) {
      expect(value, `--color-${name}`).toMatch(
        /^(var\(--[a-z0-9-]+\)|color-mix\(in srgb, var\(--primary\) \d+%, transparent\))$/,
      );
    }
  });

  it("keep docz's own tokens in hex", () => {
    for (const [name, value] of [...ownLight, ...ownDark]) {
      expect(value, `--color-${name}`).toMatch(/^#[0-9a-f]{6}$/);
    }
  });
});

describe("tokens.css's @theme block", () => {
  const theme = colors(read("tokens.css"), "@theme static {");

  it("registers no name the two files do not define", () => {
    for (const name of theme.keys()) {
      expect(aliases.has(name) || ownDark.has(name), `--color-${name}`).toBe(
        true,
      );
    }
    expect(theme.size).toBeGreaterThanOrEqual(45);
  });

  it("gives a shared name its alias and an own name its dark value", () => {
    for (const [name, value] of theme) {
      expect(value, `--color-${name}`).toBe(
        aliases.get(name) ?? ownDark.get(name),
      );
    }
  });

  it("leaves the accent to the brand utility", () => {
    // --color-accent as a theme key would take the package's accent
    // utility, its hover surface, and paint it in docz's brand colour.
    expect([...theme.keys()].filter((n) => n.includes("accent"))).toEqual([]);
    const brand = colors(read("tokens.css"), "@theme inline {");
    expect(Object.fromEntries(brand)).toEqual({
      brand: "var(--color-accent)",
      "brand-dim": "var(--color-accent-dim)",
      "brand-bg": "var(--color-accent-bg)",
      "brand-border": "var(--color-accent-border)",
      "on-brand": "var(--color-on-accent)",
    });
  });
});

/*
 * What every shared name resolved to before the package: shell.css's
 * hex, light and dark, as of IMPL-0032. The aliases must land on the
 * same values, or moving onto the package changed a colour.
 */
const BEFORE: Record<string, [light: string, dark: string]> = {
  "bg-base": ["#f6f7f9", "#17191c"],
  "bg-raised": ["#ffffff", "#202328"],
  "bg-elevated": ["#edeff2", "#262a31"],
  "bg-active": ["#d9dde3", "#32373e"],
  "border-default": ["#d9dde3", "#32373e"],
  "border-strong": ["#b8bec7", "#474e57"],
  "fg-primary": ["#1a1e23", "#e8eaed"],
  "fg-muted": ["#515b67", "#9aa2ac"],
  accent: ["#1963b3", "#61a8f5"],
  "accent-bg": ["rgba(25, 99, 179, 0.1)", "rgba(97, 168, 245, 0.1)"],
  "accent-border": ["rgba(25, 99, 179, 0.3)", "rgba(97, 168, 245, 0.3)"],
  "on-accent": ["#ffffff", "#17191c"],
  ok: ["#196629", "#40ba52"],
  link: ["#1963b3", "#61a8f5"],
  warn: ["#835107", "#d99f21"],
  bad: ["#a7241b", "#f7756e"],
  "st-draft": ["#835107", "#d99f21"],
  "st-proposed": ["#2757b0", "#7aa7ff"],
  "st-active": ["#196629", "#40ba52"],
  "st-accepted": ["#196629", "#40ba52"],
  "st-adopted": ["#196629", "#40ba52"],
  "st-rejected": ["#a7241b", "#f7756e"],
  "st-deprecated": ["#515b67", "#9aa2ac"],
  "hash-0": ["#4164b1", "#7aa2f7"],
  "hash-1": ["#497103", "#9ece6a"],
  "hash-2": ["#8a5f0a", "#e0af68"],
  "hash-3": ["#b43958", "#f7768e"],
  "hash-4": ["#7756a9", "#bb9af7"],
  "hash-5": ["#057366", "#73daca"],
  "hash-6": ["#a44d05", "#ff9e64"],
  "hash-7": ["#057080", "#2ac3de"],
  "code-fg": ["#e8eaed", "#e8eaed"],
};

describe("the shared names, resolved through the package", () => {
  const light = shellLook("light");
  const dark = shellLook("dark");

  it("cover exactly the aliases", () => {
    expect(Object.keys(BEFORE).sort()).toEqual([...aliases.keys()].sort());
  });

  it.each(Object.entries(BEFORE))(
    "--color-%s is what the shell carried",
    (name, [lightValue, darkValue]) => {
      expect(light.get(name)).toBe(lightValue);
      expect(dark.get(name)).toBe(darkValue);
    },
  );

  it("resolve all 58 names in both modes", () => {
    expect(light.size).toBe(58);
    expect(dark.size).toBe(58);
  });
});
