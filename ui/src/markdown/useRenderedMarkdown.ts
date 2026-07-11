import { useQuery } from "@tanstack/react-query";

import { preprocessDoczMarkdown } from "@/markdown/preprocess";
import { renderMarkdown, type RenderedMarkdown } from "@/markdown/processor";

import type { Document } from "@/api/__generated__/docz-api.schemas";

/** Anything renderable: a stable identity plus a content-address. */
export interface RenderableSource {
  /** Cache identity (doc_id, "repo-index:owner/name", …). */
  id: string;
  /** Content hash — the cache never reprocesses unchanged content. */
  hash: string;
  raw: string;
}

/**
 * Runs the sanitizing pipeline for markdown from the API. Keyed on
 * `(id, hash)` in the query cache, so revisiting a page (or
 * re-rendering the route) never reprocesses unchanged content —
 * `staleTime: Infinity` because a hash-addressed result can't go stale.
 */
export function useRenderedSource(
  source: RenderableSource | undefined,
  options?: { stripLeadingH1?: boolean },
) {
  const stripLeadingH1 = options?.stripLeadingH1 ?? false;
  return useQuery<RenderedMarkdown>({
    enabled: source !== undefined,
    queryKey: ["rendered-markdown", source?.id, source?.hash, stripLeadingH1],
    queryFn: () =>
      renderMarkdown(
        preprocessDoczMarkdown(source?.raw ?? "", { stripLeadingH1 }),
      ),
    staleTime: Infinity,
    // ReactNode trees aren't serializable; keep them out of any future
    // persister and don't try structural sharing on them.
    structuralSharing: false,
  });
}

/** Reader variant: the header renders the structured title, so the
 * markdown's own leading h1 would duplicate it. */
export function useRenderedMarkdown(doc: Document | undefined) {
  return useRenderedSource(
    doc?.raw_md === undefined
      ? undefined
      : { id: doc.doc_id, hash: doc.content_hash, raw: doc.raw_md },
    { stripLeadingH1: true },
  );
}
