import {
  useCallback,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";

/*
 * The appearance shared by docz-site and the review page (DESIGN-0025 §3
 * as amended by IMPL-0030, DESIGN-0026, and DESIGN-0028 §5). The mode is
 * on <html> twice, because two things select on it:
 *
 *   - data-mode, which pkg/theme's document themes, exported pages, and
 *     the review page's document view read. With no stored mode it stays
 *     unset, so each theme's prefers-color-scheme block applies.
 *   - the dark class, which the design system's tokens and parts read.
 *     The package has no prefers-color-scheme block, so with no stored
 *     mode the class follows the system here, through a media-query
 *     listener.
 *
 * Both are written by the one writeMode(), so they cannot disagree.
 * Neither page puts a theme on <html>: a document's theme sits on its
 * own .docz-page. The mode is the reader's, remembered under docz:mode,
 * the key every exported page's runtime uses too.
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

function systemMode(): Mode {
  return typeof window.matchMedia === "function" &&
    window.matchMedia(DARK_QUERY).matches
    ? "dark"
    : "light";
}

/**
 * writeMode puts the mode on <html>: data-mode from the stored mode, or
 * unset to follow the system, and the dark class from the mode in
 * effect, which is the stored one or else the system's.
 */
function writeMode(mode: Mode | null): void {
  const root = document.documentElement;
  if (mode === null) {
    root.removeAttribute("data-mode");
  } else {
    root.setAttribute("data-mode", mode);
  }
  root.classList.toggle("dark", (mode ?? systemMode()) === "dark");
}

let followingSystem = false;

/**
 * followSystem keeps the dark class on the system's scheme while nothing
 * is stored. data-mode needs no listener, because CSS follows the media
 * query by itself; a class does not.
 */
function followSystem(): void {
  if (followingSystem || typeof window.matchMedia !== "function") {
    return;
  }
  followingSystem = true;
  window.matchMedia(DARK_QUERY).addEventListener("change", () => {
    if (storedMode() === null) {
      writeMode(null);
    }
  });
}

/**
 * applyStoredMode sets <html>'s data-mode and dark class from storage
 * and starts following the system. main.tsx calls it before the first
 * render, so the page never paints in the wrong mode.
 */
export function applyStoredMode(): void {
  writeMode(storedMode());
  followSystem();
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

export interface Appearance {
  /** The mode in effect: the stored one, else the system's. */
  mode: Mode;
  toggleMode: () => void;
}

/**
 * useAppearance holds the mode, writes it to <html> (data-mode and the
 * dark class), stores a toggled mode, and calls onChange after both move, so a caller that
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
