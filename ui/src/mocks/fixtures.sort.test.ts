import { describe, expect, it } from "vitest";

import type { SearchResult } from "@/api/__generated__/docz-api.schemas";

/*
 * The demo search resolver must order hits the way Meilisearch does, or
 * `dev:msw` and every e2e journey quietly disagree with the real API on
 * the one thing this phase changed.
 *
 * These call the handler through MSW rather than importing a helper, so
 * what is under test is the resolver the rest of the suite actually
 * runs against.
 */

async function search(query: string): Promise<SearchResult> {
  const response = await fetch(`http://localhost/api/v1/search?${query}`);
  expect(response.status).toBe(200);
  return (await response.json()) as SearchResult;
}

describe("demo searchDocs ordering", () => {
  it("returns relevance order when no sort is asked for", async () => {
    const { hits } = await search("limit=50");
    // Docs first, then pages — the resolver's own concatenation order.
    const kinds = [...new Set(hits.map((hit) => hit.source))];
    expect(kinds).toEqual(["doc", "page"]);
  });

  it("sorts newest-first on updated_at:desc", async () => {
    const { hits } = await search("limit=50&sort=updated_at:desc");
    const stamps = hits.map((hit) => hit.updated_at);
    expect(stamps).toEqual([...stamps].sort().reverse());
  });

  it("sorts oldest-first on updated_at:asc", async () => {
    const { hits } = await search("limit=50&sort=updated_at:asc");
    const stamps = hits.map((hit) => hit.updated_at);
    expect(stamps).toEqual([...stamps].sort());
  });

  it("puts undated records last in BOTH directions", async () => {
    // Meilisearch treats an empty value as absent, not as the
    // lexicographic minimum. Page hits carry no `created`, so they land
    // after every document whichever way `created` is sorted — the
    // quirk most likely to be got wrong by a naive fixture.
    for (const direction of ["asc", "desc"]) {
      const { hits } = await search(`limit=50&sort=created:${direction}`);
      // `lastIndexOf` over a boolean projection rather than
      // `findLastIndex`, which needs a newer lib than the app targets.
      const dated = hits.map((hit) => hit.created !== "");
      const firstUndated = dated.indexOf(false);
      const lastDated = dated.lastIndexOf(true);
      expect(firstUndated).toBeGreaterThan(-1);
      expect(firstUndated).toBeGreaterThan(lastDated);
    }
  });

  it("paginates a sorted set without reshuffling", async () => {
    // The directory grows `limit` from a fixed offset 0, so a wider
    // window must be a prefix-preserving superset of a narrower one.
    const narrow = await search("limit=3&sort=updated_at:desc");
    const wide = await search("limit=8&sort=updated_at:desc");
    expect(wide.hits.slice(0, 3).map((hit) => hit.title)).toEqual(
      narrow.hits.map((hit) => hit.title),
    );
  });
});

describe("demo searchDocs source filter", () => {
  it("returns only documents for source=doc", async () => {
    const { hits } = await search("limit=50&source=doc");
    expect(hits.length).toBeGreaterThan(0);
    expect(hits.every((hit) => hit.source === "doc")).toBe(true);
  });

  it("returns only pages for source=page", async () => {
    const { hits } = await search("limit=50&source=page");
    expect(hits.length).toBeGreaterThan(0);
    expect(hits.every((hit) => hit.source === "page")).toBe(true);
  });
});

describe("demo searchDocs validation", () => {
  it("400s an unrecognized sort", async () => {
    const response = await fetch(
      "http://localhost/api/v1/search?sort=title:desc",
    );
    expect(response.status).toBe(400);
    expect(await response.json()).toEqual({ error: "invalid sort" });
  });

  it("does not validate filter values — they just match nothing", async () => {
    const { hits } = await search("limit=50&status=NotAStatus");
    expect(hits).toEqual([]);
  });
});
