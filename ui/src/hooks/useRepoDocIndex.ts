import { useQueries } from "@tanstack/react-query";
import { useMemo } from "react";

import {
  getListDocsQueryOptions,
  useGetRepo,
} from "@/api/__generated__/docz-api";
import { arr } from "@/lib/wire";

import type { XrefResolver } from "@/markdown/xrefs";

const FIVE_MINUTES = 5 * 60_000;

/**
 * Every doc in a repo, mapped to its reader href two ways (DESIGN-0002
 * Component 2): `byId` keys UPPERCASED doc ids — the xref resolver for
 * doc-id tokens in rendered bodies — and `byPath` keys the ingested
 * repo-relative file paths, the whitelist relative markdown links
 * resolve through. Hrefs are always built from API data, never from
 * document text.
 */
export interface RepoDocIndex {
  /** UPPERCASED doc_id -> SPA href ("/owner/name/type/DOC-0001"). */
  byId: XrefResolver;
  /** Repo-relative doc path ("docs/adr/0013-….md") -> the same href. */
  byPath: ReadonlyMap<string, string>;
}

/**
 * Both maps come from getRepo's type set plus one listDocs per type
 * (the same queries the repo nav runs, so the cache is usually warm).
 * Undefined until every list resolves, so a body is linkified at most
 * once more after first paint.
 */
export function useRepoDocIndex(
  owner: string,
  name: string,
): RepoDocIndex | undefined {
  const repoQuery = useGetRepo(owner, name);
  const types =
    repoQuery.data?.status === 200 ? arr(repoQuery.data.data.types) : undefined;

  const docLists = useQueries({
    queries: (types ?? []).map((docType) => ({
      ...getListDocsQueryOptions(owner, name, docType.name),
      staleTime: FIVE_MINUTES,
    })),
  });

  return useMemo(() => {
    if (
      types === undefined ||
      docLists.some((query) => query.data === undefined)
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
    return { byId, byPath };
  }, [types, docLists, owner, name]);
}
