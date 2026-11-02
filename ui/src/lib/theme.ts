import {
  DEFAULT_THEME,
  THEMES,
  type ThemeName,
} from "@/theme/__generated__/themes";

/*
 * The theme docz-site shows documents in (DESIGN-0026 §3). The shell has
 * one look of its own (shell.css); a document sits inside .docz-page in
 * this theme, which is the deployment's: the runtime value server/serve.ts
 * injects from DOCZ_THEME, then the build-time VITE_THEME (dev and e2e),
 * then paper. There is no reader's pick (DESIGN-0026 Open Question 2).
 * Both ends validate, as with nav pins and the mermaid layout: the server
 * whitelisted before injecting, and this module re-validates against the
 * theme list generated from pkg/theme, since the value lands in an
 * attribute page.css selects on. An injected value that fails validation
 * falls through rather than winning. The review page never imports this
 * module: its theme is the repository's.
 */

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

/** The document theme: injected, else build-time, else paper. */
export function defaultTheme(): ThemeName {
  return (
    runtimeTheme() ?? parseTheme(import.meta.env.VITE_THEME) ?? DEFAULT_THEME
  );
}
