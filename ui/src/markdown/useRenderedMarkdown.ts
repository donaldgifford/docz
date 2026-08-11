import { useQuery } from "@tanstack/react-query";

import { fnv1a } from "@/lib/colors";
import { preprocessDoczMarkdown } from "@/markdown/preprocess";
import { renderMarkdown, type RenderedMarkdown } from "@/markdown/processor";

import type { Document } from "@/api/__generated__/docz-api.schemas";
import type { XrefResolver } from "@/markdown/xrefs";

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
/**
 * Cheap fingerprint of the link maps so they can key the cache: both
 * the doc-id keys (xrefs) and the path keys (relative-link resolution)
 * — either map growing must re-render the body exactly once.
 */
function linkFingerprint(
  xrefs: XrefResolver | undefined,
  paths: ReadonlyMap<string, string> | undefined,
): number | null {
  if (xrefs === undefined && paths === undefined) {
    return null;
  }
  const idKeys = xrefs === undefined ? [] : [...xrefs.keys()].sort();
  const pathKeys = paths === undefined ? [] : [...paths.keys()].sort();
  return fnv1a(`${idKeys.join(",")}|${pathKeys.join(",")}`);
}

export function useRenderedSource(
  source: RenderableSource | undefined,
  options?: {
    stripLeadingH1?: boolean;
    xrefs?: XrefResolver;
    /** `RepoDocIndex.byPath` — the relative-link resolution whitelist. */
    paths?: ReadonlyMap<string, string>;
    /** The source's own repo path; relative hrefs resolve against its
     * directory. Resolution needs both `paths` and `base`. */
    base?: string;
  },
) {
  const stripLeadingH1 = options?.stripLeadingH1 ?? false;
  const xrefs = options?.xrefs;
  const paths = options?.paths;
  const base = options?.base;
  return useQuery<RenderedMarkdown>({
    enabled: source !== undefined,
    queryKey: [
      "rendered-markdown",
      source?.id,
      source?.hash,
      stripLeadingH1,
      linkFingerprint(xrefs, paths),
      base ?? null,
    ],
    queryFn: () =>
      renderMarkdown(
        preprocessDoczMarkdown(source?.raw ?? "", { stripLeadingH1 }),
        {
          xrefs,
          links:
            paths === undefined || base === undefined
              ? undefined
              : { base, byPath: paths },
        },
      ),
    staleTime: Infinity,
    // ReactNode trees aren't serializable; keep them out of any future
    // persister and don't try structural sharing on them.
    structuralSharing: false,
  });
}

/** Reader variant: the header renders the structured title, so the
 * markdown's own leading h1 would duplicate it. Relative links resolve
 * against the doc's own `path`. */
export function useRenderedMarkdown(
  doc: Document | undefined,
  links?: { xrefs?: XrefResolver; paths?: ReadonlyMap<string, string> },
) {
  return useRenderedSource(
    doc?.raw_md === undefined
      ? undefined
      : { id: doc.doc_id, hash: doc.content_hash, raw: doc.raw_md },
    { stripLeadingH1: true, base: doc?.path, ...links },
  );
}
