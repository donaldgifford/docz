import { FilterChip } from "@donaldgifford/design-system/blocks/filter-chip";
import { ListRow } from "@donaldgifford/design-system/blocks/list-row";
import { Marked } from "@donaldgifford/design-system/blocks/marked";
import {
  CommandDialog,
  CommandEmpty,
  CommandFilters,
  CommandFooter,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@donaldgifford/design-system/components/command";
import { keepPreviousData } from "@tanstack/react-query";
import {
  useEffect,
  useMemo,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from "react";
import { useNavigate } from "react-router";

import { useSearchDocs } from "@/api/__generated__/docz-api";
import { StatusBadge, StatusPill, TypeDot } from "@/components/badges";
import { usePrefetchDoc } from "@/hooks/usePrefetchDoc";
import { usePrefetchPage } from "@/hooks/usePrefetchPage";
import { readRecentDocs } from "@/lib/recentDocs";

import type {
  SearchDocsParams,
  SearchHit,
} from "@/api/__generated__/docz-api.schemas";

/*
 * ⌘K palette (DESIGN-0028 §7): the design system's CommandDialog with
 * its three optional regions, a row of filters under the input, a
 * preview of the highlighted hit beside the list (built purely from hit
 * data, no extra fetch), and key hints along the foot. Search state
 * here is palette-local (DESIGN-0001: the filters are facet shortcuts
 * applied to the palette query only); the directory URL is untouched
 * until a hit is opened.
 *
 * This module is its own chunk. command-palette-lazy.tsx holds the
 * shortcut and loads it on first use, so cmdk, the dialog, and
 * everything below stay out of the first paint.
 */

const Q_DEBOUNCE_MS = 200;
const PALETTE_LIMIT = 50;

type PaletteFilter =
  | { kind: "all" }
  | { kind: "repo"; value: string }
  | { kind: "type"; value: string };

const ALL: PaletteFilter = { kind: "all" };

function useDebouncedValue<T>(value: T, ms: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebounced(value);
    }, ms);
    return () => {
      clearTimeout(timer);
    };
  }, [value, ms]);
  return debounced;
}

// Lowercased: cmdk normalizes item values, so keys must round-trip
// through it unchanged. Navigation resolves the hit through a map and
// uses the original casing. Page hits key on their published path —
// their doc coordinates are all "".
function hitKey(hit: SearchHit): string {
  return (
    hit.source === "page"
      ? `${hit.repo}/pages/${hit.path}`
      : `${hit.repo}/${hit.type}/${hit.doc_id}`
  ).toLowerCase();
}

function hitHref(hit: SearchHit): string {
  return hit.source === "page"
    ? `/${hit.repo}/pages/${hit.path}`
    : `/${hit.repo}/${hit.type}/${hit.doc_id}`;
}

/** The mono id line: doc id, or the neutral "page" marker + path. */
function hitMarker(hit: SearchHit): string {
  return hit.source === "page" ? `page · ${hit.path}` : hit.doc_id;
}

function repoName(repo: string): string {
  return repo.split("/").at(-1) ?? repo;
}

/** A recent entry as a hit: coordinates and title, nothing else known. */
function recentHit(
  entry: ReturnType<typeof readRecentDocs>[number],
): SearchHit {
  // Recents store coordinates and title only — never dates, never
  // tokens — so status, author, and the 1.5.0 stamps are unknown here
  // and stay "" permanently. The palette shows none of them.
  const unknown = {
    status: "",
    author: "",
    created: "",
    updated_at: "",
    snippet: "",
  };
  return entry.kind === "page"
    ? {
        ...unknown,
        source: "page",
        repo: entry.repo,
        doc_id: "",
        type: "",
        title: entry.title,
        path: entry.path,
      }
    : {
        ...unknown,
        source: "doc",
        repo: entry.repo,
        doc_id: entry.docId,
        type: entry.type,
        title: entry.title,
        path: "",
      };
}

/**
 * One hit, drawn by the design system's ListRow inside cmdk's item.
 * The item is the control: a click or Enter on it selects the hit,
 * which closes the palette and navigates. The row carries no link of
 * its own, because cmdk's item is a `role="option"` and a link inside
 * one is a control nested in a control (axe's nested-interactive).
 */
function HitRow({
  hit,
  showStatus,
}: {
  hit: SearchHit;
  /** Results show the status and the snippet; recents know neither. */
  showStatus: boolean;
}) {
  return (
    <ListRow
      className="w-full"
      // Pages carry the neutral marker, never a type badge
      // (DESIGN-0004 OQ-3a).
      eyebrow={
        showStatus
          ? hitMarker(hit)
          : `${hitMarker(hit)} · ${repoName(hit.repo)}`
      }
      title={hit.title}
      detail={
        showStatus && hit.snippet !== "" ? (
          <Marked text={hit.snippet} />
        ) : undefined
      }
      trailing={
        showStatus && hit.status !== "" ? (
          <StatusPill status={hit.status} />
        ) : undefined
      }
    />
  );
}

