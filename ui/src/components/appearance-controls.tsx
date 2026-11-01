import type { Mode } from "@/lib/appearance";

/*
 * docz-site's light/dark toggle (DESIGN-0026 §2). The shell has one look
 * in two modes, and the toggle names the mode it switches to, as an
 * exported page's does; there is no theme selector (DESIGN-0026 Open
 * Question 2). The mode is the one a document inside .docz-page follows
 * too.
 */
export function AppearanceControls({
  mode,
  onToggleMode,
}: {
  mode: Mode;
  onToggleMode: () => void;
}) {
  const next = mode === "dark" ? "light" : "dark";
  return (
    <button
      type="button"
      aria-label={`Switch to ${next} mode`}
      onClick={onToggleMode}
      className="border border-border-default px-[8px] py-[2px] text-fg-tertiary hover:border-border-strong hover:text-fg-primary rounded-sm"
    >
      {next === "dark" ? "Dark" : "Light"}
    </button>
  );
}
