import type { Page } from "@playwright/test";

import { THEMES, THEMES_WITH_MODES } from "../src/theme/__generated__/themes";

/*
 * Every look a reader can pick (IMPL-0030 Phase 3): each pkg/theme
 * theme, and both modes for a theme that has them. Built from the
 * generated list, so a new theme joins every spec that loops over this
 * without an edit here.
 */
export interface Appearance {
  readonly theme: string;
  readonly mode: "light" | "dark" | null;
  readonly label: string;
}

export const APPEARANCES: readonly Appearance[] = THEMES.flatMap(
  (theme): Appearance[] =>
    (THEMES_WITH_MODES as readonly string[]).includes(theme)
      ? [
          { theme, mode: "light", label: `${theme} light` },
          { theme, mode: "dark", label: `${theme} dark` },
        ]
      : [{ theme, mode: null, label: theme }],
);

/**
 * useAppearance stores the reader's pick before the page's first script
 * runs, exactly as the topbar's controls would have on an earlier visit.
 */
export async function useAppearance(
  page: Page,
  appearance: Appearance,
): Promise<void> {
  await page.addInitScript(({ theme, mode }) => {
    window.localStorage.setItem("docz:theme", theme);
    if (mode !== null) {
      window.localStorage.setItem("docz:mode", mode);
    }
  }, appearance);
}
