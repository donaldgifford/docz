import { useEffect, useRef, useState } from "react";

import type { Mermaid, MermaidConfig } from "mermaid";

import { mermaidLayout } from "@/lib/mermaidLayout";

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
 * A THIRD setting keeps the second one true — see MERMAID_SECURE_KEYS.
 *
 * Both survive mermaid 12 unchanged: `securityLevel` keeps its four
 * levels, and the root `htmlLabels` now explicitly OUTRANKS every
 * per-diagram copy (`flowchart.htmlLabels` and friends are deprecated
 * in its favour), which makes one `false` cover diagram types this file
 * never names. The nested one stays set anyway — it costs a line and
 * the precedence rule is upstream's to change.
 *
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
    mod.default.initialize(mermaidInitConfig());
    return mod.default;
  });
  return mermaidPromise;
}

/*
 * `secure` is the list of config keys a diagram's own YAML front matter
 * is forbidden to set. Mermaid's default list covers `securityLevel`
 * but NOT `htmlLabels`, and the gap is real rather than theoretical:
 * audited against the installed 11.16.0 and proven by test in
 * `mermaid-config.test.ts`, a hostile `config:` block otherwise flips
 * BOTH the global flag and the nested `flowchart.htmlLabels`,
 * re-opening the exact vector strict mode leaves open. Mermaid's
 * directive sanitizer recurses into nested config objects, so the ONE
 * top-level entry covers both paths. `secure` is itself always secure,
 * so a document cannot unset any of this.
 *
 * Upstream's own defaults are restated here rather than added to,
 * because mermaid currently UNIONS this array with its defaults and
 * that is an implementation detail — under clobber semantics a
 * one-element array would silently drop `securityLevel` from the
 * protected set. The EFFECTIVE list is what the test asserts on, and
 * it also fails if a future mermaid protects a key this list omits.
 */
export const MERMAID_SECURE_KEYS = [
  // Mermaid's defaults as of 11.16.0, unchanged in 12.0.0.
  "secure",
  "securityLevel",
  "startOnLoad",
  "maxTextSize",
  "suppressErrorRendering",
  "maxEdges",
  // Ours.
  "htmlLabels",
] as const;

/*
 * v12 also changed the default `look` from `classic` to `neo`, which
 * rounds node corners, thickens strokes, and adds a drop shadow. That
 * is a separate change riding along with the layout one, and this site
 * has a stated position on it: the radius scale in tokens.css is wiped
 * — sharp corners everywhere, `rounded-pill` the only exception — so
 * `neo` would leave diagrams the one surface with rounded boxes.
 * Pinned rather than inherited, so the appearance is a decision on the
 * record instead of a side effect of a dependency bump. Flipping to
 * `neo` is this one line, and the specimen's Mermaid section is where
 * to judge it.
 */
const MERMAID_LOOK = "classic";

/**
 * The exact config we ship — exported so tests exercise it, not a copy.
 *
 * `layout` is resolved per page load (IMPL-0006 OQ-1): ELK by default,
 * `dagre` when the deployment says so through DOCZ_MERMAID_LAYOUT. The
 * module memoizes `mermaidPromise`, so this runs once, which is right —
 * the value cannot change without a new document. ELK arrives as its
 * own ~436 KB gzipped chunk behind the same dynamic import as mermaid
 * itself, and mermaid only fetches it when the layout is `elk`; neither
 * may ever become eager, and the chunk assertion in
 * e2e/rendering.spec.ts has to match the ELK filename, which does NOT
 * contain "mermaid".
 */
export function mermaidInitConfig(): MermaidConfig {
  return {
    startOnLoad: false,
    theme: "base",
    layout: mermaidLayout(),
    look: MERMAID_LOOK,
    securityLevel: "strict",
    htmlLabels: false,
    flowchart: { htmlLabels: false },
    secure: [...MERMAID_SECURE_KEYS],
    themeVariables: mermaidThemeFromTokens(),
  };
}

/*
 * Minimal documented variable set, read from the live tokens so
 * diagrams follow tokens.css; fallbacks keep jsdom/tests valid. Every
 * key here was re-checked against mermaid 12's theme-base — an unknown
 * variable breaks `mermaid.render` SILENTLY, so the failure mode is a
 * blank figure rather than an error, and the test asserts each one
 * survives the merge.
 *
 * `nodeBorder` is load-bearing beyond its own color in v12. The `neo`
 * look strokes nodes with a gradient whenever the theme sets
 * `useGradient` — and `base` does. Theme.calculate turns it back off
 * precisely when the overrides carry `nodeBorder` and not
 * `useGradient`, which is this map. MERMAID_LOOK keeps us off `neo`
 * today, but the two guards are independent: drop `nodeBorder` and a
 * later look change grows gradients across every diagram, against the
 * monochrome policy.
 */
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
    // Edge labels sit on the diagram surface, not the page: give them
    // the figure's own background so arrows don't read through them.
    edgeLabelBackground: bgRaised,
    // ASCII family name only — passing the full font stack with quotes
    // breaks mermaid.render silently (carried from the old portal).
    fontFamily: "monospace",
    fontSize: "14px",
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
