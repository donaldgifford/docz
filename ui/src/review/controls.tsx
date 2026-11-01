import type { ComponentPropsWithoutRef } from "react";

import { type Appearance, hasModes } from "@/lib/appearance";

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
 * The light/dark toggle, on a theme with modes. The toggle names the
 * mode it switches to, as an exported page's does. There is no theme
 * selector: the repository chose the theme (DESIGN-0025 §3).
 */
export function AppearanceControls({ appearance }: { appearance: Appearance }) {
  const { theme, mode, toggleMode } = appearance;
  if (!hasModes(theme)) {
    return null;
  }
  const next = mode === "dark" ? "light" : "dark";
  return (
    <ReviewButton aria-label={`Switch to ${next} mode`} onClick={toggleMode}>
      {next === "dark" ? "Dark" : "Light"}
    </ReviewButton>
  );
}
