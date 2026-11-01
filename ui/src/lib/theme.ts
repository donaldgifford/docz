import { useCallback, useState } from "react";

import {
  DEFAULT_THEME,
  THEMES,
  type ThemeName,
} from "@/theme/__generated__/themes";

/*
 * docz-site's theme (DESIGN-0025 §3 as amended by IMPL-0030). One site
 * shows many repositories, so it never reads a repository's html.theme:
 * the deployment picks the default and the reader may pick another.
 *
 * The reader's pick, stored under docz:theme, wins; then the runtime
 * default server/serve.ts injects from DOCZ_THEME; then the build-time
 * VITE_THEME (dev and e2e); then paper. Both ends validate, as with nav
 * pins and the mermaid layout: the server whitelisted before injecting,
 * and this module re-validates whatever it reads against the theme list
 * generated from pkg/theme, since the value lands in an attribute every
 * token block selects on. An injected value that fails validation falls
 * through rather than winning. The review page never imports this
 * module: its theme is the repository's.
 */

const THEME_KEY = "docz:theme";

/** Narrow an arbitrary value to a theme pkg/theme has. */
export function parseTheme(value: unknown): ThemeName | undefined {
  if (typeof value !== "string") {
    return undefined;
  }
  const normalized = value.trim().toLowerCase();
  return THEMES.find((theme) => theme === normalized);
}

function runtimeTheme(): ThemeName | undefined {
  if (typeof window === "undefined") {
    return undefined;
  }
  return parseTheme(window.__DOCZ_CONFIG__?.theme);
}

/** The deployment's default: injected, else build-time, else paper. */
export function defaultTheme(): ThemeName {
  return (
    runtimeTheme() ?? parseTheme(import.meta.env.VITE_THEME) ?? DEFAULT_THEME
  );
}

/** The reader's stored pick, else the deployment's default. */
export function storedTheme(): ThemeName {
  try {
    const stored = parseTheme(window.localStorage.getItem(THEME_KEY));
    if (stored !== undefined) {
      return stored;
    }
  } catch {
    // Storage is optional: fall through to the default.
  }
  return defaultTheme();
}

function saveTheme(theme: ThemeName): void {
  try {
    window.localStorage.setItem(THEME_KEY, theme);
  } catch {
    // Storage is optional: the pick holds for this page only.
  }
}

/**
 * useSiteTheme holds the site's theme and stores the reader's pick. The
 * caller passes the theme to useAppearance, which writes it to <html>.
 */
export function useSiteTheme(): [ThemeName, (theme: ThemeName) => void] {
  const [theme, setThemeState] = useState<ThemeName>(storedTheme);
  const setTheme = useCallback((next: ThemeName) => {
    saveTheme(next);
    setThemeState(next);
  }, []);
  return [theme, setTheme];
}
