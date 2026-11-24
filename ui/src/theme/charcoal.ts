import {
  colorToHex,
  resolveToken,
  TokenError,
  type TokenSource,
} from "./oklch";

/*
 * pkg/theme's charcoal theme, derived (DESIGN-0028 §11). charcoal is the
 * shell's look as a document theme: the design system's dark set under
 * pkg/theme's 58 token names. This module is the derivation, with no
 * file access, so vitest can hold it; scripts/gen-charcoal.ts does the
 * reading and writing, as scripts/gen-themes.ts does over scope-css.ts.
 *
 * Every name gets its value one of four ways:
 *
 *   package  the package's dark token under another name (21 names)
 *   same     another of these names, by rule: link is the accent,
 *            st-draft is warn, st-deprecated is fg-muted (9 names)
 *   tint     the accent at 10% and 30% (2 names)
 *   own      docz's own, read from docz-tokens.css's dark block (26)
 *
 * The first three are the 32 names whose value is the package's. The
 * table lists them in the order charcoal.css prints them, one group
 * per paragraph. It is also the only place a docz name is mapped onto
 * a package token: src/test/shell-look.ts reads it to give the
 * contrast test the shell's look under these names.
 */

export const PACKAGE = "@donaldgifford/design-system";

export type Derivation =
  | { readonly kind: "package"; readonly token: string }
  | { readonly kind: "same"; readonly as: string }
  | { readonly kind: "tint"; readonly of: string; readonly percent: number }
  | { readonly kind: "own" };

const pkg = (token: string): Derivation => ({ kind: "package", token });
const same = (as: string): Derivation => ({ kind: "same", as });
const tint = (of: string, percent: number): Derivation => ({
  kind: "tint",
  of,
  percent,
});
const own: Derivation = { kind: "own" };

type Entry = readonly [name: string, derivation: Derivation];

/** The 58 names, grouped as charcoal.css prints them. */
export const CHARCOAL_GROUPS: readonly (readonly Entry[])[] = [
  [
    ["bg-base", pkg("background")],
    ["bg-raised", pkg("card")],
    ["bg-elevated", pkg("secondary")],
    ["bg-hover", own],
    ["bg-active", pkg("surface-pressed")],
  ],
  [
    ["border-hairline", own],
    ["border-default", pkg("border")],
    ["border-strong", pkg("border-strong")],
  ],
  [
    ["fg-primary", pkg("foreground")],
    ["fg-secondary", own],
    ["fg-tertiary", own],
    ["fg-muted", pkg("muted-foreground")],
    ["fg-prose", own],
  ],
  [
    ["accent", pkg("primary")],
    ["accent-dim", own],
    ["accent-bg", tint("accent", 10)],
    ["accent-border", tint("accent", 30)],
    ["on-accent", same("bg-base")],
  ],
  [
    ["ok", pkg("success")],
    ["link", same("accent")],
    ["warn", pkg("warning")],
    ["bad", pkg("destructive")],
  ],
  [
    ["st-draft", same("warn")],
    ["st-proposed", pkg("info")],
    ["st-active", same("ok")],
    ["st-accepted", same("ok")],
    ["st-adopted", same("ok")],
    ["st-rejected", same("bad")],
    ["st-superseded", own],
    ["st-deprecated", same("fg-muted")],
  ],
  [
    ["adm-note-bg", own],
    ["adm-tip-bg", own],
    ["adm-important-bg", own],
    ["adm-warning-bg", own],
    ["adm-caution-bg", own],
  ],
  [
    ["t-rfc", own],
    ["t-adr", own],
    ["t-design", own],
    ["t-impl", own],
    ["t-investigation", own],
    ["t-mandate", own],
    ["t-guide", own],
    ["t-principle", own],
    ["t-policy", own],
    ["t-framework", own],
    ["t-working", own],
    ["t-runbook", own],
    ["t-docs", own],
  ],
  [
    ["hash-0", pkg("category-1")],
    ["hash-1", pkg("category-2")],
    ["hash-2", pkg("category-3")],
    ["hash-3", pkg("category-4")],
    ["hash-4", pkg("category-5")],
    ["hash-5", pkg("category-6")],
    ["hash-6", pkg("category-7")],
    ["hash-7", pkg("category-8")],
  ],
  [
    ["code-bg", own],
    ["code-fg", same("fg-primary")],
  ],
];

/** How many names charcoal has; pkg/theme's every theme defines these. */
export const CHARCOAL_NAME_COUNT = 58;

/** The package's tokens.json, as far as the generator reads it. */
export interface TokenFile {
  readonly dark: Readonly<Record<string, { readonly $value: string }>>;
}

