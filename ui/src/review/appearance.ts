import { useCallback, useEffect, useState, useSyncExternalStore } from "react";

/*
 * The review page's theme and mode (DESIGN-0025 §3, IMPL-0028 Phase 6).
 * Both live on <html> as data-theme and data-mode, the attributes
 * pkg/theme's token blocks select on, and both are remembered under the
 * keys an exported page's runtime uses: docz:mode is shared with every
 * page docz export html writes, and docz:theme is the review page's own.
 * With no stored mode the page follows the system, and data-mode stays
 * unset so the theme's prefers-color-scheme block applies.
 */

export const THEMES = ["paper", "classic"] as const;
export type Theme = (typeof THEMES)[number];
export type Mode = "light" | "dark";

const THEME_KEY = "docz:theme";
const MODE_KEY = "docz:mode";
const DARK_QUERY = "(prefers-color-scheme: dark)";

function load(key: string): string | null {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function save(key: string, value: string): void {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    // Storage is optional: the choice holds for this page only.
  }
}

function isTheme(value: string | null): value is Theme {
  return (THEMES as readonly (string | null)[]).includes(value);
}

function isMode(value: string | null): value is Mode {
  return value === "light" || value === "dark";
}

/** The stored theme, else paper. */
export function storedTheme(): Theme {
  const value = load(THEME_KEY);
  return isTheme(value) ? value : "paper";
}

/** The stored mode, or null to follow the system. */
export function storedMode(): Mode | null {
  const value = load(MODE_KEY);
  return isMode(value) ? value : null;
}

/**
 * applyStoredAppearance sets <html>'s attributes from storage. main.tsx
 * calls it before the first render, so the page never paints in the
 * wrong theme.
 */
export function applyStoredAppearance(): void {
  const root = document.documentElement;
  root.setAttribute("data-theme", storedTheme());
  const mode = storedMode();
  if (mode === null) {
    root.removeAttribute("data-mode");
  } else {
    root.setAttribute("data-mode", mode);
  }
}

function subscribeSystem(onChange: () => void): () => void {
  if (typeof window.matchMedia !== "function") {
    return () => undefined;
  }
  const query = window.matchMedia(DARK_QUERY);
  query.addEventListener("change", onChange);
  return () => {
    query.removeEventListener("change", onChange);
  };
}

function systemMode(): Mode {
  return typeof window.matchMedia === "function" &&
    window.matchMedia(DARK_QUERY).matches
    ? "dark"
    : "light";
}

export interface Appearance {
  theme: Theme;
  /** The mode in effect: the stored one, else the system's. */
  mode: Mode;
  setTheme: (theme: Theme) => void;
  toggleMode: () => void;
}

/**
 * useAppearance holds the theme and mode, writes them to <html> and to
 * storage, and calls onChange after the attributes move, so a caller
 * that reads tokens (mermaid) reads the new ones. A mode change also
 * dispatches docz:mode on the document, as an exported page's runtime
 * does.
 */
export function useAppearance(onChange?: () => void): Appearance {
  const [theme, setThemeState] = useState<Theme>(storedTheme);
  const [stored, setStored] = useState<Mode | null>(storedMode);
  const system = useSyncExternalStore(subscribeSystem, systemMode);
  const mode = stored ?? system;

  useEffect(() => {
    const root = document.documentElement;
    root.setAttribute("data-theme", theme);
    if (stored === null) {
      root.removeAttribute("data-mode");
    } else {
      root.setAttribute("data-mode", stored);
    }
    document.dispatchEvent(
      new CustomEvent("docz:mode", { detail: { mode, theme } }),
    );
    onChange?.();
  }, [theme, stored, mode, onChange]);

  const setTheme = useCallback((next: Theme) => {
    save(THEME_KEY, next);
    setThemeState(next);
  }, []);

  const toggleMode = useCallback(() => {
    const next: Mode = mode === "dark" ? "light" : "dark";
    save(MODE_KEY, next);
    setStored(next);
  }, [mode]);

  return { theme, mode, setTheme, toggleMode };
}
