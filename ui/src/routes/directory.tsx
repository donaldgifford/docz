import { FilterBar } from "@donaldgifford/design-system/blocks/filter-bar";
import { ListRow } from "@donaldgifford/design-system/blocks/list-row";
import { LoadMore } from "@donaldgifford/design-system/blocks/load-more";
import { Marked } from "@donaldgifford/design-system/blocks/marked";
import { PageHeader } from "@donaldgifford/design-system/blocks/page-header";
import { Time } from "@donaldgifford/design-system/blocks/time";
import { Button } from "@donaldgifford/design-system/components/button";
import { Input } from "@donaldgifford/design-system/components/input";
import { cn } from "@donaldgifford/design-system/lib/utils";
import { keepPreviousData } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router";

import { useSearchDocs } from "@/api/__generated__/docz-api";
import { SessionRequiredError } from "@/api/fetcher";
import { StatusPill } from "@/components/badges";
import { RepoPicker, TypeChips } from "@/components/directory-controls";
import {
  EmptyPanel,
  ErrorPanel,
  SessionRequiredRedirect,
  Skeleton,
} from "@/components/query-states";
import { usePrefetchDoc } from "@/hooks/usePrefetchDoc";
import { usePrefetchPage } from "@/hooks/usePrefetchPage";
import {
  EMPTY_SEARCH_STATE,
  hasActiveFilters,
  parseSearchParams,
  serializeSearchState,
  toSearchDocsParams,
  type DirectorySearchState,
} from "@/lib/searchParams";

import type { SearchHit } from "@/api/__generated__/docz-api.schemas";

export const PAGE_SIZE = 25;
const Q_DEBOUNCE_MS = 200;

/*
 * The directory is a searchDocs view of the whole registry
 * (DESIGN-0001 Decision 4). The URL is the only source of filter truth:
 * every control reads parseSearchParams(useSearchParams()) and writes
 * back through serializeSearchState — no mirrored component state
 * except the debounce buffer inside SearchBox.
 */

/*
 * A hit is the design system's ListRow in a card (DESIGN-0028 §7):
 * "DOC-ID / repo" over the title, the snippet under it when the search
 * matched the body, and the status and the updated stamp on the right.
 * The title is the row's link, stretched over the card. Status is the
 * only color in the row — the doc id already spells the type out.
 */
const ROW_CARD =
  "rounded-md border bg-card transition-colors hover:border-border-strong hover:bg-muted";
const ROW_PADDING = "px-4 py-3";
/** The stamp's column: wide enough for a date, so the statuses line up. */
const ROW_STAMP = "hidden w-24 md:inline-flex";

function DirectoryHero() {
  return (
    <PageHeader
      className="px-0"
      title="Documentation"
      description="Designs, plans, RFCs, and decisions — across every repo registered with the API."
    />
  );
}

/**
 * Debounced query input. `value` is the committed URL state; the draft
 * only exists while typing and yields to external URL changes
 * (back/forward, palette navigation).
 */
function SearchBox({
  value,
  onCommit,
}: {
  value: string;
  onCommit: (q: string) => void;
}) {
  const [draft, setDraft] = useState(value);

  // Adjust-during-render (react.dev "adjusting state when a prop
  // changes"): external URL changes override the draft without
  // remounting, so the input keeps focus while typing.
  const [prevValue, setPrevValue] = useState(value);
  if (prevValue !== value) {
    setPrevValue(value);
    setDraft(value);
  }

  useEffect(() => {
    if (draft === value) {
      return;
    }
    const timer = setTimeout(() => {
      onCommit(draft);
    }, Q_DEBOUNCE_MS);
    return () => {
      clearTimeout(timer);
    };
  }, [draft, value, onCommit]);

  return (
    <Input
      type="search"
      aria-label="Search documents"
      placeholder="search docs…"
      value={draft}
      onChange={(event) => {
        setDraft(event.target.value);
      }}
      className="font-mono"
    />
  );
}

