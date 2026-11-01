import {
  useEffect,
  useEffectEvent,
  useLayoutEffect,
  useMemo,
  useRef,
} from "react";
import { createRoot, type Root } from "react-dom/client";
import { useNavigate, useSearchParams } from "react-router";

import { accentProperties } from "@/components/doc-surface";
import { defaultTheme } from "@/lib/theme";
import { MermaidBlock } from "@/markdown/mermaid-block";

import "@/theme/__generated__/page.css";
// pkg/theme's runtime, the one every exported page runs. It starts itself
// only on a page whose <html> carries data-doc, which the site's never
// does, so importing it defines window.docz and nothing else.
import "../../../pkg/theme/runtime.js";

/** The slice of pkg/theme/runtime.js's global the pane calls. */
interface DoczRuntime {
  mount: (
    scope: Element,
    opts: {
      hash: boolean;
      tab?: string;
      onTab?: (tab: string) => void;
    },
  ) => { showTab: (tab: string) => void; unmount: () => void };
}

declare global {
  interface Window {
    docz?: DoczRuntime;
  }
}

/** A link the server resolved to a site route: same origin, a path. */
function siteRoute(anchor: HTMLAnchorElement): string | undefined {
  const href = anchor.getAttribute("href");
  if (href === null || !href.startsWith("/") || href.startsWith("//")) {
    return undefined;
  }
  return anchor.target === "" ? href : undefined;
}

/**
 * DocPane is a document's docz-html fragment in the reader (DESIGN-0026
 * §7): docz-api renders it, and the pane injects it as it came. That is
 * sound because pkg/export/html escapes text, rebuilds raw HTML from its
 * allow-list, and is fuzzed against scripts, handlers, and unsafe URLs;
 * the XSS suite here holds the fragment path to the same bar.
 *
 * After it lands the pane renders the fragment in the deployment's
 * document theme, with the type's colour as its accent (DESIGN-0026 §8), mounts pkg/theme's runtime inside it with the tab kept
 * in ?tab= instead of the hash, draws each pre.mermaid with the site's
 * own mermaid, and routes clicks on links the server resolved to site
 * routes through the router.
 */
export function DocPane({ html, accent }: { html: string; accent?: string }) {
  const ref = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const tab = params.get("tab") ?? undefined;
  // The tab at mount time, read from the effect without making the tab a
  // reason to remount; later moves go through showTab below.
  const initialTab = useEffectEvent(() => tab);
  // A tab the reader opens goes into ?tab=, replacing the entry. The tab
  // the runtime opens while mounting is not the reader's, so a plain
  // document URL stays plain.
  const mounting = useRef(false);
  const keepTab = useEffectEvent((next: string) => {
    if (mounting.current || next === tab) {
      return;
    }
    setParams(
      (prev) => {
        const out = new URLSearchParams(prev);
        out.set("tab", next);
        return out;
      },
      { replace: true },
    );
  });
  const showTab = useRef<((tab: string) => void) | undefined>(undefined);
  // React 19 compares this prop by identity and rewrites innerHTML on any
  // new object, which would wipe what the runtime and mermaid did to the
  // fragment on every render; one object per fragment keeps it in place.
  const inner = useMemo(() => ({ __html: html }), [html]);

  useLayoutEffect(() => {
    const el = ref.current?.firstElementChild;
    if (el === null || el === undefined || window.docz === undefined) {
      return;
    }
    el.setAttribute("data-theme", defaultTheme());
    if (accent !== undefined) {
      for (const [name, value] of Object.entries(accentProperties(accent))) {
        (el as HTMLElement).style.setProperty(name, value);
      }
    }

    mounting.current = true;
    const mounted = window.docz.mount(el, {
      hash: false,
      tab: initialTab(),
      onTab: keepTab,
    });
    mounting.current = false;
    showTab.current = mounted.showTab;

    const roots: Root[] = [];
    for (const pre of el.querySelectorAll("pre.mermaid")) {
      const host = document.createElement("div");
      pre.replaceWith(host);
      const root = createRoot(host);
      root.render(<MermaidBlock source={pre.textContent} />);
      roots.push(root);
    }

    return () => {
      mounted.unmount();
      showTab.current = undefined;
      // Unmount after this commit: React refuses to unmount a root while
      // it is rendering.
      queueMicrotask(() => {
        for (const root of roots) root.unmount();
      });
    };
  }, [html, accent]);

  // Back and forward move ?tab= without a remount.
  useEffect(() => {
    if (tab !== undefined) {
      showTab.current?.(tab);
    }
  }, [tab]);

  return (
    // The fragment's own buttons and links are the controls; the wrapper
    // only delegates clicks on site routes to the router.
    // eslint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-static-element-interactions
    <div
      ref={ref}
      data-testid="doc-pane"
      onClick={(e) => {
        if (
          e.defaultPrevented ||
          e.button !== 0 ||
          e.metaKey ||
          e.ctrlKey ||
          e.shiftKey ||
          e.altKey
        ) {
          return;
        }
        const anchor = (e.target as Element).closest("a");
        const to = anchor === null ? undefined : siteRoute(anchor);
        if (to !== undefined) {
          e.preventDefault();
          void navigate(to);
        }
      }}
      // The one sanctioned injection of server HTML: see the comment above.
      dangerouslySetInnerHTML={inner}
    />
  );
}
