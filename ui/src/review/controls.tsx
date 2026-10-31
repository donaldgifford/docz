import type { ComponentPropsWithoutRef } from "react";

import { THEMES, type Appearance, type Theme } from "@/review/appearance";

/*
 * The review page's one button, in docz-site's chrome: mono, sharp
 * corners, the accent for the primary action. Kept here rather than in
 * src/components because nothing else in the site posts anything.
 */
export function ReviewButton({
  variant = "plain",
  className = "",
  ...rest
}: ComponentPropsWithoutRef<"button"> & {
  variant?: "plain" | "primary" | "selected";
}) {
  const look = {
    plain: "border-border-default text-fg-secondary hover:bg-bg-hover",
    primary: "border-accent-border bg-accent-bg text-accent hover:bg-bg-hover",
    selected: "border-accent-border bg-bg-active text-fg-primary",
  }[variant];
  return (
    <button
      type="button"
      className={`border px-[10px] py-[3px] font-mono text-[12.5px] disabled:cursor-not-allowed disabled:opacity-50 ${look} ${className}`}
      {...rest}
    />
  );
}

/*
 * The theme selector and, for a theme with modes, the light/dark
 * toggle. The toggle names the mode it switches to, as an exported
 * page's does; classic has one look in every mode, so it has none.
 */
export function AppearanceControls({ appearance }: { appearance: Appearance }) {
  const { theme, mode, setTheme, toggleMode } = appearance;
  const next = mode === "dark" ? "light" : "dark";
  return (
    <>
      <label className="flex items-center gap-2 font-mono text-[12.5px] text-fg-secondary">
        Theme
        <select
          value={theme}
          onChange={(e) => {
            const value = e.target.value;
            if ((THEMES as readonly string[]).includes(value)) {
              setTheme(value as Theme);
            }
          }}
          className="border border-border-default bg-bg-raised px-[6px] py-[3px] text-fg-primary"
        >
          {THEMES.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
      </label>
      {theme !== "classic" && (
        <ReviewButton
          aria-label={`Switch to ${next} mode`}
          onClick={toggleMode}
        >
          {next === "dark" ? "Dark" : "Light"}
        </ReviewButton>
      )}
    </>
  );
}