/** Row key AND page-vs-doc link target — pages key on published path. */
function hitKey(hit: SearchHit): string {
  return hit.source === "page"
    ? `${hit.repo}/pages/${hit.path}`
    : `${hit.repo}/${hit.type}/${hit.doc_id}`;
}

function HitRow({ hit }: { hit: SearchHit }) {
  const repoName = hit.repo.split("/").at(-1) ?? hit.repo;
  const prefetchDoc = usePrefetchDoc();
  const prefetchPage = usePrefetchPage();
  const [hitOwner = "", hitRepo = ""] = hit.repo.split("/");
  const isPage = hit.source === "page";
  const prefetch = () => {
    if (isPage) {
      prefetchPage(hitOwner, hitRepo, hit.path);
    } else {
      prefetchDoc(hitOwner, hitRepo, hit.type, hit.doc_id);
    }
  };
  return (
    <li className={ROW_CARD}>
      <ListRow
        className={ROW_PADDING}
        // Pages have no doc id, so they keep the neutral source marker
        // in its place (DESIGN-0004 OQ-3a). The repo rides the same
        // line: "DESIGN-0001 / docz-site" is one coordinate.
        eyebrow={
          <span title={isPage ? `${hit.repo}/${hit.path}` : hit.repo}>
            {isPage ? "page" : hit.doc_id} / {repoName}
          </span>
        }
        title={hit.title}
        detail={hit.snippet === "" ? undefined : <Marked text={hit.snippet} />}
        trailing={
          <>
            {hit.status !== "" && <StatusPill status={hit.status} />}
            {/* Both record kinds carry `updated_at` since spec 1.5.0,
                so page rows are dated too; Time draws the em dash for
                a stamp that is unset. */}
            <Time iso={hit.updated_at} style="stacked" className={ROW_STAMP} />
          </>
        }
        render={
          <Link
            to={`/${hitKey(hit)}`}
            onMouseEnter={prefetch}
            onFocus={prefetch}
          />
        }
      />
    </li>
  );
}

function SkeletonRows() {
  return (
    <div
      aria-hidden
      data-testid="directory-skeleton"
      className="mt-4 space-y-2"
    >
      {Array.from({ length: 6 }, (_, i) => (
        <div
          key={i}
          className={cn(ROW_CARD, ROW_PADDING, "flex items-center gap-3")}
        >
          <div className="min-w-0 flex-1">
            <Skeleton className="h-3 w-20" />
            <Skeleton
              className="mt-2 h-4"
              style={{ width: `${String(72 - (i % 3) * 14)}%` }}
            />
          </div>
          <Skeleton className="h-5 w-20" />
          <div className={cn(ROW_STAMP, "flex-col items-end gap-2")}>
            <Skeleton className="h-3 w-20" />
            <Skeleton className="h-3 w-12" />
          </div>
        </div>
      ))}
    </div>
  );
}