const DARK_BLOCK = /(?:^|\n)\.dark \{([^}]*)\}/;
const COLOR_DECLARATION = /--color-([a-z0-9-]+):\s*([^;]+);/g;

/**
 * doczDarkTokens reads docz-tokens.css's dark block: docz's own names
 * and their values. A name declared twice is an error, since the second
 * would silently win.
 */
export function doczDarkTokens(css: string): Map<string, string> {
  const block = DARK_BLOCK.exec(css)?.[1];
  if (block === undefined) {
    throw new TokenError("docz-tokens.css has no .dark block");
  }
  const tokens = new Map<string, string>();
  for (const match of block.matchAll(COLOR_DECLARATION)) {
    const name = match[1] ?? "";
    if (tokens.has(name)) {
      throw new TokenError(
        `docz-tokens.css gives --color-${name} two values in .dark`,
      );
    }
    tokens.set(name, (match[2] ?? "").trim());
  }
  return tokens;
}

function rgba(hex: string, percent: number): string {
  const [r, g, b] = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16));
  return `rgba(${String(r)}, ${String(g)}, ${String(b)}, ${String(percent / 100)})`;
}

/**
 * charcoalTokens gives each of the 58 names its value: hex, or rgba()
 * for the two tints. It fails, naming the token, when a name is listed
 * twice, when a name has no value or two, when docz-tokens.css holds a
 * name the table does not expect from it, and when the package's
 * tokens.json lacks a token the table reads.
 */
export function charcoalTokens(
  tokenFile: TokenFile,
  ownTokens: ReadonlyMap<string, string>,
  groups: readonly (readonly Entry[])[] = CHARCOAL_GROUPS,
): Map<string, string> {
  const entries = groups.flat();
  const table = new Map<string, Derivation>();
  for (const [name, derivation] of entries) {
    if (table.has(name)) {
      throw new TokenError(`the derivation table lists ${name} twice`);
    }
    table.set(name, derivation);
  }

  for (const name of ownTokens.keys()) {
    const derivation = table.get(name);
    if (derivation === undefined) {
      throw new TokenError(
        `docz-tokens.css defines --color-${name}, which charcoal has no name for`,
      );
    }
    if (derivation.kind !== "own") {
      throw new TokenError(
        `--color-${name} has two values: docz-tokens.css defines it and the derivation table takes it from the package`,
      );
    }
  }

  const source: TokenSource = (name) => tokenFile.dark[name]?.$value;
  const values = new Map<string, string>();

  const valueOf = (name: string, trail: readonly string[]): string => {
    const known = values.get(name);
    if (known !== undefined) {
      return known;
    }
    const derivation = table.get(name);
    if (derivation === undefined) {
      throw new TokenError(`the derivation table has no ${name}`);
    }
    if (trail.includes(name)) {
      throw new TokenError(
        `derivation cycle: ${[...trail, name].join(" -> ")}`,
      );
    }
    switch (derivation.kind) {
      case "package":
        return colorToHex(resolveToken(derivation.token, source));
      case "same":
        return valueOf(derivation.as, [...trail, name]);
      case "tint":
        return rgba(
          valueOf(derivation.of, [...trail, name]),
          derivation.percent,
        );
      case "own": {
        const value = ownTokens.get(name);
        if (value === undefined) {
          throw new TokenError(
            `--color-${name} has no value: docz-tokens.css's .dark block does not define it`,
          );
        }
        return colorToHex(value);
      }
    }
  };

  for (const [name] of entries) {
    values.set(name, valueOf(name, []));
  }
  return values;
}

const VERSION_MARK = "__PACKAGE_VERSION__";
const TOKENS_MARK = "  /* @tokens */\n";

/**
 * renderCharcoal fills the template: the package version in the header
 * and the token block where the marker is. The header's prose and the
 * font block are the template's, hand-written.
 */
export function renderCharcoal(
  template: string,
  version: string,
  tokens: ReadonlyMap<string, string>,
  groups: readonly (readonly Entry[])[] = CHARCOAL_GROUPS,
): string {
  for (const mark of [VERSION_MARK, TOKENS_MARK]) {
    if (template.split(mark).length !== 2) {
      throw new TokenError(
        `the charcoal template must hold ${JSON.stringify(mark)} exactly once`,
      );
    }
  }
  const block = groups
    .map((group) =>
      group
        .map(([name]) => {
          const value = tokens.get(name);
          if (value === undefined) {
            throw new TokenError(`--color-${name} has no value`);
          }
          return `  --color-${name}: ${value};\n`;
        })
        .join(""),
    )
    .join("\n");
  return template.replace(VERSION_MARK, version).replace(TOKENS_MARK, block);
}
