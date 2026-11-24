import type { ThemeRegistrationRaw } from "shiki/core";

/*
 * Lifts a Shiki theme's token colors to WCAG AA (4.5:1) against the
 * code-block surface. Code the site renders follows the page
 * (DESIGN-0028 §9): it is highlighted in github-light and github-dark
 * at once, on the design system's --code-bg, and CSS picks the set
 * from the `dark` class. Each theme has foregrounds under 4.5:1 on
 * its mode's surface (github-light's comment grey is one), which
 * full-rule axe flags as a serious violation on every block that has
 * one. Rather than fork a theme, each failing foreground is nudged
 * toward the far end of the luminance scale until it clears the bar;
 * passing colors and every font style are untouched, so the theme
 * still reads as itself.
 *
 * CODE_BG is --code-bg in each mode, in hex because the lift is
 * arithmetic on channels. theme-contrast.test.tsx pins both to the
 * package's tokens, so a bump that moves the surface fails there and
 * the lift never runs against a colour the page no longer has.
 */
export const CODE_BG = { light: "#f6f7f9", dark: "#17191c" } as const;
export type CodeMode = keyof typeof CODE_BG;
export const AA_NORMAL_TEXT = 4.5;

const HEX_RE = /^#[0-9a-f]{6}$/i;

type RawSetting = NonNullable<ThemeRegistrationRaw["tokenColors"]>[number];

export function liftThemeContrast(
  theme: ThemeRegistrationRaw,
  bg: string,
): ThemeRegistrationRaw {
  const lift = (entries: RawSetting[]): RawSetting[] =>
    entries.map((entry) => {
      const fg = entry.settings.foreground;
      if (typeof fg !== "string" || !HEX_RE.test(fg)) {
        return entry;
      }
      const lifted = liftColor(fg, bg);
      return lifted === fg
        ? entry
        : { ...entry, settings: { ...entry.settings, foreground: lifted } };
    });
  return {
    ...theme,
    ...(Array.isArray(theme.tokenColors)
      ? { tokenColors: lift(theme.tokenColors) }
      : {}),
    ...(Array.isArray(theme.settings)
      ? { settings: lift(theme.settings) }
      : {}),
  };
}

/** Returns `fg` unchanged when it already meets `min` on `bg`; otherwise
 *  the nearest color along the line toward white (dark surfaces) or
 *  black (light surfaces) that does, as lowercase hex. */
export function liftColor(
  fg: string,
  bg: string,
  min: number = AA_NORMAL_TEXT,
): string {
  if (contrastRatio(fg, bg) >= min) {
    return fg;
  }
  const target = luminance(bg) < 0.5 ? 255 : 0;
  let [r, g, b] = channels(fg);
  let hex = fg.toLowerCase();
  for (let step = 0; step < 60 && contrastRatio(hex, bg) < min; step += 1) {
    r += (target - r) * 0.05;
    g += (target - g) * 0.05;
    b += (target - b) * 0.05;
    hex = toHex(r, g, b);
  }
  return hex;
}

export function contrastRatio(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return ((hi ?? 0) + 0.05) / ((lo ?? 0) + 0.05);
}

function channels(hex: string): [number, number, number] {
  return [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16)) as [
    number,
    number,
    number,
  ];
}

function luminance(hex: string): number {
  const [r, g, b] = channels(hex).map((v) => {
    const s = v / 255;
    return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
  });
  return 0.2126 * (r ?? 0) + 0.7152 * (g ?? 0) + 0.0722 * (b ?? 0);
}

function toHex(r: number, g: number, b: number): string {
  return `#${[r, g, b]
    .map((v) =>
      Math.round(Math.min(255, Math.max(0, v)))
        .toString(16)
        .padStart(2, "0"),
    )
    .join("")}`;
}
