import { keepPreviousData } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router";

import { useSearchDocs } from "@/api/__generated__/docz-api";
import { SessionRequiredError } from "@/api/fetcher";
import { StatusBadge, TypeBadge } from "@/components/badges";
import { RepoPicker, TypeChips } from "@/components/directory-controls";
import { ErrorPanel, SessionRequiredRedirect } from "@/components/query-states";
import { usePrefetchDoc } from "@/hooks/usePrefetchDoc";
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

// Mockup .doc-row grid; narrow viewports collapse to type/title/status.
const ROW_GRID =
  "grid grid-cols-[92px_minmax(0,1fr)_auto] items-center gap-[0.8rem] border-b border-border-hairline px-[0.4rem] py-[0.7rem] md:grid-cols-[116px_86px_minmax(0,1fr)_110px_150px_70px]";

function DirectoryHero({ repo }: { repo: string | null }) {
  return (
    <header className="pt-10 pb-2">
      <div className="font-mono text-[12.5px] tracking-[0.05em] text-accent">
        / docz <span className="text-fg-muted">/</span> {repo ?? "all repos"}
      </div>
      <h1 className="mt-2 mb-1 text-[clamp(1.6rem,4vw,2rem)] font-semibold tracking-[-0.01em] text-fg-primary">
        Documentation
      </h1>
      <p className="text-[14px] text-fg-tertiary">
        Designs, plans, RFCs, and decisions — across every repo registered with
        the API.
      </p>
    </header>
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
    <input
      type="search"
      aria-label="Search documents"
      placeholder="search docs…"
      value={draft}
      onChange={(event) => {
        setDraft(event.target.value);
      }}
      className="mt-5 w-full border border-border-default bg-bg-raised px-3 py-2 font-mono text-[13px] text-fg-primary placeholder:text-fg-muted focus:border-border-strong focus:outline-none"
    />
  );
}

function HitRow({ hit }: { hit: SearchHit }) {
  const repoName = hit.repo.split("/").at(-1) ?? hit.repo;
  const prefetchDoc = usePrefetchDoc();
  const [hitOwner = "", hitRepo = ""] = hit.repo.split("/");
  const prefetch = () => {
    prefetchDoc(hitOwner, hitRepo, hit.type, hit.doc_id);
  };
  return (
    <li>
      <Link
        to={`/${hit.repo}/${hit.type}/${hit.doc_id}`}
        onMouseEnter={prefetch}
        onFocus={prefetch}
        className={`${ROW_GRID} transition-colors hover:bg-bg-raised`}
      >
        <TypeBadge type={hit.type} />
        <span className="hidden font-mono text-[12.5px] text-fg-tertiary md:block">
          {hit.doc_id}
        </span>
        <span className="truncate text-[14px] text-fg-primary">
          {hit.title}
        </span>
        {hit.status === "" ? (
          <span aria-hidden />
        ) : (
          <StatusBadge status={hit.status} />
        )}
        <span
          title={hit.repo}
          className="hidden truncate font-mono text-[11.5px] text-fg-tertiary before:text-fg-muted before:content-['›_'] md:block"
        >
          {repoName}
        </span>
        {/*
         * SearchHit carries no updated_at yet (additive ask in
         * DESIGN-0001); formatRelativeTime takes over when it lands.
         */}
        <span className="hidden text-right font-mono text-[11px] text-fg-muted md:block">
          —
        </span>
      </Link>
    </li>
  );
}

