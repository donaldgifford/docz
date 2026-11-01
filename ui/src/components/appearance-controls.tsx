import { hasModes, type Mode } from "@/lib/appearance";
import { THEMES, type ThemeName } from "@/theme/__generated__/themes";

/*
 * docz-site's topbar appearance controls (IMPL-0030 Phase 3): a labelled
 * theme selector over every theme pkg/theme has, and, on a theme with a
 * dark block, the light/dark toggle, which names the mode it switches
 * to, as an exported page's does. The site is the one surface with a
 * selector: its default is the deployment's, and the reader may change
 * it (DESIGN-0025 §3).
 */
export function AppearanceControls({
  theme,
  mode,
  onTheme,
  onToggleMode,
}: {
  theme: ThemeName;
  mode: Mode;
  onTheme: (theme: ThemeName) => void;
  onToggleMode: () => void;
}) {
  const next = mode === "dark" ? "light" : "dark";
  return (
    <>
      <label className="flex items-center gap-2 text-fg-tertiary">
        <span className="sr-only min-[720px]:not-sr-only">Theme</span>
        <select
          value={theme}
          onChange={(e) => {
            const value = THEMES.find((t) => t === e.target.value);
            if (value !== undefined) {
              onTheme(value);
            }
          }}
          className="border border-border-default bg-bg-raised px-[6px] py-[2px] text-fg-primary"
        >
          {THEMES.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
      </label>
      {hasModes(theme) && (
        <button
          type="button"
          aria-label={`Switch to ${next} mode`}
          onClick={onToggleMode}
          className="border border-border-default px-[8px] py-[2px] text-fg-tertiary hover:border-border-strong hover:text-fg-primary"
        >
          {next === "dark" ? "Dark" : "Light"}
        </button>
      )}
    </>
  );
}
