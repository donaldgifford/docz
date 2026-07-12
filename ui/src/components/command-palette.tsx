import { keepPreviousData } from "@tanstack/react-query";
import { Command } from "cmdk";
import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router";

import { useSearchDocs } from "@/api/__generated__/docz-api";
import { StatusBadge } from "@/components/badges";
import { Snippet } from "@/components/snippet";
import { usePrefetchDoc } from "@/hooks/usePrefetchDoc";

import type {
  SearchDocsParams,
  SearchHit,
} from "@/api/__generated__/docz-api.schemas";

/*
 * ⌘K palette per the mockup's search modal: input row, facet pills,
 * results grouped by repo on the left, a preview of the highlighted hit
 * on the right (built purely from hit data — no extra fetch), footer
 * hints. Search state here is palette-local (DESIGN-0001: the pills are
 * facet shortcuts applied to the palette query only); the directory URL
 * is untouched until Enter navigates to the reader.
 */

const Q_DEBOUNCE_MS = 200;
const PALETTE_LIMIT = 50;

type PalettePill =
  | { kind: "all" }
  | { kind: "repo"; value: string }
  | { kind: "type"; value: string };

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

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  return (
    target instanceof HTMLInputElement ||
    target instanceof HTMLTextAreaElement ||
    target instanceof HTMLSelectElement ||
    target.isContentEditable
  );
}

// Lowercased: cmdk normalizes item values, so keys must round-trip
// through it unchanged. Navigation resolves the hit through a map and
// uses the original casing.
function hitKey(hit: SearchHit): string {
  return `${hit.repo}/${hit.type}/${hit.doc_id}`.toLowerCase();
}

function PillButton({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={`border px-[0.6rem] py-[0.2rem] font-mono text-[11.5px] ${
        active
          ? "border-(--color-accent-border) bg-(--color-accent-bg) text-accent"
          : "border-border-default text-fg-tertiary hover:text-fg-secondary"
      }`}
    >
      {label}
    </button>
  );
}

