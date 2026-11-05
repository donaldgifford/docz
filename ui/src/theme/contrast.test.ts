/// <reference types="node" />
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import {
  THEMES as GENERATED_THEMES,
  THEMES_WITH_MODES,
} from "./__generated__/themes";

// Read the file directly: vitest's CSS stubbing swallows `?raw` imports
// of .css files (verified — they arrive as ""), and import.meta.url is
// not a file: URL under jsdom. The node types are referenced per-file;
// the app tsconfig deliberately stays browser-only.
const tokensCss = readFileSync(
  resolve(process.cwd(), "src/theme/tokens.css"),
  "utf8",
);

/*
 * The themes (DESIGN-0025 §3, IMPL-0028 Phase 6) live in pkg/theme, and
 * the list comes from the generated THEMES, so a theme added there is
 * checked here with no edit (IMPL-0030 Phase 4). Each mode is one
 * top-level block: a theme without modes has one for every mode, a
 * theme with them one for light and one for dark. Paper's
 * prefers-color-scheme block repeats the dark values, which pkg/theme's
 * theme_test.go holds equal, so it is not checked twice here.
 * The shell (DESIGN-0026 §2) is checked in both modes, and tokens.css's
 * @theme values, its dark set (shell.test.ts), as the source Tailwind
 * reads.
 */
const themeDir = resolve(process.cwd(), "../pkg/theme/tokens");

/** The body of the top-level block whose selector is exactly `selector`. */
function block(css: string, selector: string): string {
  const start = css.indexOf(`\n${selector} {`);
  if (start < 0) {
    throw new Error(`no block ${selector}`);
  }
  const end = css.indexOf("\n}", start);
  return css.slice(start, end);
}

const shellCss = readFileSync(
  resolve(process.cwd(), "src/theme/shell.css"),
  "utf8",
);

const THEMES: [string, string][] = [
  ["tokens.css", tokensCss],
  ["shell light", block(shellCss, ":root")],
  ["shell dark", block(shellCss, ':root[data-mode="dark"]')],
  ...GENERATED_THEMES.flatMap((name): [string, string][] => {
    const css = readFileSync(resolve(themeDir, `${name}.css`), "utf8");
    const root = `:root[data-theme="${name}"]`;
    return THEMES_WITH_MODES.includes(name)
      ? [
          [`${name} light`, block(css, root)],
          [`${name} dark`, block(css, `${root}[data-mode="dark"]`)],
        ]
      : [[name, block(css, root)]];
  }),
];

/*
 * Text color contrast (IMPL-0001 Phase 4): every token that renders
 * text — the fg scale, accent (links), and the st-/t-/hash- badge
 * palettes (~11px badge text) — must hold WCAG AA for normal text
 * (4.5:1) against every surface it can sit on. jsdom's axe can't
 * compute contrast — this checks the token source mathematically
 * instead; real-browser axe (full rules) runs in the e2e suite.
 */

// ok, link, warn, and bad are pkg/theme's own: text in exported pages
// and on the review page, which tokens.css never had.
const TEXT_TOKEN = /^(st|t|hash|fg)-|^(accent|ok|link|warn|bad)$/;
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

