import {
  useCallback,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";

/*
 * The appearance shared by docz-site and the review page (DESIGN-0025 §3
 * as amended by IMPL-0030 and DESIGN-0026): data-mode on <html>, the
 * attribute the shell's and the document themes' dark blocks select on.
 * Neither page puts a theme on <html>: both wear the shell (shell.css),
 * and a document's theme sits on its own .docz-page. The mode is the
 * reader's: data-mode, remembered under docz:mode, the key every
 * exported page's runtime uses too. With no stored mode the page follows
 * the system, and data-mode stays unset so each prefers-color-scheme
 * block applies.
 */

export type Mode = "light" | "dark";

const MODE_KEY = "docz:mode";
const DARK_QUERY = "(prefers-color-scheme: dark)";

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
  /** The mode in effect: the stored one, else the system's. */
  mode: Mode;
  toggleMode: () => void;
}

/**
 * useAppearance holds the mode, writes it to <html>, stores a toggled
 * mode, and calls onChange after the attribute moves, so a caller that
 * reads tokens (mermaid) reads the new ones; not on mount, where
 * main.tsx already applied it before anything rendered. A change also
 * dispatches docz:mode on the document, as an exported page's runtime
 * does.
 */
export function useAppearance(onChange?: () => void): Appearance {
  const [stored, setStored] = useState<Mode | null>(storedMode);
  const system = useSyncExternalStore(subscribeSystem, systemMode);
  const mode = stored ?? system;
  const mounted = useRef(false);

  useEffect(() => {
    writeMode(stored);
    document.dispatchEvent(new CustomEvent("docz:mode", { detail: { mode } }));
    if (mounted.current) {
      onChange?.();
    }
    mounted.current = true;
  }, [stored, mode, onChange]);

  const toggleMode = useCallback(() => {
    const next: Mode = mode === "dark" ? "light" : "dark";
    saveMode(next);
    setStored(next);
  }, [mode]);

  return { mode, toggleMode };
}