function PreviewPane({ hit }: { hit: SearchHit | undefined }) {
  if (hit === undefined) {
    return (
      <p className="font-mono text-[12px] text-fg-muted">Nothing selected.</p>
    );
  }
  return (
    <>
      <div className="font-mono text-[11.5px] text-accent">
        {hit.doc_id} · {hit.repo.split("/").at(-1) ?? hit.repo}
      </div>
      <h3 className="mt-[0.3rem] mb-2 text-[16px] font-semibold text-fg-primary">
        {hit.title}
      </h3>
      <div className="mb-4 flex flex-wrap items-center gap-2 font-mono text-[11.5px] text-fg-tertiary">
        {hit.status !== "" && <StatusBadge status={hit.status} />}
        {hit.author !== "" && (
          <>
            <span className="text-fg-muted">·</span>
            <span>{hit.author}</span>
          </>
        )}
      </div>
      <p className="text-[13px] leading-[1.55] text-fg-tertiary">
        <Snippet snippet={hit.snippet} />
      </p>
    </>
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
  const [pill, setPill] = useState<PalettePill>({ kind: "all" });
  const [active, setActive] = useState("");
  const debouncedQ = useDebouncedValue(q, Q_DEBOUNCE_MS);

  // Global hotkeys: ⌘K/Ctrl+K toggles anywhere; "/" opens unless typing
  // in an editable element; Esc closes.
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key === "k") {
        event.preventDefault();
        onOpenChange(!open);
        return;
      }
      if (!open && event.key === "/" && !isEditableTarget(event.target)) {
        event.preventDefault();
        onOpenChange(true);
        return;
      }
      if (open && event.key === "Escape") {
        event.preventDefault();
        onOpenChange(false);
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open, onOpenChange]);

  const hitsParams: SearchDocsParams = { limit: PALETTE_LIMIT };
  if (debouncedQ !== "") {
    hitsParams.q = debouncedQ;
  }
  if (pill.kind === "repo") {
    hitsParams.repo = pill.value;
  }
  if (pill.kind === "type") {
    hitsParams.type = pill.value;
  }
  const hitsQuery = useSearchDocs(hitsParams, {
    query: { enabled: open, placeholderData: keepPreviousData },
  });
  const result =
    hitsQuery.data?.status === 200 ? hitsQuery.data.data : undefined;

  // Pill sources come from the query WITHOUT the pill filter, so every
  // repo/type stays offered while one is selected.
  const pillParams: SearchDocsParams = { limit: 0 };
  if (debouncedQ !== "") {
    pillParams.q = debouncedQ;
  }
  const pillsQuery = useSearchDocs(pillParams, {
    query: { enabled: open, placeholderData: keepPreviousData },
  });
  const pillFacets =
    pillsQuery.data?.status === 200 ? pillsQuery.data.data.facets : undefined;
  const repoPills = Object.keys(pillFacets?.repo ?? {}).sort();
  const typePills = Object.keys(pillFacets?.type ?? {}).sort();

  const hits = useMemo(() => result?.hits ?? [], [result]);

  // Keep the highlight on a real hit (adjust-during-render): cmdk's
  // value is controlled, so it never auto-selects — without this,
  // Enter before any ↑/↓/Tab would silently do nothing.
  const firstHit = hits[0];
  if (firstHit !== undefined && !hits.some((hit) => hitKey(hit) === active)) {
    setActive(hitKey(firstHit));
  }

  const activeHit = hits.find((hit) => hitKey(hit) === active) ?? hits[0];

  // Warm the reader for the highlighted hit — the same treatment doc
  // links get on hover/focus, so Enter paints without a skeleton.
  const prefetchDoc = usePrefetchDoc();
  useEffect(() => {
    if (!open || activeHit === undefined) {
      return;
    }
    const [owner, name] = activeHit.repo.split("/");
    if (owner !== undefined && name !== undefined) {
      prefetchDoc(owner, name, activeHit.type, activeHit.doc_id);
    }
  }, [open, activeHit, prefetchDoc]);

  const groups = useMemo(() => {
    const byRepo = new Map<string, SearchHit[]>();
    for (const hit of hits) {
      const list = byRepo.get(hit.repo) ?? [];
      list.push(hit);
      byRepo.set(hit.repo, list);
    }
    return [...byRepo.entries()];
  }, [hits]);

  if (!open) {
    return null;
  }

  const openDoc = (key: string) => {
    const hit = hits.find((candidate) => hitKey(candidate) === key);
    if (hit === undefined) {
      return;
    }
    onOpenChange(false);
    void navigate(`/${hit.repo}/${hit.type}/${hit.doc_id}`);
  };

  const moveActive = (delta: number) => {
    const keys = hits.map(hitKey);
    if (keys.length === 0) {
      return;
    }
    const index = Math.max(0, keys.indexOf(active));
    const next = keys[(index + delta + keys.length) % keys.length];
    if (next !== undefined) {
      setActive(next);
    }
  };

  return (
    // Backdrop click is a redundant pointer affordance — Esc is the
    // keyboard path (global listener above), hence no key handler here.
    <div
      data-testid="command-palette"
      role="presentation"
      className="fixed inset-x-0 top-[52px] bottom-0 z-[120] flex items-start justify-center bg-[rgba(8,11,16,0.66)] backdrop-blur-[2px] sm:px-4 sm:pt-[7vh]"
      onClick={(event) => {
        if (event.target === event.currentTarget) {
          onOpenChange(false);
        }
      }}
    >
      <Command
        // The panel is a modal dialog to assistive tech; cmdk's root
        // carries no role of its own, so it takes the dialog semantics.
        role="dialog"
        aria-modal="true"
        aria-label="Search documents"
        label="Search documents"
        shouldFilter={false}
        value={active}
        onValueChange={setActive}
        onKeyDown={(event) => {
          // Tab steps the highlight (and with it the preview) without
          // leaving the input; ↑/↓/↵ are cmdk built-ins.
          if (event.key === "Tab") {
            event.preventDefault();
            moveActive(event.shiftKey ? -1 : 1);
          }
        }}
        // Full-screen below sm; the mockup's floating modal above it.
        className="flex h-full w-full flex-col border-border-strong bg-bg-raised shadow-[0_28px_90px_rgba(0,0,0,0.6)] sm:h-auto sm:max-h-[76vh] sm:w-[min(780px,96vw)] sm:border"
      >
        <div className="flex items-center gap-3 border-b border-border-default px-4 py-[0.85rem]">
          <svg
            aria-hidden
            width="18"
            height="18"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            className="text-fg-tertiary"
          >
            <circle cx="11" cy="11" r="8" />
            <path d="m21 21-4.3-4.3" />
          </svg>
          {/* Focusing the query input on open is the expected modal
              palette behavior (⌘K convention); nothing else is usable
              until it has focus. */}
          {/* eslint-disable jsx-a11y/no-autofocus */}
          <Command.Input
            autoFocus
            value={q}
            onValueChange={setQ}
            placeholder="search docs, rfcs, authors…"
            className="flex-1 bg-transparent font-mono text-[15px] text-fg-primary outline-none placeholder:text-fg-muted"
          />
          {/* eslint-enable jsx-a11y/no-autofocus */}
          <span className="flex items-center gap-[0.4rem] font-mono text-[11px] text-fg-muted">
            <kbd className="border border-border-default px-1">esc</kbd>
            to close
          </span>
        </div>

        <div className="flex flex-wrap gap-[0.4rem] border-b border-border-hairline px-4 py-[0.55rem]">
          <PillButton
            label={`all${result === undefined ? "" : ` ${String(result.estimated_total_hits)}`}`}
            active={pill.kind === "all"}
            onClick={() => {
              setPill({ kind: "all" });
            }}
          />
          {repoPills.map((repo) => (
            <PillButton
              key={`repo:${repo}`}
              label={repo.split("/").at(-1) ?? repo}
              active={pill.kind === "repo" && pill.value === repo}
              onClick={() => {
                setPill(
                  pill.kind === "repo" && pill.value === repo
                    ? { kind: "all" }
                    : { kind: "repo", value: repo },
                );
              }}
            />
          ))}
          {typePills.map((type) => (
            <PillButton
              key={`type:${type}`}
              label={type}
              active={pill.kind === "type" && pill.value === type}
              onClick={() => {
                setPill(
                  pill.kind === "type" && pill.value === type
                    ? { kind: "all" }
                    : { kind: "type", value: type },
                );
              }}
            />
          ))}
        </div>

        <div className="grid min-h-0 flex-1 grid-cols-1 sm:grid-cols-2">
          <Command.List className="max-h-full overflow-y-auto p-[0.3rem] sm:border-r sm:border-border-hairline">
            <Command.Empty className="px-4 py-6 font-mono text-[12.5px] text-fg-tertiary">
              No results{debouncedQ === "" ? "" : ` for “${debouncedQ}”`}.
            </Command.Empty>
            {groups.map(([repo, groupHits]) => (
              <Command.Group
                key={repo}
                heading={`${repo} — ${String(groupHits.length)} ${
                  groupHits.length === 1 ? "match" : "matches"
                }`}
                className="[&_[cmdk-group-heading]]:px-[0.6rem] [&_[cmdk-group-heading]]:pt-[0.6rem] [&_[cmdk-group-heading]]:pb-[0.3rem] [&_[cmdk-group-heading]]:font-mono [&_[cmdk-group-heading]]:text-[10.5px] [&_[cmdk-group-heading]]:tracking-[0.06em] [&_[cmdk-group-heading]]:text-fg-muted [&_[cmdk-group-heading]]:uppercase"
              >
                {groupHits.map((hit) => {
                  const key = hitKey(hit);
                  return (
                    <Command.Item
                      key={key}
                      value={key}
                      onSelect={openDoc}
                      className="cursor-pointer border-l-2 border-l-transparent px-[0.6rem] py-2 data-[selected=true]:border-l-accent data-[selected=true]:bg-bg-elevated"
                    >
                      <div className="mb-[2px] flex items-center justify-between gap-2">
                        <span className="font-mono text-[11.5px] text-fg-tertiary">
                          {hit.doc_id}
                        </span>
                        {hit.status !== "" && (
                          <StatusBadge status={hit.status} />
                        )}
                      </div>
                      <div className="text-[13.5px] text-fg-primary">
                        {hit.title}
                      </div>
                      <div className="mt-[2px] line-clamp-2 text-[12px] leading-[1.45] text-fg-tertiary">
                        <Snippet snippet={hit.snippet} />
                      </div>
                    </Command.Item>
                  );
                })}
              </Command.Group>
            ))}
          </Command.List>

          <div
            data-testid="palette-preview"
            className="hidden overflow-y-auto px-[1.1rem] py-4 sm:block"
          >
            <PreviewPane hit={activeHit} />
          </div>
        </div>

        <div className="flex items-center gap-[1.1rem] border-t border-border-default px-4 py-[0.55rem] font-mono text-[11px] text-fg-muted">
          <span className="flex items-center gap-[0.3rem]">
            <kbd className="border border-border-default px-1">↑</kbd>
            <kbd className="border border-border-default px-1">↓</kbd>
            navigate
          </span>
          <span className="flex items-center gap-[0.3rem]">
            <kbd className="border border-border-default px-1">↵</kbd>
            open
          </span>
          <span className="flex items-center gap-[0.3rem]">
            <kbd className="border border-border-default px-1">tab</kbd>
            preview
          </span>
          <span className="ml-auto">docz-index</span>
        </div>
      </Command>
    </div>
  );
}