function SkeletonRows() {
  return (
    <div
      aria-hidden
      data-testid="directory-skeleton"
      className="mt-4 animate-pulse border-t border-border-hairline"
    >
      {Array.from({ length: 6 }, (_, i) => (
        <div key={i} className={ROW_GRID}>
          <div className="h-4 w-16 bg-bg-elevated" />
          <div className="hidden h-3 w-14 bg-bg-raised md:block" />
          <div
            className="h-3 bg-bg-elevated"
            style={{ width: `${String(88 - (i % 3) * 14)}%` }}
          />
          <div className="h-3 w-12 bg-bg-raised" />
          <div className="hidden h-3 w-20 bg-bg-raised md:block" />
          <div className="hidden h-3 w-10 justify-self-end bg-bg-raised md:block" />
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
    toSearchDocsParams({ ...state, offset: 0 }, state.offset + PAGE_SIZE),
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
      <DirectoryHero repo={state.repo} />
      <SearchBox value={state.q} onCommit={commitQuery} />

      <div className="mt-5 flex flex-wrap items-center gap-4">
        <RepoPicker
          current={state.repo}
          counts={repoCounts}
          onPick={(repo) => {
            applyFilters({ repo });
          }}
        />
        {hasActiveFilters(state) && (
          <button
            type="button"
            onClick={() => {
              applyFilters(EMPTY_SEARCH_STATE);
            }}
            className="font-mono text-[11.5px] text-fg-muted hover:text-fg-primary"
          >
            clear filters ✕
          </button>
        )}
        {result !== undefined && (
          <div
            data-testid="results-count"
            className="ml-auto font-mono text-[12px] text-fg-tertiary"
          >
            showing{" "}
            <b className="font-medium text-fg-secondary">
              {result.hits.length}
            </b>{" "}
            of {result.estimated_total_hits}
          </div>
        )}
      </div>
      <TypeChips
        available={availableTypes}
        selected={state.types[0] ?? null}
        onSelect={(type) => {
          applyFilters({ types: type === null ? [] : [type] });
        }}
      />

      {searchQuery.isError ? (
        <div className="mt-6">
          <ErrorPanel
            message={
              searchQuery.error instanceof Error
                ? searchQuery.error.message
                : "Search failed"
            }
            onRetry={() => {
              void searchQuery.refetch();
            }}
          />
        </div>
      ) : result === undefined ? (
        <SkeletonRows />
      ) : result.hits.length === 0 ? (
        hasActiveFilters(state) ? (
          <div className="mx-auto my-16 w-max max-w-full border border-border-default bg-bg-raised px-8 py-6 text-center">
            <p className="font-mono text-[13px] text-fg-secondary">
              No matches
            </p>
            <button
              type="button"
              onClick={() => {
                applyFilters(EMPTY_SEARCH_STATE);
              }}
              className="mt-3 border border-border-strong px-4 py-1 font-mono text-[12px] text-fg-secondary hover:bg-bg-hover"
            >
              clear filters
            </button>
          </div>
        ) : (
          <div className="mx-auto my-16 w-max max-w-full border border-border-default bg-bg-raised px-8 py-6 text-center">
            <p className="font-mono text-[13px] text-fg-secondary">
              No documents yet
            </p>
            <p className="mt-2 max-w-96 text-[13px] text-fg-tertiary">
              Onboard a repo with the docz GitHub App to index its docs here.
            </p>
          </div>
        )
      ) : (
        <div className="mt-4 mb-16">
          <ul className="border-t border-border-hairline">
            {result.hits.map((hit) => (
              <HitRow key={`${hit.repo}/${hit.type}/${hit.doc_id}`} hit={hit} />
            ))}
          </ul>
          {result.hits.length < result.estimated_total_hits && (
            <button
              type="button"
              disabled={searchQuery.isFetching}
              onClick={() => {
                // Pushed: back shrinks the window again.
                setSearchParams(
                  serializeSearchState({
                    ...state,
                    offset: state.offset + PAGE_SIZE,
                  }),
                );
              }}
              className="mx-auto mt-6 block border border-border-default px-5 py-[0.45rem] font-mono text-[12px] text-fg-secondary hover:border-border-strong hover:text-fg-primary disabled:opacity-50"
            >
              {searchQuery.isFetching
                ? "loading…"
                : `load more · ${String(result.estimated_total_hits - result.hits.length)} remaining`}
            </button>
          )}
        </div>
      )}
    </main>
  );
}
