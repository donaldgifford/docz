import { useListTypes } from "@/api/__generated__/docz-api";
import { useActiveHeading } from "@/hooks/useActiveHeading";
import { arr } from "@/lib/wire";

import type { TocEntry } from "@/markdown/processor";

export type DocFormat = "html" | "md";

/*
 * The outline rail. Rows hang off a hairline and the section being read
 * colors its own 2px segment of it (the portal reference's treatment) —
 * position in a long document is otherwise invisible. The active id
 * comes from a scroll spy, so it is undefined until a heading reaches
 * the top band and wherever IntersectionObserver is missing.
 */
export function TocList({ toc }: { toc: TocEntry[] }) {
  const activeId = useActiveHeading(toc.map((entry) => entry.id));

  if (toc.length === 0) {
    return <p className="font-mono text-[12px] text-fg-muted">no sections</p>;
  }
  return (
    <ul className="border-l border-border-hairline">
      {toc.map((entry) => {
        const isActive = entry.id === activeId;
        return (
          <li key={entry.id}>
            <a
              href={`#${entry.id}`}
              aria-current={isActive ? "location" : undefined}
              className={`-ml-px block border-l-2 py-[3px] ${
                entry.depth > 2 ? "pl-6 text-[12.5px]" : "pl-3 text-[13px]"
              } ${
                isActive
                  ? "border-accent text-accent"
                  : "border-transparent text-fg-tertiary hover:text-fg-primary"
              }`}
            >
              {entry.text}
            </a>
          </li>
        );
      })}
    </ul>
  );
}

const STOP_DOT: Record<"done" | "current" | "pending", string> = {
  done: "border-(--color-st-active) bg-(--color-st-active)",
  current:
    "border-(--color-st-draft) bg-(--color-st-draft) shadow-[0_0_0_3px_color-mix(in_srgb,var(--color-st-draft)_18%,transparent)]",
  pending: "border-fg-muted bg-bg-base",
};

/**
 * Position-only lifecycle: the type's `statuses` from `.docz.yaml` (via
 * listTypes, cached per repo) with the doc's status as the active stop.
 * No dates — docz-api doesn't expose lifecycle timestamps yet
 * (DESIGN-0001 additive ask). Renders nothing when the type is unknown
 * or declares no statuses — including its disclosure shell, so an
 * unknown type never leaves an empty box. Since IMPL-0002 Phase 5
 * (OQ-4a) it is a closed-by-default disclosure under the reader's
 * metadata table (the nav-drawer pattern), not an always-open rail
 * block.
 */
export function LifecycleRail({
  owner,
  name,
  typeName,
  currentStatus,
}: {
  owner: string;
  name: string;
  typeName: string;
  currentStatus: string;
}) {
  const typesQuery = useListTypes(owner, name, {
    // Type definitions change on .docz.yaml edits, not per navigation.
    query: { staleTime: 5 * 60_000 },
  });
  const types =
    typesQuery.data?.status === 200 ? typesQuery.data.data.types : undefined;
  const docType = arr(types).find(
    (t) => t.name === typeName || arr(t.aliases).includes(typeName),
  );
  const stages = arr(docType?.statuses);
  if (docType === undefined || stages.length === 0) {
    return null;
  }

  const current = stages.findIndex(
    (status) => status.toLowerCase() === currentStatus.toLowerCase(),
  );

  return (
    <details
      className="mb-6 border border-border-hairline px-4 py-3"
      data-testid="lifecycle-disclosure"
    >
      <summary className="cursor-pointer font-mono text-[11px] tracking-[0.14em] text-fg-muted uppercase">
        Lifecycle · {currentStatus === "" ? "unset" : currentStatus}
      </summary>
      <div
        className="relative mt-4 ml-1 border-l border-border-default pl-[1.1rem]"
        data-testid="lifecycle-rail"
      >
        {stages.map((stage, index) => {
          const state =
            current === -1
              ? "pending"
              : index < current
                ? "done"
                : index === current
                  ? "current"
                  : "pending";
          return (
            <div key={stage} className="relative pb-4 last:pb-1">
              <span
                aria-hidden
                className={`absolute top-[5px] left-[calc(-1.1rem-5px)] size-[9px] rounded-pill border-2 ${STOP_DOT[state]}`}
              />
              <span
                data-lifecycle-state={state}
                className={`font-mono text-[13px] ${
                  state === "pending" ? "text-fg-muted" : "text-fg-primary"
                }`}
              >
                {stage}
              </span>
            </div>
          );
        })}
      </div>
    </details>
  );
}
