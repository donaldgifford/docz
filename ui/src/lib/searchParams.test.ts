import { describe, expect, it } from "vitest";

import {
  EMPTY_SEARCH_STATE,
  hasActiveFilters,
  parseSearchParams,
  serializeSearchState,
  toSearchDocsParams,
  type DirectorySearchState,
} from "@/lib/searchParams";

describe("parseSearchParams", () => {
  it("returns the empty state for no params", () => {
    expect(parseSearchParams(new URLSearchParams())).toEqual(
      EMPTY_SEARCH_STATE,
    );
  });

  it("parses every supported param", () => {
    const params = new URLSearchParams(
      "q=ingest&repo=acme%2Fdocs&type=rfc&type=adr&status=Draft&author=don&offset=25",
    );
    expect(parseSearchParams(params)).toEqual({
      q: "ingest",
      repo: "acme/docs",
      types: ["rfc", "adr"],
      statuses: ["Draft"],
      authors: ["don"],
      offset: 25,
    });
  });

  it.each([
    ["-5", 0],
    ["abc", 0],
    ["0", 0],
    ["3.9", 3],
  ])("normalizes offset %s to %d", (raw, expected) => {
    const params = new URLSearchParams({ offset: raw });
    expect(parseSearchParams(params).offset).toBe(expected);
  });

  it("drops blank facet values and blank repo", () => {
    const params = new URLSearchParams("repo=+&type=&type=rfc&status=+++");
    expect(parseSearchParams(params)).toEqual({
      ...EMPTY_SEARCH_STATE,
      types: ["rfc"],
    });
  });
});

describe("round trips", () => {
  const states: DirectorySearchState[] = [
    EMPTY_SEARCH_STATE,
    {
      q: "search term with spaces",
      repo: "donaldgifford/docz-api",
      types: ["rfc", "design"],
      statuses: ["Draft", "In Review"],
      authors: ["Donald Gifford"],
      offset: 50,
    },
    { ...EMPTY_SEARCH_STATE, types: ["framework"] },
    { ...EMPTY_SEARCH_STATE, q: "a&b=c?d" },
  ];

  it.each(states.map((state) => [state]))(
    "parse(serialize(state)) is identity: %j",
    (state) => {
      expect(parseSearchParams(serializeSearchState(state))).toEqual(state);
    },
  );

  it("serialize omits defaults entirely", () => {
    expect(serializeSearchState(EMPTY_SEARCH_STATE).toString()).toBe("");
  });

  it("serialize(parse(url)) is stable for canonical urls", () => {
    const url = "q=x&repo=a%2Fb&type=rfc&type=adr&offset=25";
    const once = serializeSearchState(
      parseSearchParams(new URLSearchParams(url)),
    );
    expect(serializeSearchState(parseSearchParams(once)).toString()).toBe(
      once.toString(),
    );
  });
});

describe("hasActiveFilters", () => {
  it("is false for the empty state and offset-only state", () => {
    expect(hasActiveFilters(EMPTY_SEARCH_STATE)).toBe(false);
    expect(hasActiveFilters({ ...EMPTY_SEARCH_STATE, offset: 25 })).toBe(false);
  });

  it("is true when any facet or query is set", () => {
    expect(hasActiveFilters({ ...EMPTY_SEARCH_STATE, q: "x" })).toBe(true);
    expect(hasActiveFilters({ ...EMPTY_SEARCH_STATE, types: ["rfc"] })).toBe(
      true,
    );
    expect(hasActiveFilters({ ...EMPTY_SEARCH_STATE, repo: "a/b" })).toBe(true);
  });
});

describe("toSearchDocsParams ordering (IMPL-0006 OQ-3)", () => {
  it("sorts the empty listing newest-first", () => {
    const params = toSearchDocsParams(EMPTY_SEARCH_STATE, 25, {
      ordered: true,
    });
    expect(params.sort).toBe("updated_at:desc");
  });

  it("drops the sort once a query is typed", () => {
    // `sort` is a total order, not a relevance tie-break, so keeping it
    // during a text search would rank recent-but-irrelevant hits above
    // the best match.
    const params = toSearchDocsParams(
      { ...EMPTY_SEARCH_STATE, q: "reader" },
      25,
      { ordered: true },
    );
    expect(params.sort).toBeUndefined();
    expect(params.q).toBe("reader");
  });

  it("never sorts a facet query, even with an empty q", () => {
    // Facet queries run at limit 0 — ordering them is server-side work
    // for a count nobody reads.
    expect(toSearchDocsParams(EMPTY_SEARCH_STATE, 0).sort).toBeUndefined();
  });

  it("keeps ordering independent of the other filters", () => {
    const params = toSearchDocsParams(
      { ...EMPTY_SEARCH_STATE, types: ["design"], offset: 25 },
      50,
      { ordered: true },
    );
    expect(params).toMatchObject({
      limit: 50,
      offset: 25,
      type: "design",
      sort: "updated_at:desc",
    });
  });

  it("leaves no sort key in the URL", () => {
    // Derived, not stored: no control selects it, so it has no URL
    // surface and a round trip must not invent one.
    const url = serializeSearchState(EMPTY_SEARCH_STATE);
    expect(url.has("sort")).toBe(false);
    expect(url.has("source")).toBe(false);
  });
});
