import type { Page } from "@playwright/test";

/*
 * The looks the site's specs loop over: docz-site's shell has one look in
 * two modes (DESIGN-0026 §2). The review spec crosses them with every
 * pkg/theme theme, its document column's (DESIGN-0026 §9).
 */
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