describe.each(THEMES)("%s text token contrast", (_, css) => {
  const tokens = parseTokens(css);
  const textTokens = [...tokens.keys()].filter((name) => TEXT_TOKEN.test(name));

  it("finds the token families it means to check", () => {
    // Guard against a refactor renaming families and silently skipping.
    expect(textTokens.length).toBeGreaterThanOrEqual(25);
    for (const surface of SURFACES) {
      expect(tokens.has(surface)).toBe(true);
    }
  });

  it.each(textTokens)("--color-%s holds 4.5:1 on every surface", (name) => {
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

/*
 * The accent tint (IMPL-0030 Phase 3): a selected nav item and a
 * pressed toggle set accent text on --color-accent-bg, a translucent
 * fill, so the pair is checked on that fill composited over the two
 * surfaces those controls sit on (the page and a raised panel; none
 * sits on bg-elevated). Real-browser axe found paper light 0.05 short
 * of AA here.
 */
const TINT_SURFACES = ["bg-base", "bg-raised"] as const;

function composite(rgba: string, bg: string): string {
  const match = /rgba\((\d+),\s*(\d+),\s*(\d+),\s*([\d.]+)\)/.exec(rgba);
  if (match === null) {
    throw new Error(`not an rgba() value: ${rgba}`);
  }
  const alpha = Number(match[4]);
  return `#${[1, 2, 3]
    .map((i) => {
      const fg = Number(match[i]);
      const under = parseInt(bg.slice(2 * i - 1, 2 * i + 1), 16);
      return Math.round(alpha * fg + (1 - alpha) * under)
        .toString(16)
        .padStart(2, "0");
    })
    .join("")}`;
}

describe.each(THEMES)("%s accent tint contrast", (_, css) => {
  const tokens = parseTokens(css);
  const tint = /--color-accent-bg:\s*(rgba\([^)]*\))/.exec(css)?.[1];

  it.each(TINT_SURFACES)(
    "accent holds 4.5:1 on accent-bg over %s",
    (surface) => {
      const accent = tokens.get("accent");
      const bg = tokens.get(surface);
      if (accent === undefined || bg === undefined || tint === undefined) {
        throw new Error("accent, accent-bg, or the surface is missing");
      }
      const fill = composite(tint, bg);
      const ratio = contrastRatio(accent, fill);
      expect(
        ratio,
        `--color-accent (${accent}) on accent-bg over --color-${surface} (${fill}) is ${ratio.toFixed(2)}:1`,
      ).toBeGreaterThanOrEqual(AA_NORMAL_TEXT);
    },
  );
});

describe.each(THEMES)("%s admonition tint contrast", (_, css) => {
  const tokens = parseTokens(css);
  // Each alert kind renders its label in the mapped status color and
  // its body in fg-prose (the reader body color, DESIGN-0005), both on
  // the precomputed adm-*-bg tint (tokens.css keeps those as literal
  // hex for exactly this check).
  const KIND_TO_LABEL_TOKEN: Record<string, string> = {
    note: "st-proposed",
    tip: "st-accepted",
    important: "st-superseded",
    warning: "st-draft",
    caution: "st-rejected",
  };

  it.each(Object.entries(KIND_TO_LABEL_TOKEN))(
    "%s label and body text hold 4.5:1 on the tint",
    (kind, labelToken) => {
      const bg = tokens.get(`adm-${kind}-bg`);
      if (bg === undefined) {
        throw new Error(`--color-adm-${kind}-bg missing from tokens.css`);
      }
      for (const textToken of [labelToken, "fg-prose"]) {
        const hex = tokens.get(textToken);
        if (hex === undefined) {
          throw new Error(`--color-${textToken} missing from tokens.css`);
        }
        const ratio = contrastRatio(hex, bg);
        expect(
          ratio,
          `--color-${textToken} (${hex}) vs --color-adm-${kind}-bg (${bg}) is ${ratio.toFixed(2)}:1`,
        ).toBeGreaterThanOrEqual(AA_NORMAL_TEXT);
      }
    },
  );
});

/*
 * Type accents (DESIGN-0026 §8): on a type's routes the shell's accent is
 * the type's colour, and the active sidebar item sets it on its own 10%
 * tint, so each shell type and hash colour is held to AA on itself at
 * 10% over the page and a raised panel. Inside a document the accent
 * changes only text and links; the theme keeps its own tint, so the
 * document themes' colours are held only by the checks above.
 */
function mix(hex: string, bg: string, alpha: number): string {
  const [r, g, b] = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16));
  return composite(
    `rgba(${String(r)}, ${String(g)}, ${String(b)}, ${String(alpha)})`,
    bg,
  );
}

const SHELL_LOOKS = THEMES.filter(
  ([name]) => name === "tokens.css" || name.startsWith("shell "),
);

describe.each(SHELL_LOOKS)("%s type accent tint contrast", (_, css) => {
  const tokens = parseTokens(css);
  const accents = [...tokens.keys()].filter((name) => /^(t|hash)-/.test(name));

  it.each(accents)("--color-%s holds 4.5:1 on its own tint", (name) => {
    const hex = tokens.get(name);
    if (hex === undefined) {
      throw new Error(`token ${name} vanished mid-test`);
    }
    for (const surface of TINT_SURFACES) {
      const bg = tokens.get(surface);
      if (bg === undefined) {
        throw new Error(`surface ${surface} missing`);
      }
      const fill = mix(hex, bg, 0.1);
      const ratio = contrastRatio(hex, fill);
      expect(
        ratio,
        `--color-${name} (${hex}) on its tint over --color-${surface} (${fill}) is ${ratio.toFixed(2)}:1`,
      ).toBeGreaterThanOrEqual(AA_NORMAL_TEXT);
    }
  });
});

/*
 * The review page's round diff (ui/src/review/diff.tsx) sets its lines on
 * the shell's page surface: added text in fg-primary on a 12% st-active
 * tint, removed in fg-secondary on a 12% st-rejected tint, unchanged in
 * fg-tertiary. Each pair is held in both shell modes, because the box once
 * sat on the always-dark code background and the light shell read 1.04:1.
 */
const DIFF_LINES = [
  ["fg-primary", "st-active"],
  ["fg-secondary", "st-rejected"],
  ["fg-tertiary", null],
] as const;

describe.each(SHELL_LOOKS)("%s round diff contrast", (_, css) => {
  const tokens = parseTokens(css);

  it.each(DIFF_LINES)("%s holds 4.5:1 on the diff surface", (text, tint) => {
    const fg = tokens.get(text);
    const base = tokens.get("bg-base");
    const hue = tint === null ? undefined : tokens.get(tint);
    if (
      fg === undefined ||
      base === undefined ||
      (tint !== null && hue === undefined)
    ) {
      throw new Error(`a token for ${text} on the diff surface is missing`);
    }
    const bg = hue === undefined ? base : mix(hue, base, 0.12);
    const ratio = contrastRatio(fg, bg);
    expect(
      ratio,
      `--color-${text} (${fg}) on the diff surface (${bg}) is ${ratio.toFixed(2)}:1`,
    ).toBeGreaterThanOrEqual(AA_NORMAL_TEXT);
  });
});