function PreviewPane({ hit }: { hit: SearchHit | undefined }) {
  if (hit === undefined) {
    return <p className="text-xs text-muted-foreground">Nothing selected.</p>;
  }
  return (
    <div className="flex flex-col gap-2">
      <div className="font-mono text-xs text-muted-foreground">
        {hitMarker(hit)} · {repoName(hit.repo)}
      </div>
      <h3 className="text-lg/tight font-semibold">{hit.title}</h3>
      {(hit.status !== "" || hit.author !== "") && (
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          {hit.status !== "" && <StatusBadge status={hit.status} />}
          {hit.author !== "" && <span>{hit.author}</span>}
        </div>
      )}
      {hit.snippet !== "" && (
        <p className="text-xs/relaxed text-muted-foreground">
          <Marked text={hit.snippet} />
        </p>
      )}
    </div>
  );
}

function KeyHint({ keys, children }: { keys: string[]; children: ReactNode }) {
  return (
    <span className="flex items-center gap-1">
      {keys.map((key) => (
        <kbd key={key} className="rounded-sm border px-1 font-mono">
          {key}
        </kbd>
      ))}
      {children}
    </span>
  );
}

export function CommandPalette({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const navigate = useNavigate();
  const [q, setQ] = useState("");
  const [filter, setFilter] = useState<PaletteFilter>(ALL);
  const [active, setActive] = useState("");
  const debouncedQ = useDebouncedValue(q, Q_DEBOUNCE_MS);

  const hitsParams: SearchDocsParams = { limit: PALETTE_LIMIT };
  if (debouncedQ !== "") {
    hitsParams.q = debouncedQ;
  }
  if (filter.kind === "repo") {
    hitsParams.repo = filter.value;
  }
  if (filter.kind === "type") {
    hitsParams.type = filter.value;
  }
  const hitsQuery = useSearchDocs(hitsParams, {
    query: { enabled: open, placeholderData: keepPreviousData },
  });
  const result =
    hitsQuery.data?.status === 200 ? hitsQuery.data.data : undefined;

  // The filters' sources come from the query WITHOUT the filter, so
  // every repo and type stays offered while one is selected.
  const facetParams: SearchDocsParams = { limit: 0 };
  if (debouncedQ !== "") {
    facetParams.q = debouncedQ;
  }
  const facetsQuery = useSearchDocs(facetParams, {
    query: { enabled: open, placeholderData: keepPreviousData },
  });
  const facets =
    facetsQuery.data?.status === 200 ? facetsQuery.data.data.facets : undefined;
  const repoFilters = Object.keys(facets?.repo ?? {}).sort();
  const typeFilters = Object.keys(facets?.type ?? {}).sort();

  const hits = useMemo(() => result?.hits ?? [], [result]);

  // Empty-query state leads with recently-opened docs and pages
  // (IMPL-0002 Phase 6, OQ-7a; kind per DESIGN-0004 OQ-5a) —
  // synthesized as snippet-less hits so navigation, preview, and
  // prefetch reuse the same machinery. Values get a `recent:` prefix;
  // the same doc also appearing in the results list below keeps its
  // own key.
  const recentHits = useMemo<SearchHit[]>(
    () => (!open || debouncedQ !== "" ? [] : readRecentDocs().map(recentHit)),
    [open, debouncedQ],
  );

  const entries = useMemo(
    () => [
      ...recentHits.map((hit) => ({ value: `recent:${hitKey(hit)}`, hit })),
      ...hits.map((hit) => ({ value: hitKey(hit), hit })),
    ],
    [recentHits, hits],
  );

  // Keep the highlight on a real entry (adjust-during-render): cmdk's
  // value is controlled, so it never auto-selects — without this,
  // Enter before any ↑/↓/Tab would silently do nothing.
  const firstEntry = entries[0];
  if (
    firstEntry !== undefined &&
    !entries.some((entry) => entry.value === active)
  ) {
    setActive(firstEntry.value);
  }

  const activeHit =
    entries.find((entry) => entry.value === active)?.hit ?? entries[0]?.hit;

  // Warm the reader for the highlighted hit — the same treatment doc
  // links get on hover/focus, so Enter paints without a skeleton.
  const prefetchDoc = usePrefetchDoc();
  const prefetchPage = usePrefetchPage();
  useEffect(() => {
    if (!open || activeHit === undefined) {
      return;
    }
    const [owner, name] = activeHit.repo.split("/");
    if (owner === undefined || name === undefined) {
      return;
    }
    if (activeHit.source === "page") {
      prefetchPage(owner, name, activeHit.path);
    } else {
      prefetchDoc(owner, name, activeHit.type, activeHit.doc_id);
    }
  }, [open, activeHit, prefetchDoc, prefetchPage]);

  const groups = useMemo(() => {
    const byRepo = new Map<string, SearchHit[]>();
    for (const hit of hits) {
      const list = byRepo.get(hit.repo) ?? [];
      list.push(hit);
      byRepo.set(hit.repo, list);
    }
    return [...byRepo.entries()];
  }, [hits]);

  const close = () => {
    onOpenChange(false);
  };

  // Following a hit: cmdk calls this with the item's value on Enter and
  // on a click, and the palette closes before the route changes.
  const openEntry = (value: string) => {
    const hit = entries.find((entry) => entry.value === value)?.hit;
    if (hit === undefined) {
      return;
    }
    close();
    void navigate(hitHref(hit));
  };

  const moveActive = (delta: number) => {
    const keys = entries.map((entry) => entry.value);
    if (keys.length === 0) {
      return;
    }
    const index = Math.max(0, keys.indexOf(active));
    const next = keys[(index + delta + keys.length) % keys.length];
    if (next !== undefined) {
      setActive(next);
    }
  };

  const toggle = (next: PaletteFilter, pressed: boolean) => {
    setFilter(pressed ? next : ALL);
  };

  return (
    <CommandDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Search documents"
      description="Search the documents and pages of every repository."
      // Esc closes, and the footer says so; the corner button would
      // only crowd the input.
      showCloseButton={false}
      commandProps={{
        label: "Search documents",
        // docz-api searches; cmdk must not filter the answer again.
        shouldFilter: false,
        value: active,
        onValueChange: setActive,
        onKeyDown: (event: KeyboardEvent) => {
          // Tab steps the highlight (and with it the preview) without
          // leaving the input; ↑/↓/↵ are cmdk built-ins.
          if (event.key === "Tab") {
            event.preventDefault();
            moveActive(event.shiftKey ? -1 : 1);
          }
        },
      }}
      preview={<PreviewPane hit={activeHit} />}
    >
      <CommandInput
        value={q}
        onValueChange={setQ}
        placeholder="search docs, rfcs, authors…"
      />

      <CommandFilters aria-label="Narrow the search">
        <FilterChip
          pressed={filter.kind === "all"}
          onPressedChange={() => {
            setFilter(ALL);
          }}
        >
          all
          {result === undefined
            ? ""
            : ` ${String(result.estimated_total_hits)}`}
        </FilterChip>
        {repoFilters.map((repo) => (
          <FilterChip
            key={`repo:${repo}`}
            pressed={filter.kind === "repo" && filter.value === repo}
            onPressedChange={(pressed) => {
              toggle({ kind: "repo", value: repo }, pressed);
            }}
          >
            {repoName(repo)}
          </FilterChip>
        ))}
        {typeFilters.map((type) => (
          <FilterChip
            key={`type:${type}`}
            pressed={filter.kind === "type" && filter.value === type}
            onPressedChange={(pressed) => {
              toggle({ kind: "type", value: type }, pressed);
            }}
          >
            <TypeDot type={type} />
            {type}
          </FilterChip>
        ))}
      </CommandFilters>

      <CommandList>
        <CommandEmpty>
          No results{debouncedQ === "" ? "" : ` for “${debouncedQ}”`}.
        </CommandEmpty>
        {recentHits.length > 0 && (
          <CommandGroup heading="recent">
            {recentHits.map((hit) => {
              const value = `recent:${hitKey(hit)}`;
              return (
                <CommandItem key={value} value={value} onSelect={openEntry}>
                  <HitRow hit={hit} showStatus={false} />
                </CommandItem>
              );
            })}
          </CommandGroup>
        )}
        {groups.map(([repo, groupHits]) => (
          <CommandGroup
            key={repo}
            heading={`${repo} — ${String(groupHits.length)} ${
              groupHits.length === 1 ? "match" : "matches"
            }`}
          >
            {groupHits.map((hit) => {
              const value = hitKey(hit);
              return (
                <CommandItem key={value} value={value} onSelect={openEntry}>
                  <HitRow hit={hit} showStatus />
                </CommandItem>
              );
            })}
          </CommandGroup>
        ))}
      </CommandList>

      <CommandFooter>
        <KeyHint keys={["↑", "↓"]}>navigate</KeyHint>
        <KeyHint keys={["↵"]}>open</KeyHint>
        <KeyHint keys={["tab"]}>preview</KeyHint>
        <KeyHint keys={["esc"]}>close</KeyHint>
        <span className="ml-auto font-mono">docz-index</span>
      </CommandFooter>
    </CommandDialog>
  );
}
