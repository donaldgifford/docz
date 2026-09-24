/*
 * The URL is the only source of filter truth for the directory
 * (DESIGN-0001): parse on read, serialize on change, never mirror into
 * component state. Multi-value facets repeat their key (`?type=rfc&
 * type=adr`); defaults are omitted so shared URLs stay clean.
 *
 * Note: searchDocs currently accepts a single value per facet — the
 * route sends the first selection of each array (interim until the API
 * grows multi-value filters).
 */
import { SearchDocsSort } from "@/api/__generated__/docz-api.schemas";

import type { SearchDocsParams } from "@/api/__generated__/docz-api.schemas";

export interface DirectorySearchState {
  q: string;
  repo: string | null;
  types: string[];
  statuses: string[];
  authors: string[];
  offset: number;
}

export const EMPTY_SEARCH_STATE: DirectorySearchState = {
  q: "",
  repo: null,
  types: [],
  statuses: [],
  authors: [],
  offset: 0,
};

function parseOffset(raw: string | null): number {
  if (raw === null) {
    return 0;
  }
  const value = Number.parseInt(raw, 10);
  return Number.isFinite(value) && value > 0 ? value : 0;
}

function cleanAll(values: string[]): string[] {
  return values.map((value) => value.trim()).filter((value) => value !== "");
}

export function parseSearchParams(
  params: URLSearchParams,
): DirectorySearchState {
  const repo = params.get("repo");
  return {
    q: params.get("q") ?? "",
    repo: repo === null || repo.trim() === "" ? null : repo,
    types: cleanAll(params.getAll("type")),
    statuses: cleanAll(params.getAll("status")),
    authors: cleanAll(params.getAll("author")),
    offset: parseOffset(params.get("offset")),
  };
}

export function serializeSearchState(
  state: DirectorySearchState,
): URLSearchParams {
  const params = new URLSearchParams();
  if (state.q !== "") {
    params.set("q", state.q);
  }
  if (state.repo !== null) {
    params.set("repo", state.repo);
  }
  for (const type of state.types) {
    params.append("type", type);
  }
  for (const status of state.statuses) {
    params.append("status", status);
  }
  for (const author of state.authors) {
    params.append("author", author);
  }
  if (state.offset > 0) {
    params.set("offset", String(state.offset));
  }
  return params;
}

/**
 * Map URL state onto the searchDocs query params. Facets send the first
 * selection of each array (see the module note); defaults are omitted.
 *
 * `ordered` asks for the listing's sort and is passed ONLY by the query
 * that renders rows. Facet queries run at `limit: 0` and have no order
 * to speak of, so making them sort would be pure server-side work for a
 * count.
 */
export function toSearchDocsParams(
  state: DirectorySearchState,
  limit: number,
  { ordered = false }: { ordered?: boolean } = {},
): SearchDocsParams {
  const params: SearchDocsParams = { limit };
  /*
   * Recency for browsing, relevance for searching (IMPL-0006 OQ-3).
   *
   * `sort` is a TOTAL order over the matches, not a tie-break within
   * relevance — the spec is explicit about that. Sorting a text search
   * would therefore rank a recently-ingested irrelevant document above
   * the best match, which is why the empty directory gets newest-first
   * and a typed query does not. It is derived rather than stored: no
   * control selects it, so it has no business in the URL.
   */
  if (ordered && state.q === "") {
    params.sort = SearchDocsSort["updated_at:desc"];
  }
  if (state.q !== "") {
    params.q = state.q;
  }
  if (state.repo !== null) {
    params.repo = state.repo;
  }
  const [type] = state.types;
  if (type !== undefined) {
    params.type = type;
  }
  const [status] = state.statuses;
  if (status !== undefined) {
    params.status = status;
  }
  const [author] = state.authors;
  if (author !== undefined) {
    params.author = author;
  }
  if (state.offset > 0) {
    params.offset = state.offset;
  }
  return params;
}

export function hasActiveFilters(state: DirectorySearchState): boolean {
  return (
    state.q !== "" ||
    state.repo !== null ||
    state.types.length > 0 ||
    state.statuses.length > 0 ||
    state.authors.length > 0
  );
}
