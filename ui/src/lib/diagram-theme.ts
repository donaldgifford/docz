/*
 * The mermaid theme generation (IMPL-0028 Phase 6, IMPL-0030). A page
 * that changes its theme or mode calls refreshMermaidTheme after the
 * attributes move: mermaid is initialized again from the new tokens the
 * next time it renders, the SVG cache (keyed by generation) misses, and
 * every mounted block renders again.
 *
 * It lives apart from src/markdown/mermaid-block.tsx so the site's
 * shell, which is in the eager bundle, can signal a redraw without
 * importing the markdown pipeline (scripts/bundle-budget.ts).
 *
 * The file is not named for mermaid on purpose. The bundler can emit
 * it as a chunk of its own, named for the module and preloaded on
 * every page, and e2e/rendering.spec.ts fails a diagram-free page on
 * any request whose URL says "mermaid": that is how it catches the
 * library loading where it should not.
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
