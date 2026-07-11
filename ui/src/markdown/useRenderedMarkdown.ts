import { useQuery } from "@tanstack/react-query";

import { preprocessDoczMarkdown } from "@/markdown/preprocess";
import { renderMarkdown, type RenderedMarkdown } from "@/markdown/processor";

import type { Document } from "@/api/__generated__/docz-api.schemas";

/**
 * Runs the sanitizing pipeline for a fetched document. Keyed on
 * `(doc_id, content_hash)` in the query cache, so revisiting a doc (or
 * re-rendering the route) never reprocesses unchanged content —
 * `staleTime: Infinity` because a hash-addressed result can't go stale.
 */
export function useRenderedMarkdown(doc: Document | undefined) {
  const rawMd = doc?.raw_md;
  return useQuery<RenderedMarkdown>({
    enabled: rawMd !== undefined,
    queryKey: ["rendered-markdown", doc?.doc_id, doc?.content_hash],
    queryFn: () =>
      renderMarkdown(
        // The reader header renders the structured title, so the
        // markdown's own leading h1 would duplicate it.
        preprocessDoczMarkdown(rawMd ?? "", { stripLeadingH1: true }),
      ),
    staleTime: Infinity,
    // ReactNode trees aren't serializable; keep them out of any future
    // persister and don't try structural sharing on them.
    structuralSharing: false,
  });
}
