import type { ReactNode } from "react";

/*
 * Search snippets are UNTRUSTED strings (DESIGN-0001 "Snippets are
 * untrusted"). The only markup honored is the API's literal <em></em>
 * match markers, re-emitted as <mark>; every other character stays
 * literal text that React escapes, so no HTML in a snippet can render
 * or execute. No parsing, no dangerouslySetInnerHTML — an <em> variant
 * with attributes is NOT a marker and renders as visible text.
 */

const EM_MARKER_RE = /(<\/?em>)/;

export function Snippet({ snippet }: { snippet: string }) {
  const nodes: ReactNode[] = [];
  let depth = 0;
  for (const [index, part] of snippet.split(EM_MARKER_RE).entries()) {
    if (part === "<em>") {
      depth += 1;
    } else if (part === "</em>") {
      // Unbalanced closers clamp to zero instead of corrupting state.
      depth = Math.max(0, depth - 1);
    } else if (part !== "") {
      nodes.push(
        depth > 0 ? (
          <mark key={index} className="bg-transparent font-medium text-accent">
            {part}
          </mark>
        ) : (
          part
        ),
      );
    }
  }
  return <>{nodes}</>;
}
