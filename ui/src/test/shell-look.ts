/// <reference types="node" />
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { CHARCOAL_GROUPS } from "@/theme/charcoal";
import { colorToHex, resolveToken } from "@/theme/oklch";

/*
 * The shell's look, resolved (DESIGN-0028 §13). The shell is the design
 * system's tokens.css (oklch, by reference) with docz's own 26 names
 * beside it in docz-tokens.css (hex). A test that checks colours reads
 * hex, so this follows every var() to its literal and converts it.
 *
 * shellLook gives the look under pkg/theme's 58 names, so one contrast
 * test covers the shell and every document theme. No stylesheet maps a
 * docz name onto a package token any more (IMPL-0033 Phase 9): the
 * mapping is charcoal.ts's derivation table, the one that writes
 * pkg/theme's charcoal theme from the same tokens, read here in both
 * modes.
 *
 * Nothing resolves to nothing: a name that points at a token the
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
  const own = read("src/theme/docz-tokens.css");
  const light = merged(declarations(pkg, ":root"), declarations(own, ":root"));
  return mode === "light"
    ? light
    : merged(light, declarations(pkg, ".dark"), declarations(own, ".dark"));
}

function rgba(hex: string, alpha: number): string {
  const [r, g, b] = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16));
  return `rgba(${String(r)}, ${String(g)}, ${String(b)}, ${String(alpha)})`;
}

/**
 * shellLook returns pkg/theme's 58 names as the shell has them in
 * `mode`, each resolved to `#rrggbb`, or to `rgba(r, g, b, a)` for the
 * two accent tints.
 */
export function shellLook(mode: ShellMode): Map<string, string> {
  const tokens = declared(mode);
  const source = (name: string) => tokens.get(name);
  const table = new Map(CHARCOAL_GROUPS.flat());
  const valueOf = (name: string, seen: readonly string[] = []): string => {
    const derivation = table.get(name);
    if (derivation === undefined || seen.includes(name)) {
      throw new Error(`the derivation table cannot resolve ${name}`);
    }
    switch (derivation.kind) {
      case "package":
        return colorToHex(resolveToken(derivation.token, source));
      case "own":
        return colorToHex(resolveToken(`color-${name}`, source));
      case "same":
        return valueOf(derivation.as, [...seen, name]);
      case "tint":
        return rgba(
          valueOf(derivation.of, [...seen, name]),
          derivation.percent / 100,
        );
    }
  };
  return new Map([...table.keys()].map((name) => [name, valueOf(name)]));
}

/**
 * shellToken resolves one custom property in effect on <html> in
 * `mode` to `#rrggbb`: a name of the package's own (`code-bg`,
 * `foreground`), which is how the chrome reads it.
 */
export function shellToken(mode: ShellMode, name: string): string {
  const tokens = declared(mode);
  return colorToHex(resolveToken(name, (token) => tokens.get(token)));
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
