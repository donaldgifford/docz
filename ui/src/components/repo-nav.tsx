import { NavLink } from "react-router";

import { useGetRepo, useListDocs } from "@/api/__generated__/docz-api";
import { useRepoFacts } from "@/hooks/useRepoFacts";

import type { DocType } from "@/api/__generated__/docz-api.schemas";

/*
 * Shared left-rail repo nav per the mockup's TechDocs-style repo view:
 * identity header (letter mark, name, branch · docz.yaml), Home, then
 * one item per configured doc type with its count and the type's docs
 * nested beneath. Counts come from useRepoFacts — the same source as
 * the repos grid and directory facets, so numbers agree everywhere.
 * Active states fall out of NavLink route matching: the type item stays
 * lit while one of its docs is open; doc links match exactly.
 */

const FIVE_MINUTES = 5 * 60_000;

function navItemClass({ isActive }: { isActive: boolean }): string {
  return `flex justify-between gap-3 px-[0.45rem] py-[0.28rem] ${
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
}: {
  owner: string;
  name: string;
  docType: DocType;
  count: number | undefined;
}) {
  const docsQuery = useListDocs(owner, name, docType.name, {
    query: { staleTime: FIVE_MINUTES },
  });
  const docs =
    docsQuery.data?.status === 200 ? docsQuery.data.data.docs : undefined;

  return (
    <>
      {/* Links use the canonical type name; the API also resolves
          id_prefix/alias URLs. */}
      <NavLink
        to={`/${owner}/${name}/${docType.name}`}
        end
        className={navItemClass}
      >
        <span>{docType.name}</span>{" "}
        <span className="text-[11px] text-fg-muted">
          {count ?? docs?.length ?? ""}
        </span>
      </NavLink>
      {docs !== undefined && docs.length > 0 && (
        <div className="mt-px mb-1">
          {docs.map((doc) => (
            <NavLink
              key={doc.doc_id}
              to={`/${owner}/${name}/${docType.name}/${doc.doc_id}`}
              end
              title={doc.title}
              className={docLinkClass}
            >
              {doc.doc_id} · {doc.title}
            </NavLink>
          ))}
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

      {(detail?.types ?? []).map((docType) => (
        <NavTypeSection
          key={docType.name}
          owner={owner}
          name={name}
          docType={docType}
          count={facts?.typeCounts[docType.name]}
        />
      ))}
    </nav>
  );
}
