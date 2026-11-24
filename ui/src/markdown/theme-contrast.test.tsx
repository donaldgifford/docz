import { render } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import githubDark from "shiki/themes/github-dark.mjs";
import githubLight from "shiki/themes/github-light.mjs";
import { describe, expect, it } from "vitest";

import { renderMarkdown } from "@/markdown/processor";
import {
  AA_NORMAL_TEXT,
  CODE_BG,
  contrastRatio,
  liftColor,
  liftThemeContrast,
  type CodeMode,
} from "@/markdown/theme-contrast";
import { shellToken } from "@/test/shell-look";

const HEX_RE = /^#[0-9a-f]{6}$/i;

type Theme = typeof githubLight;
type RawSetting = NonNullable<Theme["tokenColors"]>[number];

/** The theme each mode highlights in (DESIGN-0028 question 6). */
const THEMES: [CodeMode, Theme][] = [
  ["light", githubLight],
  ["dark", githubDark],
];

function entriesOf(theme: Theme): RawSetting[] {
  return [
    ...(Array.isArray(theme.tokenColors) ? theme.tokenColors : []),
    ...(Array.isArray(theme.settings) ? theme.settings : []),
  ];
}

function hexForeground(entry: RawSetting): string | null {
  const fg = entry.settings.foreground;
  return typeof fg === "string" && HEX_RE.test(fg) ? fg : null;
}

function failing(theme: Theme, bg: string): RawSetting[] {
  return entriesOf(theme).filter((entry) => {
    const fg = hexForeground(entry);
    return fg !== null && contrastRatio(fg, bg) < AA_NORMAL_TEXT;
  });
}

describe("CODE_BG", () => {
  // The lift is arithmetic on a hex surface, and the page paints the
  // design system's --code-bg. They are the same colour or the lift
  // clears a bar the page does not have.
  it.each(["light", "dark"] as const)(
    "is the design system's --code-bg in %s",
    (mode) => {
      expect(CODE_BG[mode]).toBe(shellToken(mode, "code-bg"));
    },
  );

  it("holds the package's own text colour, for a block with no grammar", () => {
    for (const mode of ["light", "dark"] as const) {
      expect(
        contrastRatio(shellToken(mode, "foreground"), CODE_BG[mode]),
        mode,
      ).toBeGreaterThanOrEqual(AA_NORMAL_TEXT);
    }
  });
});

describe("liftThemeContrast", () => {
  it("has something to lift", () => {
    // If both themes ever ship AA-clean on these surfaces the lift is a
    // no-op, and this guard notices.
    const before = THEMES.flatMap(([mode, theme]) =>
      failing(theme, CODE_BG[mode]),
    );
    expect(before.length).toBeGreaterThan(0);
  });

  it.each(THEMES)(
    "brings every %s foreground to AA on that mode's code surface",
    (mode, theme) => {
      const lifted = entriesOf(liftThemeContrast(theme, CODE_BG[mode]));
      expect(lifted.length).toBeGreaterThan(30);
      for (const entry of lifted) {
        const fg = hexForeground(entry);
        if (fg === null) continue;
        expect(
          contrastRatio(fg, CODE_BG[mode]),
          `${JSON.stringify(entry.scope)} ${fg}`,
        ).toBeGreaterThanOrEqual(AA_NORMAL_TEXT);
      }
    },
  );

  it.each(THEMES)(
    "keeps passing %s entries by reference and font styles on lifted ones",
    (mode, theme) => {
      const original = theme.tokenColors ?? [];
      const lifted = liftThemeContrast(theme, CODE_BG[mode]).tokenColors ?? [];
      expect(lifted).toHaveLength(original.length);
      original.forEach((entry, i) => {
        const liftedEntry = lifted[i];
        if (liftedEntry === undefined) {
          throw new Error(`entry ${String(i)} vanished`);
        }
        const fg = hexForeground(entry);
        if (fg === null || contrastRatio(fg, CODE_BG[mode]) >= AA_NORMAL_TEXT) {
          expect(liftedEntry).toBe(entry);
        } else {
          expect(liftedEntry.settings.fontStyle).toBe(entry.settings.fontStyle);
          expect(liftedEntry.scope).toBe(entry.scope);
        }
      });
    },
  );

  it("nudges toward white on dark surfaces and black on light ones", () => {
    expect(liftColor("#ffffff", "#000000")).toBe("#ffffff");
    const onDark = liftColor("#51597d", "#161b28");
    expect(contrastRatio(onDark, "#161b28")).toBeGreaterThanOrEqual(4.5);
    expect(contrastRatio(onDark, "#161b28")).toBeLessThan(5.5);
    const onLight = liftColor("#c0c0c0", "#ffffff");
    expect(contrastRatio(onLight, "#ffffff")).toBeGreaterThanOrEqual(4.5);
  });
});

describe("rendered code", () => {
  async function tokens(): Promise<HTMLElement[]> {
    const { content } = await renderMarkdown(
      "```ts\n// a comment\nconst x = 1;\n```",
    );
    const { container } = render(<MemoryRouter>{content}</MemoryRouter>);
    return [...container.querySelectorAll<HTMLElement>("pre span")].filter(
      (el) => el.style.getPropertyValue("--shiki-light") !== "",
    );
  }

  it("carries both themes on every token and a colour of neither", async () => {
    const all = await tokens();
    expect(all.length).toBeGreaterThan(3);
    for (const token of all) {
      // CSS picks the set from the `dark` class; an inline colour
      // would outrank it and pin one mode.
      expect(token.style.color).toBe("");
      expect(token.style.getPropertyValue("--shiki-dark")).toMatch(HEX_RE);
    }
  });

  it("meets AA in each mode, the comment included", async () => {
    const all = await tokens();
    const comment = all.find((el) => el.textContent.includes("// a comment"));
    if (comment === undefined) {
      throw new Error("no colored comment token rendered");
    }
    for (const token of all) {
      for (const mode of ["light", "dark"] as const) {
        const hex = token.style.getPropertyValue(`--shiki-${mode}`);
        expect(
          contrastRatio(hex, CODE_BG[mode]),
          `${token.textContent} ${hex} in ${mode}`,
        ).toBeGreaterThanOrEqual(AA_NORMAL_TEXT);
      }
    }
  });
});
