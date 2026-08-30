import { useQueries } from "@tanstack/react-query";
import { useMemo } from "react";

import {
  getListDocsQueryOptions,
  useGetRepo,
  useListRepoPages,
} from "@/api/__generated__/docz-api";
import { apiConfig } from "@/lib/apiConfig";
import { pageSourceKeys } from "@/lib/pagePaths";
import { arr } from "@/lib/wire";

import type { XrefResolver } from "@/markdown/xrefs";

const FIVE_MINUTES = 5 * 60_000;

/**
 * Every link target in a repo, mapped to its SPA href (DESIGN-0002
 * Component 2; pages per DESIGN-0004 Component 3): `byId` keys
 * UPPERCASED doc ids — the xref resolver for doc-id tokens in rendered
 * bodies — and `byPath` keys ingested repo-relative file paths, the
 * whitelist relative markdown links resolve through. Since IMPL-0005
 * `byPath` also carries published pages under their RECONSTRUCTED
 * source paths (both README/index keys for a directory page), so docs
 * and pages cross-link both directions. Hrefs are always built from
 * API data, never from document text.
 */
export interface RepoDocIndex {
  /** UPPERCASED doc_id -> SPA href ("/owner/name/type/DOC-0001"). */
  byId: XrefResolver;
  /** Repo-relative source path -> SPA href (doc reader or page). */
  byPath: ReadonlyMap<string, string>;
}

/**
 * Both maps come from getRepo's type set plus one listDocs per type
 * (the same queries the repo nav runs, so the cache is usually warm),
 * plus one listRepoPages gated on the repo's `api:` block — a repo
 * without the block never fires the pages request. Undefined until
 * every list resolves, so a body is linkified at most once more after
 * first paint.
 */
export function useRepoDocIndex(
  owner: string,
  name: string,
): RepoDocIndex | undefined {
  const repoQuery = useGetRepo(owner, name);
  const detail =
    repoQuery.data?.status === 200 ? repoQuery.data.data : undefined;
  const types = detail === undefined ? undefined : arr(detail.types);
  // apiConfig returns a fresh object; memo on the stable detail so the
  // map memo below doesn't rebuild every render.
  const cfg = useMemo(() => apiConfig(detail?.config_snapshot), [detail]);

  const docLists = useQueries({
    queries: (types ?? []).map((docType) => ({
      ...getListDocsQueryOptions(owner, name, docType.name),
      staleTime: FIVE_MINUTES,
    })),
  });
  const pagesQuery = useListRepoPages(owner, name, {
    query: { enabled: cfg !== undefined, staleTime: FIVE_MINUTES },
  });
  const pageList =
    pagesQuery.data?.status === 200 ? pagesQuery.data.data : undefined;

  return useMemo(() => {
    if (
      types === undefined ||
      docLists.some((query) => query.data === undefined) ||
      (cfg !== undefined && pageList === undefined)
    ) {
      return undefined;
    }
    const byId = new Map<string, string>();
    const byPath = new Map<string, string>();
    docLists.forEach((query, i) => {
      const typeName = types[i]?.name;
      if (query.data?.status !== 200 || typeName === undefined) {
        return;
      }
      for (const doc of arr(query.data.data.docs)) {
        const href = `/${owner}/${name}/${typeName}/${doc.doc_id}`;
        byId.set(doc.doc_id.toUpperCase(), href);
        byPath.set(doc.path, href);
      }
    });
    if (cfg !== undefined && pageList !== undefined && detail !== undefined) {
      for (const page of arr(pageList.pages)) {
        const href = `/${owner}/${name}/pages/${page.path}`;
        for (const key of pageSourceKeys(
          page.path,
          detail.docs_dir,
          cfg.additionalDocs,
        )) {
          // Docs win a key collision: a doc path is authoritative for
          // its own file, and page keys are reconstructions.
          if (!byPath.has(key)) {
            byPath.set(key, href);
          }
        }
      }
    }
    return { byId, byPath };
  }, [types, docLists, cfg, pageList, detail, owner, name]);
}
