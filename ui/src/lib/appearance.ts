import {
  useCallback,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";

import { THEMES_WITH_MODES } from "@/theme/__generated__/themes";

/*
 * The appearance shared by docz-site and the review page (DESIGN-0025 §3
 * as amended by IMPL-0030): data-theme and data-mode on <html>, the
 * attributes pkg/theme's token blocks select on. Who picks the theme
 * differs by surface and lives elsewhere: the review page takes the
 * repository's from its session, and the site reads src/lib/theme.ts.
 * The mode is always the reader's: data-mode, remembered under
 * docz:mode, the key every exported page's runtime uses too. With no
 * stored mode the page follows the system, and data-mode stays unset so
 * the theme's prefers-color-scheme block applies.
 */

export type Mode = "light" | "dark";

const MODE_KEY = "docz:mode";
const DARK_QUERY = "(prefers-color-scheme: dark)";

/**
 * hasModes says whether the light/dark toggle means anything on theme:
 * a theme without a dark block has one look in every mode.
 */
export function hasModes(theme: string): boolean {
  return (THEMES_WITH_MODES as readonly string[]).includes(theme);
}

function isMode(value: string | null): value is Mode {
  return value === "light" || value === "dark";
}

/** The stored mode, or null to follow the system. */
export function storedMode(): Mode | null {
  try {
    const value = window.localStorage.getItem(MODE_KEY);
    return isMode(value) ? value : null;
  } catch {
    return null;
  }
}

function saveMode(mode: Mode): void {
  try {
    window.localStorage.setItem(MODE_KEY, mode);
  } catch {
    // Storage is optional: the choice holds for this page only.
  }
}

function writeMode(mode: Mode | null): void {
  const root = document.documentElement;
  if (mode === null) {
    root.removeAttribute("data-mode");
  } else {
    root.setAttribute("data-mode", mode);
  }
}

/** applyTheme sets <html>'s data-theme. */
export function applyTheme(theme: string): void {
  document.documentElement.setAttribute("data-theme", theme);
}

/**
 * applyStoredMode sets <html>'s data-mode from storage. main.tsx calls
 * it before the first render, so the page never paints in the wrong
 * mode.
 */
export function applyStoredMode(): void {
  writeMode(storedMode());
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
  theme: string;
  /** The mode in effect: the stored one, else the system's. */
  mode: Mode;
  toggleMode: () => void;
}

/**
 * useAppearance holds the mode for the session's theme, writes both to
 * <html>, stores a toggled mode, and calls onChange after the attributes
 * move, so a caller that reads tokens (mermaid) reads the new ones; not
 * on mount, where main.tsx already applied them before anything
 * rendered. A change also dispatches docz:mode on the document, as an
 * exported page's runtime does.
 */
export function useAppearance(
  theme: string,
  onChange?: () => void,
): Appearance {
  const [stored, setStored] = useState<Mode | null>(storedMode);
  const system = useSyncExternalStore(subscribeSystem, systemMode);
  const mode = stored ?? system;
  const mounted = useRef(false);

  useEffect(() => {
    applyTheme(theme);
    writeMode(stored);
    document.dispatchEvent(
      new CustomEvent("docz:mode", { detail: { mode, theme } }),
    );
    if (mounted.current) {
      onChange?.();
    }
    mounted.current = true;
  }, [theme, stored, mode, onChange]);

  const toggleMode = useCallback(() => {
    const next: Mode = mode === "dark" ? "light" : "dark";
    saveMode(next);
    setStored(next);
  }, [mode]);

  return { theme, mode, toggleMode };
}
