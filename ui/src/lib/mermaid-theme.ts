/*
 * The mermaid theme generation (IMPL-0028 Phase 6, IMPL-0030). A page
 * that changes its theme or mode calls refreshMermaidTheme after the
 * attributes move: mermaid is initialized again from the new tokens the
 * next time it renders, the SVG cache (keyed by generation) misses, and
 * every mounted block renders again.
 *
 * It lives apart from src/markdown/mermaid-block.tsx so the site's
 * topbar, which is in the eager bundle, can signal a redraw without
 * importing the markdown pipeline (scripts/bundle-budget.ts).
 */

let generation = 0;
const listeners = new Set<() => void>();

/** subscribeMermaidTheme calls onChange after every refresh. */
export function subscribeMermaidTheme(onChange: () => void): () => void {
  listeners.add(onChange);
  return () => {
    listeners.delete(onChange);
  };
}

/** The current generation; it moves on every refresh. */
export function mermaidThemeGeneration(): number {
  return generation;
}

/** refreshMermaidTheme redraws every diagram from the current tokens. */
export function refreshMermaidTheme(): void {
  generation += 1;
  for (const fn of listeners) {
    fn();
  }
}

/** Test-only: back to generation zero. */
export function _resetMermaidTheme(): void {
  generation = 0;
}
