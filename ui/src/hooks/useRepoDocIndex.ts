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
 * Every doc id in a repo, mapped to its reader href — the xref resolver
 * for rendered bodies. Built from getRepo's type set plus one listDocs
 * per type (the same queries the repo nav runs, so the cache is
 * usually warm). Undefined until every list resolves, so a body is
 * linkified at most once more after first paint.
 */
export function useRepoDocIndex(
  owner: string,
  name: string,
): XrefResolver | undefined {
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
    const resolver = new Map<string, string>();
    docLists.forEach((query, i) => {
      const typeName = types[i]?.name;
      if (query.data?.status !== 200 || typeName === undefined) {
        return;
      }
      for (const doc of arr(query.data.data.docs)) {
        resolver.set(
          doc.doc_id.toUpperCase(),
          `/${owner}/${name}/${typeName}/${doc.doc_id}`,
        );
      }
    });
    return resolver;
  }, [types, docLists, owner, name]);
}
