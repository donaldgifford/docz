/// <reference types="node" />
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import {
  CHARCOAL_GROUPS,
  CHARCOAL_NAME_COUNT,
  charcoalTokens,
  doczDarkTokens,
  renderCharcoal,
  type Derivation,
  type TokenFile,
} from "./charcoal";
import { TokenError } from "./oklch";

function read(path: string): string {
  return readFileSync(resolve(process.cwd(), path), "utf8");
}

const tokenFile = JSON.parse(
  read("node_modules/@donaldgifford/design-system/tokens.json"),
) as TokenFile;
const doczTokens = read("src/theme/docz-tokens.css");
const own = doczDarkTokens(doczTokens);
const entries = CHARCOAL_GROUPS.flat();

function kinds(kind: Derivation["kind"]): string[] {
  return entries.filter(([, d]) => d.kind === kind).map(([name]) => name);
}

describe("the derivation table", () => {
  it("covers every name once", () => {
    const names = entries.map(([name]) => name);
    expect(names).toHaveLength(CHARCOAL_NAME_COUNT);
    expect(new Set(names).size).toBe(CHARCOAL_NAME_COUNT);
  });

  it("names exactly what pkg/theme's other themes define", () => {
    // paper is the reference theme: every theme defines its tokens
    // (pkg/theme's TestEveryThemeDefinesEveryToken).
    const paper = read("../pkg/theme/tokens/paper.css");
    const paperNames = new Set(
      [...paper.matchAll(/--color-([a-z0-9-]+):/g)].map((m) => m[1] ?? ""),
    );
    expect(entries.map(([name]) => name).sort()).toEqual(
      [...paperNames].sort(),
    );
  });

  it("takes 21 from the package, 9 by rule, 2 as tints, and 26 from docz", () => {
    expect(kinds("package")).toHaveLength(21);
    expect(kinds("same")).toHaveLength(9);
    expect(kinds("tint")).toEqual(["accent-bg", "accent-border"]);
    expect(kinds("own")).toHaveLength(26);
  });

  it("expects from docz exactly what docz-tokens.css defines", () => {
    expect(kinds("own").sort()).toEqual([...own.keys()].sort());
  });

  it("follows a rule to a name the table has", () => {
    const names = new Set(entries.map(([name]) => name));
    for (const [name, derivation] of entries) {
      if (derivation.kind === "same") {
        expect(names.has(derivation.as), name).toBe(true);
      }
      if (derivation.kind === "tint") {
        expect(names.has(derivation.of), name).toBe(true);
      }
    }
  });
});

