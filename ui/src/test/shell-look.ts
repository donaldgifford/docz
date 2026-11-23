/// <reference types="node" />
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { colorToHex, referenceName, resolveToken } from "@/theme/oklch";

/*
 * The shell's look, resolved (DESIGN-0028 §13). docz's 58 token names
 * get their values from three stylesheets: the design system's
 * tokens.css (oklch, by reference), shell.css (aliases onto the
 * package's names), and docz-tokens.css (docz's own, in hex). A test
 * that checks colours reads hex, so this follows every var() to its
 * literal and converts it.
 *
 * Nothing resolves to nothing: an alias that points at a token the
 * package no longer has, a cycle, or a value that is not a colour is an
 * error that names the token. A lost mapping therefore fails the test
 * that asked for the look, where a regular expression over hex values
 * would have skipped it silently.
 *
 * Test support only: it reads files with node:fs and is imported by no
 * application code.
 */

export type ShellMode = "light" | "dark";

const PACKAGE_TOKENS = "node_modules/@donaldgifford/design-system/src/styles";

function read(path: string): string {
  return readFileSync(resolve(process.cwd(), path), "utf8");
}

const DECLARATION = /--([a-z0-9-]+):\s*([^;]+);/g;

/** Every custom property declared in the blocks opened by `selector`. */
function declarations(css: string, selector: string): Map<string, string> {
  const found = new Map<string, string>();
  const open = new RegExp(
    `(?:^|\\n)\\s*${selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")} \\{([^}]*)\\}`,
    "g",
  );
  for (const block of css.matchAll(open)) {
    for (const match of (block[1] ?? "").matchAll(DECLARATION)) {
      found.set(match[1] ?? "", (match[2] ?? "").trim());
    }
  }
  return found;
}

function merged(...maps: Map<string, string>[]): Map<string, string> {
  return new Map(maps.flatMap((map) => [...map]));
}

/** The custom properties in effect on <html> in `mode`, unresolved. */
function declared(mode: ShellMode): Map<string, string> {
  const pkg = read(`${PACKAGE_TOKENS}/tokens.css`);
  const shell = read("src/theme/shell.css");
  const own = read("src/theme/docz-tokens.css");
  const light = merged(
    declarations(pkg, ":root"),
    declarations(shell, ":root"),
    declarations(own, ":root"),
  );
  return mode === "light"
    ? light
    : merged(light, declarations(pkg, ".dark"), declarations(own, ".dark"));
}

const TINT =
  /^color-mix\(in srgb,\s*(var\(--[a-z0-9-]+\))\s+(\d+)%,\s*transparent\)$/;

function rgba(hex: string, alpha: number): string {
  const [r, g, b] = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16));
  return `rgba(${String(r)}, ${String(g)}, ${String(b)}, ${String(alpha)})`;
}

/**
 * shellLook returns docz's --color-* names in `mode`, each resolved to
 * `#rrggbb`, or to `rgba(r, g, b, a)` for the two accent tints.
 */
export function shellLook(mode: ShellMode): Map<string, string> {
  const tokens = declared(mode);
  const source = (name: string) => tokens.get(name);
  const look = new Map<string, string>();
  for (const [name, value] of tokens) {
    if (!name.startsWith("color-")) {
      continue;
    }
    const tint = TINT.exec(value);
    if (tint !== null) {
      const of = referenceName(tint[1] ?? "") ?? "";
      const hex = colorToHex(resolveToken(of, source));
      look.set(name.slice("color-".length), rgba(hex, Number(tint[2]) / 100));
      continue;
    }
    look.set(
      name.slice("color-".length),
      colorToHex(resolveToken(name, source)),
    );
  }
  return look;
}

/**
 * shellLookCss writes a look as the declarations a hand-written token
 * block would hold, for a test that parses a block of CSS.
 */
export function shellLookCss(mode: ShellMode): string {
  return [...shellLook(mode)]
    .map(([name, value]) => `--color-${name}: ${value};`)
    .join("\n");
}
