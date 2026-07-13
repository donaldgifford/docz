import { useState } from "react";
import { NavLink, useParams } from "react-router";

import { useGetRepo, useListDocs } from "@/api/__generated__/docz-api";
import { usePrefetchDoc } from "@/hooks/usePrefetchDoc";
import { useRepoFacts } from "@/hooks/useRepoFacts";
import { resolveDocType } from "@/lib/docTypes";
import { arr } from "@/lib/wire";

import type { DocType } from "@/api/__generated__/docz-api.schemas";

/*
 * Shared left-rail repo nav per the mockup's TechDocs-style repo view:
 * identity header (letter mark, name, branch · docz.yaml), Home, then
 * one item per configured doc type with its count. Each type's docs
 * live in a collapsible drawer — collapsed by default so a repo with
 * dozens of docs doesn't turn the rail into one long scroll. The
 * route's active type auto-expands (and manual toggles reset when the
 * active type changes); the caret peeks into other types without
 * navigating. Counts come from useRepoFacts — the same source as the
 * repos grid and directory facets, so numbers agree everywhere.
 * Active states fall out of NavLink route matching.
 */

const FIVE_MINUTES = 5 * 60_000;

function navItemClass({ isActive }: { isActive: boolean }): string {
  return `flex min-w-0 flex-1 justify-between gap-3 px-[0.45rem] py-[0.28rem] ${
    isActive
      ? "bg-(--color-accent-bg) text-accent"
      : "text-fg-tertiary hover:bg-bg-raised hover:text-fg-primary"
  }`;
}

function docLinkClass({ isActive }: { isActive: boolean }): string {
  return `block overflow-hidden py-[0.17rem] pr-[0.45rem] pl-[1.15rem] text-[11.5px] text-ellipsis whitespace-nowrap ${
    isActive ? "text-accent" : "text-fg-muted hover:text-fg-primary"
  }`;
}

function NavTypeSection({
  owner,
  name,
  docType,
  count,
  expanded,
  onToggle,
}: {
  owner: string;
  name: string;
  docType: DocType;
  count: number | undefined;
  expanded: boolean;
  onToggle: () => void;
}) {
  // Docs are only fetched once the drawer opens — a collapsed rail
  // costs one listDocs per repo view instead of one per type.
  const docsQuery = useListDocs(owner, name, docType.name, {
    query: { staleTime: FIVE_MINUTES, enabled: expanded },
  });
  const prefetchDoc = usePrefetchDoc();
  const docs =
    docsQuery.data?.status === 200 ? arr(docsQuery.data.data.docs) : undefined;

  return (
    <>
      <div className="flex items-stretch">
        {/* Links use the canonical type name; the API also resolves
            id_prefix/alias URLs. */}
        <NavLink
          to={`/${owner}/${name}/${docType.name}`}
          end
          className={navItemClass}
        >
          <span className="truncate">{docType.name}</span>{" "}
          <span className="text-[11px] text-fg-muted">{count ?? ""}</span>
        </NavLink>
        <button
          type="button"
          aria-expanded={expanded}
          aria-label={`${docType.name} documents`}
          disabled={count === 0}
          onClick={onToggle}
          className="w-6 flex-none cursor-pointer text-center text-[10px] text-fg-muted hover:bg-bg-raised hover:text-fg-primary disabled:cursor-default disabled:opacity-40"
        >
          <span aria-hidden>{expanded ? "▾" : "▸"}</span>
        </button>
      </div>
      {expanded && docs !== undefined && docs.length > 0 && (
        <div className="mt-px mb-1">
          {docs.map((doc) => {
            const prefetch = () => {
              prefetchDoc(owner, name, docType.name, doc.doc_id);
            };
            return (
              <NavLink
                key={doc.doc_id}
                to={`/${owner}/${name}/${docType.name}/${doc.doc_id}`}
                end
                title={doc.title}
                onMouseEnter={prefetch}
                onFocus={prefetch}
                className={docLinkClass}
              >
                {doc.doc_id} · {doc.title}
              </NavLink>
            );
          })}
        </div>
      )}
    </>
  );
}

export function RepoNav({ owner, name }: { owner: string; name: string }) {
  const repoId = `${owner}/${name}`;
  const repoQuery = useGetRepo(owner, name);
  const detail =
    repoQuery.data?.status === 200 ? repoQuery.data.data : undefined;
  const { facts } = useRepoFacts(repoId);

  // The route's `:type` segment (possibly an alias) picks the type
  // whose drawer follows navigation.
  const params = useParams<{ type?: string }>();
  const types = arr(detail?.types);
  const activeTypeName =
    params.type === undefined
      ? undefined
      : resolveDocType(types, params.type)?.name;

  // Manual open/close overrides, reset whenever navigation moves to a
  // different type (adjust-during-render, not an effect).
  const [overrides, setOverrides] = useState<Record<string, boolean>>({});
  const [prevActive, setPrevActive] = useState(activeTypeName);
  if (prevActive !== activeTypeName) {
    setPrevActive(activeTypeName);
    setOverrides({});
  }

  return (
    <nav
      aria-label={`${repoId} navigation`}
      className="font-mono text-[12.5px]"
    >
      <div className="mb-4 flex items-center gap-[0.55rem]">
        <span
          aria-hidden
          className="grid size-[22px] flex-none place-items-center border border-border-default bg-bg-elevated text-[11px] text-accent"
        >
          {name.charAt(0).toUpperCase()}
        </span>
        <div className="min-w-0">
          <div className="truncate leading-[1.2] font-semibold text-fg-primary">
            {repoId}
          </div>
          <div className="text-[10.5px] text-fg-muted">
            {detail === undefined
              ? "…"
              : `${detail.default_branch} · docz.yaml`}
          </div>
        </div>
      </div>

      <NavLink to={`/${repoId}`} end className={navItemClass}>
        <span>Home</span>{" "}
        <span className="text-[11px] text-fg-muted">index.md</span>
      </NavLink>

      <div className="mt-[1.15rem] mb-[0.45rem] border-b border-border-hairline pb-[0.4rem] text-[10px] tracking-[0.14em] text-fg-muted uppercase">
        doc types
      </div>

      {types.map((docType) => (
        <NavTypeSection
          key={docType.name}
          owner={owner}
          name={name}
          docType={docType}
          // Facets omit zero-hit types, so once facts have loaded a
          // missing key IS a zero (drives count text and the disabled
          // caret for empty drawers).
          count={
            facts === undefined
              ? undefined
              : (facts.typeCounts[docType.name] ?? 0)
          }
          expanded={overrides[docType.name] ?? docType.name === activeTypeName}
          onToggle={() => {
            setOverrides((prev) => ({
              ...prev,
              [docType.name]: !(
                prev[docType.name] ?? docType.name === activeTypeName
              ),
            }));
          }}
        />
      ))}
    </nav>
  );
}