describe("charcoalTokens", () => {
  const tokens = charcoalTokens(tokenFile, own);

  it("gives all 58 names a value", () => {
    expect(tokens.size).toBe(CHARCOAL_NAME_COUNT);
    for (const [name, value] of tokens) {
      expect(value, name).toMatch(
        /^(#[0-9a-f]{6}|rgba\(\d+, \d+, \d+, 0\.[13]\))$/,
      );
    }
  });

  it("writes the shell's dark values", () => {
    // DESIGN-0028 §11: the surfaces, `bad`, the tints, and the rules.
    expect(Object.fromEntries(tokens)).toMatchObject({
      "bg-base": "#17191c",
      "bg-raised": "#202328",
      "bg-elevated": "#262a31",
      "border-default": "#32373e",
      "fg-primary": "#e8eaed",
      "fg-muted": "#9aa2ac",
      accent: "#61a8f5",
      "accent-bg": "rgba(97, 168, 245, 0.1)",
      "accent-border": "rgba(97, 168, 245, 0.3)",
      "on-accent": "#17191c",
      link: "#61a8f5",
      bad: "#f7756e",
      "st-draft": "#d99f21",
      "st-rejected": "#f7756e",
      "st-deprecated": "#9aa2ac",
      "hash-0": "#7aa2f7",
      "code-fg": "#e8eaed",
    });
  });

  it("takes docz's own values from docz-tokens.css unchanged", () => {
    for (const [name, value] of own) {
      expect(tokens.get(name), name).toBe(value);
    }
  });

  it("fails on a token tokens.json lacks, and names it", () => {
    const dark = Object.fromEntries(
      Object.entries(tokenFile.dark).filter(([name]) => name !== "card"),
    );
    expect(() => charcoalTokens({ dark }, own)).toThrow(TokenError);
    expect(() => charcoalTokens({ dark }, own)).toThrow("unknown token: card");
  });

  it("fails on a reference to a token tokens.json lacks", () => {
    const dark = { ...tokenFile.dark, card: { $value: "{slate-00}" } };
    expect(() => charcoalTokens({ dark }, own)).toThrow(
      "unknown token: slate-00 (from card)",
    );
  });

  it("fails on a name docz-tokens.css leaves without a value", () => {
    const missing = new Map(own);
    missing.delete("t-rfc");
    expect(() => charcoalTokens(tokenFile, missing)).toThrow(
      "--color-t-rfc has no value",
    );
  });

  it("fails on a name given two values", () => {
    const twice = new Map(own).set("bg-base", "#000000");
    expect(() => charcoalTokens(tokenFile, twice)).toThrow(
      "--color-bg-base has two values",
    );
  });

  it("fails on a name docz-tokens.css adds that charcoal does not have", () => {
    const extra = new Map(own).set("t-memo", "#123456");
    expect(() => charcoalTokens(tokenFile, extra)).toThrow(
      "--color-t-memo, which charcoal has no name for",
    );
  });

  it("fails on a name the table lists twice", () => {
    expect(() =>
      charcoalTokens(tokenFile, own, [
        ...CHARCOAL_GROUPS,
        [["bg-base", { kind: "own" }]],
      ]),
    ).toThrow("lists bg-base twice");
  });

  it("fails on rules that point at each other", () => {
    expect(() =>
      charcoalTokens(tokenFile, new Map(), [
        [
          ["a", { kind: "same", as: "b" }],
          ["b", { kind: "same", as: "a" }],
        ],
      ]),
    ).toThrow("derivation cycle: a -> b -> a");
  });
});

describe("doczDarkTokens", () => {
  it("reads the dark block and not the light one", () => {
    const css =
      ":root {\n  --color-t-rfc: #111111;\n}\n\n.dark {\n  --color-t-rfc: #222222;\n}\n";
    expect([...doczDarkTokens(css)]).toEqual([["t-rfc", "#222222"]]);
  });

  it("fails on a name declared twice", () => {
    const css = ".dark {\n  --color-a: #111111;\n  --color-a: #222222;\n}\n";
    expect(() => doczDarkTokens(css)).toThrow("gives --color-a two values");
  });

  it("fails without a dark block", () => {
    expect(() => doczDarkTokens(":root {\n}\n")).toThrow("no .dark block");
  });
});

describe("renderCharcoal", () => {
  const template = read("scripts/charcoal.template.css");
  const tokens = charcoalTokens(tokenFile, own);
  const css = renderCharcoal(template, "9.9.9", tokens);

  it("records the package version in the header", () => {
    expect(css).toContain("@donaldgifford/design-system 9.9.9.");
    expect(css).toContain("GENERATED");
    expect(css).not.toContain("__PACKAGE_VERSION__");
  });

  it("writes one token block, so charcoal stays dark under any data-mode", () => {
    expect(css.match(/:root\[data-theme="charcoal"\]/g)).toHaveLength(1);
    // No selector on the mode: the header's prose may name data-mode.
    expect(css).not.toContain("[data-mode");
    expect(css).not.toContain("@media");
    expect(css.match(/--color-[a-z0-9-]+:/g)).toHaveLength(CHARCOAL_NAME_COUNT);
  });

  it("keeps the hand-written font block", () => {
    expect(css).toContain('html[data-theme="charcoal"] {');
    expect(css).toContain('"IBM Plex Sans"');
    expect(css).not.toContain("@tokens");
  });

  it("writes the same bytes twice", () => {
    expect(renderCharcoal(template, "9.9.9", tokens)).toBe(css);
  });

  it("refuses a template without its markers", () => {
    expect(() => renderCharcoal("/* nothing */", "1.0.0", tokens)).toThrow(
      "exactly once",
    );
  });
});
