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

export function hasActiveFilters(state: DirectorySearchState): boolean {
  return (
    state.q !== "" ||
    state.repo !== null ||
    state.types.length > 0 ||
    state.statuses.length > 0 ||
    state.authors.length > 0
  );
}
