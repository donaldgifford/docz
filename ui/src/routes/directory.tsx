import { LoadMore } from "@donaldgifford/design-system/blocks/load-more";
import { PageHeader } from "@donaldgifford/design-system/blocks/page-header";
import { Button } from "@donaldgifford/design-system/components/button";
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
import { formatUpdatedStamp } from "@/lib/updatedAt";

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
 * Hit rows are cards, not table rows (DESIGN-0005 dial-in, replacing
 * the mockup's six-column `.doc-row`): "DOC-ID / repo" over the title
 * on the left, the status pill in the middle, the updated stamp on the
 * right. Status is the only color in the row — the type badge that used
 * to lead it competed with it, and the doc id already spells the type
 * out. Both outer columns are two lines, so they align with each other
 * rather than floating against a taller middle.
 */
const ROW_CARD =
  "grid grid-cols-[minmax(0,1fr)_auto] items-center gap-4 border border-border bg-card px-4 py-3 transition-colors hover:border-border-strong hover:bg-muted md:grid-cols-[minmax(0,1fr)_132px_118px] md:gap-6";

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
    <input
      type="search"
      aria-label="Search documents"
      placeholder="search docs…"
      value={draft}
      onChange={(event) => {
        setDraft(event.target.value);
      }}
      className="w-full border border-border bg-card px-3 py-2 font-mono text-[14px] text-foreground placeholder:text-muted-foreground focus:border-border-strong focus:outline-none rounded-sm"
    />
  );
}

/** Row key AND page-vs-doc link target — pages key on published path. */
function hitKey(hit: SearchHit): string {
  return hit.source === "page"
    ? `${hit.repo}/pages/${hit.path}`
    : `${hit.repo}/${hit.type}/${hit.doc_id}`;
}

/**
 * Right-hand stamp: date over time, mirroring the id/title pair on the
 * left. Both record kinds carry `updated_at` since spec 1.5.0, so page
 * rows are dated too; the em dash is now only for a genuinely unset
 * stamp.
 */
function UpdatedCell({ hit }: { hit: SearchHit }) {
  const iso = hit.updated_at;
  const stamp = formatUpdatedStamp(iso);
  if (stamp === undefined) {
    return (
      <span className="hidden text-right font-mono text-[12.5px] text-muted-foreground md:block">
        —
      </span>
    );
  }
  return (
    <time
      dateTime={iso}
      className="hidden text-right font-mono text-[12.5px] whitespace-nowrap text-muted-foreground md:block"
    >
      {stamp.date}
      <span className="mt-[3px] block text-[11.5px] text-muted-foreground">
        {stamp.time}
      </span>
    </time>
  );
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
    <li>
      <Link
        to={`/${hitKey(hit)}`}
        onMouseEnter={prefetch}
        onFocus={prefetch}
        className={ROW_CARD}
      >
        <span className="min-w-0">
          {/* Pages have no doc id, so they keep the neutral source
              marker in its place (DESIGN-0004 OQ-3a). The repo rides
              the same line: "DESIGN-0001 / docz-site" is one
              coordinate, and splitting it across the card made the
              reader scan sideways to assemble it. */}
          <span
            title={isPage ? `${hit.repo}/${hit.path}` : hit.repo}
            className="block truncate font-mono text-[12px] tracking-[0.08em] text-muted-foreground uppercase"
          >
            {isPage ? "page" : hit.doc_id}
            {/* Inherits fg-muted: every token in this line is
                contrast-checked, and a dimmer separator would only be
                one the e2e axe sweep rejects. */}
            <span aria-hidden className="px-[0.4em]">
              /
            </span>
            <span className="normal-case">{repoName}</span>
          </span>
          <span className="mt-[3px] block truncate text-[16px] text-foreground">
            {hit.title}
          </span>
        </span>
        <span className="justify-self-start md:justify-self-center">
          {hit.status !== "" && <StatusPill status={hit.status} />}
        </span>
        <UpdatedCell hit={hit} />
      </Link>
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
        <div key={i} className={ROW_CARD}>
          <div>
            <Skeleton className="h-3 w-20" />
            <Skeleton
              className="mt-2 h-4"
              style={{ width: `${String(72 - (i % 3) * 14)}%` }}
            />
          </div>
          <Skeleton className="h-5 w-20 md:justify-self-center" />
          <div className="hidden justify-self-end md:block">
            <Skeleton className="h-3 w-22" />
            <Skeleton className="mt-2 ml-auto h-3 w-14" />
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
            className="font-mono text-[12.5px] text-muted-foreground hover:text-foreground"
          >
            clear filters ✕
          </button>
        )}
        {result !== undefined && (
          <div
            data-testid="results-count"
            className="ml-auto font-mono text-[13px] text-muted-foreground"
          >
            showing{" "}
            <b className="font-medium text-foreground">{result.hits.length}</b>{" "}
            of {result.estimated_total_hits}
            {/* Source split only when pages are present, so non-opted
                deployments render today's exact line (OQ-3a). */}
            {(result.facets.source?.page ?? 0) > 0 &&
              ` · ${String(result.facets.source?.doc ?? 0)} docs · ${String(result.facets.source?.page ?? 0)} pages`}
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
