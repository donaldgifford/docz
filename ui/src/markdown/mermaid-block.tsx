import { useEffect, useRef, useState } from "react";

import type { Mermaid } from "mermaid";

/*
 * Client-side mermaid rendering (IMPL-0002 Phase 4, OQ-1a).
 *
 * MarkdownPre routes `pre[data-mermaid-source]` here; the marker only
 * ever fires for real ```mermaid fences (document HTML can't smuggle
 * the attribute past sanitize). The ~700 KB mermaid library loads via
 * dynamic import on first mount only — it must NEVER appear in the
 * eager graph (bundle budget) or even the markdown chunk.
 *
 * SECURITY — the ONE sanctioned innerHTML in this codebase: the SVG
 * string comes from mermaid.render(), never from document HTML.
 * TWO settings carry the guarantee, verified in e2e:
 *   - securityLevel "strict": click directives disabled, label HTML
 *     DOMPurify'd. Strict ALONE is not enough — purified-but-real
 *     elements (an <img src> tracking pixel) still land in
 *     foreignObject labels.
 *   - htmlLabels false (global + flowchart): labels render as SVG
 *     <text>, so hostile markup in a node label stays literal text —
 *     no element ever materializes from document text.
 * Do not copy this pattern elsewhere and do not relax either setting;
 * keep the hostile-source rows in the XSS suite and e2e green when
 * touching this. (rfc-site's "strict doesn't render" note described
 * the `sandbox` iframe mode, not strict.)
 *
 * Lessons carried from rfc-site's hydrator: SVG cache keyed by source
 * (StrictMode double-mount flashes the source text otherwise) and the
 * MINIMAL documented themeVariables set — extra variables break
 * mermaid.render silently. Render failure keeps the source visible as
 * a plain code block; a blank box is never an outcome.
 */

const SVG_CACHE = new Map<string, string>();
let mermaidPromise: Promise<Mermaid> | undefined;
let renderSeq = 0;

function getMermaid(): Promise<Mermaid> {
  mermaidPromise ??= import("mermaid").then((mod) => {
    mod.default.initialize({
      startOnLoad: false,
      theme: "base",
      securityLevel: "strict",
      htmlLabels: false,
      flowchart: { htmlLabels: false },
      themeVariables: mermaidThemeFromTokens(),
    });
    return mod.default;
  });
  return mermaidPromise;
}

// Minimal documented v11 variable set, read from the live tokens so
// diagrams follow tokens.css; fallbacks keep jsdom/tests valid.
export function mermaidThemeFromTokens(): Record<string, string> {
  const style = getComputedStyle(document.documentElement);
  const read = (name: string, fallback: string): string => {
    const value = style.getPropertyValue(name).trim();
    return value.length > 0 ? value : fallback;
  };
  const bgRaised = read("--color-bg-raised", "#121722");
  const bgElevated = read("--color-bg-elevated", "#181e2b");
  const border = read("--color-border-strong", "#34405a");
  const fgPrimary = read("--color-fg-primary", "#e8ebf0");
  const fgTertiary = read("--color-fg-tertiary", "#8a92a5");
  return {
    primaryColor: bgElevated,
    primaryBorderColor: border,
    primaryTextColor: fgPrimary,
    secondaryColor: bgRaised,
    secondaryBorderColor: border,
    tertiaryColor: bgRaised,
    tertiaryBorderColor: border,
    mainBkg: bgElevated,
    nodeBorder: border,
    lineColor: fgTertiary,
    clusterBkg: bgRaised,
    clusterBorder: border,
    titleColor: fgPrimary,
    fontFamily: "monospace",
    fontSize: "13px",
  };
}

interface RenderState {
  source: string;
  svg: string | undefined;
  failed: boolean;
}

export function MermaidBlock({
  source,
  caption,
}: {
  source: string;
  caption?: string;
}) {
  const [state, setState] = useState<RenderState>(() => ({
    source,
    svg: SVG_CACHE.get(source),
    failed: false,
  }));
  // Source changed under the same mount — adjust during render.
  if (state.source !== source) {
    setState({ source, svg: SVG_CACHE.get(source), failed: false });
  }
  const hostRef = useRef<HTMLDivElement>(null);

  const needsRender = state.svg === undefined && !state.failed;
  useEffect(() => {
    if (!needsRender) {
      return;
    }
    // `as boolean` keeps no-unnecessary-condition from narrowing the
    // closure-mutated flag to its initial literal.
    let cancelled = false as boolean;
    void (async () => {
      try {
        const mermaid = await getMermaid();
        renderSeq += 1;
        const { svg } = await mermaid.render(
          `docz-mermaid-${String(renderSeq)}`,
          source,
        );
        SVG_CACHE.set(source, svg);
        if (!cancelled) {
          setState({ source, svg, failed: false });
        }
      } catch {
        if (!cancelled) {
          setState({ source, svg: undefined, failed: true });
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [needsRender, source]);

  useEffect(() => {
    if (state.svg !== undefined && hostRef.current !== null) {
      // The documented exception — see the module comment.
      hostRef.current.innerHTML = state.svg;
    }
  }, [state.svg]);

  if (state.svg === undefined) {
    // Loading or failed: the source stays visible, never a blank box.
    /* eslint-disable jsx-a11y/no-noninteractive-tabindex --
       same WAI scrollable-region treatment as MarkdownPre */
    return (
      <pre
        role="region"
        aria-label="mermaid diagram source"
        tabIndex={0}
        data-mermaid-fallback={state.failed ? "failed" : "loading"}
      >
        <code>{source}</code>
      </pre>
    );
    /* eslint-enable jsx-a11y/no-noninteractive-tabindex */
  }

  const firstLine = source.split("\n")[0]?.trim() ?? "";
  return (
    <figure className="mermaid-figure">
      <div
        ref={hostRef}
        role="img"
        aria-label={caption ?? `mermaid diagram: ${firstLine}`}
      />
      {caption !== undefined && <figcaption>{caption}</figcaption>}
    </figure>
  );
}

/** Test-only: reset module state between suites. */
export function _resetMermaidBlock(): void {
  SVG_CACHE.clear();
  mermaidPromise = undefined;
}