export function Component() {
  const [searchParams, setSearchParams] = useSearchParams();
  const state = useMemo(() => parseSearchParams(searchParams), [searchParams]);

  // URL `offset` means "rows 0..offset+PAGE_SIZE are shown": the query
  // fetches the whole window from 0, so a shared or deep-linked URL
  // renders exactly the same rows (the URL stays the only source of
  // truth). "Load more" grows the window by pushing offset.
  const searchQuery = useSearchDocs(
    toSearchDocsParams({ ...state, offset: 0 }, state.offset + PAGE_SIZE, {
      ordered: true,
    }),
    {
      // Keep the previous window on screen while a filter change
      // refetches; the skeleton is for first paint only.
      query: { placeholderData: keepPreviousData },
    },
  );
  const result =
    searchQuery.data?.status === 200 ? searchQuery.data.data : undefined;

  // Facet sources exclude their own dimension (standard faceted-search
  // behavior): the repo picker keeps listing every repo while one is
  // selected, and the type chips keep listing every type. When nothing
  // is filtered these collapse to one deduped request.
  const repoFacetQuery = useSearchDocs(
    toSearchDocsParams({ ...state, repo: null, offset: 0 }, 0),
    { query: { placeholderData: keepPreviousData } },
  );
  const typeFacetQuery = useSearchDocs(
    toSearchDocsParams({ ...state, types: [], offset: 0 }, 0),
    { query: { placeholderData: keepPreviousData } },
  );
  const repoCounts =
    (repoFacetQuery.data?.status === 200
      ? repoFacetQuery.data.data.facets.repo
      : undefined) ?? {};
  const availableTypes = Object.keys(
    (typeFacetQuery.data?.status === 200
      ? typeFacetQuery.data.data.facets.type
      : undefined) ?? {},
  ).sort();

  const commitQuery = useCallback(
    (q: string) => {
      // Typing replaces the history entry (no per-debounce litter) and
      // resets pagination; discrete filter actions push instead, so
      // back/forward walks filter history.
      setSearchParams(serializeSearchState({ ...state, q, offset: 0 }), {
        replace: true,
      });
    },
    [state, setSearchParams],
  );

  const applyFilters = (partial: Partial<DirectorySearchState>) => {
    // Pushed (not replaced): back/forward walks filter history.
    setSearchParams(serializeSearchState({ ...state, ...partial, offset: 0 }));
  };

  if (searchQuery.error instanceof SessionRequiredError) {
    return <SessionRequiredRedirect />;
  }

  return (
    <main className="mx-auto max-w-[940px] px-5">
      <DirectoryHero />
      <SearchBox value={state.q} onCommit={commitQuery} />

      <FilterBar
        className="mt-4"
        onClear={
          hasActiveFilters(state)
            ? () => {
                applyFilters(EMPTY_SEARCH_STATE);
              }
            : undefined
        }
        count={
          result === undefined ? undefined : (
            <span data-testid="results-count">
              showing{" "}
              <b className="font-medium text-foreground">
                {result.hits.length}
              </b>{" "}
              of {result.estimated_total_hits}
              {/* Source split only when pages are present, so non-opted
                  deployments render today's exact line (OQ-3a). */}
              {(result.facets.source?.page ?? 0) > 0 &&
                ` · ${String(result.facets.source?.doc ?? 0)} docs · ${String(result.facets.source?.page ?? 0)} pages`}
            </span>
          )
        }
      >
        <RepoPicker
          current={state.repo}
          counts={repoCounts}
          onPick={(repo) => {
            applyFilters({ repo });
          }}
        />
        <TypeChips
          available={availableTypes}
          selected={state.types[0] ?? null}
          onSelect={(type) => {
            applyFilters({ types: type === null ? [] : [type] });
          }}
        />
      </FilterBar>

      {searchQuery.isError ? (
        <div className="mt-6">
          <ErrorPanel
            error={searchQuery.error}
            fallback="Search failed"
            onRetry={() => {
              void searchQuery.refetch();
            }}
          />
        </div>
      ) : result === undefined ? (
        <SkeletonRows />
      ) : result.hits.length === 0 ? (
        hasActiveFilters(state) ? (
          <EmptyPanel
            title="No matches"
            description="Nothing matches this search and these filters."
            action={
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  applyFilters(EMPTY_SEARCH_STATE);
                }}
              >
                clear filters
              </Button>
            }
          />
        ) : (
          <EmptyPanel
            title="No documents yet"
            description="Onboard a repo with the docz GitHub App to index its docs here."
          />
        )
      ) : (
        <div className="mt-4 mb-16">
          <ul className="space-y-2">
            {result.hits.map((hit) => (
              <HitRow key={hitKey(hit)} hit={hit} />
            ))}
          </ul>
          <LoadMore
            className="mt-4 border-t-0 px-0"
            noun="documents"
            loaded={result.hits.length}
            total={result.estimated_total_hits}
            hasMore={result.hits.length < result.estimated_total_hits}
            loading={searchQuery.isFetching}
            onMore={() => {
              // Pushed: back shrinks the window again.
              setSearchParams(
                serializeSearchState({
                  ...state,
                  offset: state.offset + PAGE_SIZE,
                }),
              );
            }}
          />
        </div>
      )}
    </main>
  );
}
