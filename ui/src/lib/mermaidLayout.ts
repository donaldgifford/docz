/*
 * Diagram layout engine (IMPL-0006 OQ-1). mermaid 12 made ELK the
 * default and this site agrees, but a deployment that wants dagre back
 * must be able to say so without rebuilding the image. The runtime
 * config injected by server/serve.ts (DOCZ_MERMAID_LAYOUT) wins; the
 * build-time VITE_MERMAID_LAYOUT is the dev/e2e fallback; else ELK.
 *
 * Both ends validate, as with nav pins and auth providers: the server
 * whitelisted before injecting, and this module re-validates whatever
 * it reads. Neither end trusts the other. Here the value is handed
 * straight to `mermaid.initialize`, so an unvalidated string would be a
 * config-injection surface on the one library that renders untrusted
 * document text.
 *
 * Unlike nav pins, an injected value that fails validation is NOT
 * authoritative — it falls through. There is no "the deployment chose
 * nothing" case to respect: the set has two members and the server
 * never emits anything else, so a bad value means something is wrong,
 * not that something was chosen.
 */

const LAYOUTS = ["dagre", "elk"] as const;

export type MermaidLayout = (typeof LAYOUTS)[number];

export const DEFAULT_MERMAID_LAYOUT: MermaidLayout = "elk";

/** Narrow an arbitrary value to one of the two supported layouts. */
export function parseMermaidLayout(value: unknown): MermaidLayout | undefined {
  if (typeof value !== "string") {
    return undefined;
  }
  const normalized = value.trim().toLowerCase();
  return LAYOUTS.find((layout) => layout === normalized);
}

function runtimeMermaidLayout(): MermaidLayout | undefined {
  if (typeof window === "undefined") {
    return undefined;
  }
  return parseMermaidLayout(window.__DOCZ_CONFIG__?.mermaidLayout);
}

export function mermaidLayout(): MermaidLayout {
  return (
    runtimeMermaidLayout() ??
    parseMermaidLayout(import.meta.env.VITE_MERMAID_LAYOUT) ??
    DEFAULT_MERMAID_LAYOUT
  );
}
