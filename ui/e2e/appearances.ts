import type { Page } from "@playwright/test";

import { THEMES, THEMES_WITH_MODES } from "../src/theme/__generated__/themes";

/*
 * The looks a spec loops over. The review page shows a repository's
 * theme, so it runs every pkg/theme theme, in both modes where the theme
 * has them (IMPL-0030 Phase 3); built from the generated list, so a new
 * theme joins without an edit here. docz-site's shell has one look in
 * two modes (DESIGN-0026 §2), so the site's specs run SHELL_MODES.
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

export interface ShellMode {
  readonly mode: "light" | "dark";
  readonly label: string;
}

export const SHELL_MODES: readonly ShellMode[] = [
  { mode: "light", label: "shell light" },
  { mode: "dark", label: "shell dark" },
];

/**
 * useShellMode stores the reader's mode before the page's first script
 * runs, exactly as the topbar's toggle would have on an earlier visit.
 */
export async function useShellMode(
  page: Page,
  shell: ShellMode,
): Promise<void> {
  await page.addInitScript((mode) => {
    window.localStorage.setItem("docz:mode", mode);
  }, shell.mode);
}
