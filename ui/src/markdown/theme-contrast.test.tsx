/// <reference types="node" />
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { render } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import tokyoNight from "shiki/themes/tokyo-night.mjs";
import { describe, expect, it } from "vitest";

import { renderMarkdown } from "@/markdown/processor";
import {
  AA_NORMAL_TEXT,
  CODE_BG,
  contrastRatio,
  liftColor,
  liftThemeContrast,
} from "@/markdown/theme-contrast";

const HEX_RE = /^#[0-9a-f]{6}$/i;

type RawSetting = NonNullable<typeof tokyoNight.tokenColors>[number];

function entriesOf(theme: typeof tokyoNight): RawSetting[] {
  return [
    ...(Array.isArray(theme.tokenColors) ? theme.tokenColors : []),
    ...(Array.isArray(theme.settings) ? theme.settings : []),
  ];
}

function hexForeground(entry: RawSetting): string | null {
  const fg = entry.settings.foreground;
  return typeof fg === "string" && HEX_RE.test(fg) ? fg : null;
}

describe("liftThemeContrast", () => {
  it("mirrors tokens.css --color-code-bg", () => {
    const css = readFileSync(
      resolve(process.cwd(), "src/theme/tokens.css"),
      "utf8",
    );
    const token = /--color-code-bg:\s*(#[0-9a-fA-F]{6})/.exec(css)?.[1];
    expect(token?.toLowerCase()).toBe(CODE_BG);
  });

  it("brings every tokyo-night foreground to AA on the code surface", () => {
    const failingBefore = entriesOf(tokyoNight).filter((entry) => {
      const fg = hexForeground(entry);
      return fg !== null && contrastRatio(fg, CODE_BG) < AA_NORMAL_TEXT;
    });
    // The comment family is the known offender; if the theme ever
    // ships AA-clean, the lift becomes a no-op and this guard notices.
    expect(failingBefore.length).toBeGreaterThan(0);

    const lifted = entriesOf(liftThemeContrast(tokyoNight));
    expect(lifted.length).toBeGreaterThan(50);
    for (const entry of lifted) {
      const fg = hexForeground(entry);
      if (fg === null) continue;
      expect(
        contrastRatio(fg, CODE_BG),
        `${JSON.stringify(entry.scope)} ${fg}`,
      ).toBeGreaterThanOrEqual(AA_NORMAL_TEXT);
    }
  });

  it("keeps passing entries by reference and font styles on lifted ones", () => {
    const original = tokyoNight.tokenColors ?? [];
    const lifted = liftThemeContrast(tokyoNight).tokenColors ?? [];
    expect(lifted).toHaveLength(original.length);
    original.forEach((entry, i) => {
      const liftedEntry = lifted[i];
      if (liftedEntry === undefined) {
        throw new Error(`entry ${String(i)} vanished`);
      }
      const fg = hexForeground(entry);
      if (fg === null || contrastRatio(fg, CODE_BG) >= AA_NORMAL_TEXT) {
        expect(liftedEntry).toBe(entry);
      } else {
        expect(liftedEntry.settings.fontStyle).toBe(entry.settings.fontStyle);
        expect(liftedEntry.scope).toBe(entry.scope);
      }
    });
  });

  it("nudges toward white on dark surfaces and black on light ones", () => {
    expect(liftColor("#ffffff", "#000000")).toBe("#ffffff");
    const onDark = liftColor("#51597d", "#161b28");
    expect(contrastRatio(onDark, "#161b28")).toBeGreaterThanOrEqual(4.5);
    expect(contrastRatio(onDark, "#161b28")).toBeLessThan(5.5);
    const onLight = liftColor("#c0c0c0", "#ffffff");
    expect(contrastRatio(onLight, "#ffffff")).toBeGreaterThanOrEqual(4.5);
  });
});

function cssColorToHex(color: string): string {
  if (color.startsWith("#")) {
    return color;
  }
  const match = /rgb\((\d+), (\d+), (\d+)\)/.exec(color);
  if (match === null) {
    throw new Error(`unparsed color ${color}`);
  }
  return `#${match
    .slice(1, 4)
    .map((v) => Number(v).toString(16).padStart(2, "0"))
    .join("")}`;
}

describe("rendered code comments", () => {
  it("meet AA against the code surface and stay italic", async () => {
    const { content } = await renderMarkdown(
      "```ts\n// a comment\nconst x = 1;\n```",
    );
    const { container } = render(<MemoryRouter>{content}</MemoryRouter>);
    // The innermost colored span is the token; its parent line span
    // carries the same text but no color.
    const tokens = [...container.querySelectorAll<HTMLElement>("pre span")]
      .filter((el) => el.textContent.includes("// a comment"))
      .filter((el) => el.style.color !== "");
    const token = tokens.at(-1);
    if (token === undefined) {
      throw new Error("no colored comment token rendered");
    }
    const hex = cssColorToHex(token.style.color);
    expect(contrastRatio(hex, CODE_BG), hex).toBeGreaterThanOrEqual(
      AA_NORMAL_TEXT,
    );
    expect(token.style.fontStyle).toBe("italic");
  });
});
