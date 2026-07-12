import { getGetDocUrl, useListTypes } from "@/api/__generated__/docz-api";
import { statusColor } from "@/lib/colors";
import { arr } from "@/lib/wire";

import type { Document } from "@/api/__generated__/docz-api.schemas";
import type { TocEntry } from "@/markdown/processor";
import type { ReactNode } from "react";

export type DocFormat = "html" | "md";

function SidebarHeading({ children }: { children: string }) {
  return (
    <div className="mb-3 border-b border-border-hairline pb-2 font-mono text-[10px] tracking-[0.14em] text-fg-muted uppercase">
      {children}
    </div>
  );
}

export function TocList({ toc }: { toc: TocEntry[] }) {
  if (toc.length === 0) {
    return <p className="font-mono text-[11px] text-fg-muted">no sections</p>;
  }
  return (
    <ul>
      {toc.map((entry) => (
        <li
          key={entry.id}
          className={entry.depth > 2 ? "py-[3px] pl-3" : "py-[3px]"}
        >
          <a
            href={`#${entry.id}`}
            className={`block text-fg-tertiary hover:text-fg-primary ${
              entry.depth > 2 ? "text-[11.5px]" : "text-[12px]"
            }`}
          >
            {entry.text}
          </a>
        </li>
      ))}
    </ul>
  );
}

function MetaRow({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex justify-between gap-3 py-[5px] text-[12px]">
      <span className="whitespace-nowrap text-fg-tertiary">{label}</span>
      <span className="min-w-0 text-right break-words text-fg-primary">
        {children}
      </span>
    </div>
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
 * or declares no statuses.
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
    <section className="mb-8" data-testid="lifecycle-rail">
      <SidebarHeading>Lifecycle</SidebarHeading>
      <div className="relative ml-1 border-l border-border-default pl-[1.1rem]">
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
                className={`font-mono text-[12px] ${
                  state === "pending" ? "text-fg-muted" : "text-fg-primary"
                }`}
              >
                {stage}
              </span>
            </div>
          );
        })}
      </div>
    </section>
  );
}

/**
 * Right-rail sections for the reader: "On this page" is rendered by the
 * route (sticky wide / disclosure narrow); this component carries the
 * trimmed metadata card and the formats list, with the lifecycle rail
 * slotted between them per the mockup's section order. Empty-string
 * fields are omitted — "" means unset in the docz-api contract.
 */
export function DocRailInfo({
  doc,
  format,
  onFormatChange,
  lifecycle,
}: {
  doc: Document;
  format: DocFormat;
  onFormatChange: (format: DocFormat) => void;
  lifecycle?: ReactNode;
}) {
  const [owner = "", name = ""] = doc.repo.split("/", 2);
  const jsonUrl = getGetDocUrl(owner, name, doc.type, doc.doc_id);

  return (
    <>
      <section className="mb-8">
        <SidebarHeading>Metadata</SidebarHeading>
        {doc.status !== "" && (
          <MetaRow label="Status">
            <span style={{ color: statusColor(doc.status) }}>{doc.status}</span>
          </MetaRow>
        )}
        {doc.author !== "" && <MetaRow label="Author">{doc.author}</MetaRow>}
        {doc.created !== "" && <MetaRow label="Created">{doc.created}</MetaRow>}
        {doc.updated_at !== "" && (
          <MetaRow label="Updated">{doc.updated_at.slice(0, 10)}</MetaRow>
        )}
        {doc.git_sha !== "" && (
          <MetaRow label="Commit">
            <span className="font-mono">{doc.git_sha.slice(0, 7)}</span>
          </MetaRow>
        )}
        <div className="pt-2 text-[12px]">
          <a
            href={jsonUrl}
            className="text-accent hover:underline [text-underline-offset:3px]"
          >
            all fields · json →
          </a>
        </div>
      </section>

      {lifecycle}

      <section className="mb-8">
        <SidebarHeading>Formats</SidebarHeading>
        <div className="text-[12px]">
          <button
            type="button"
            onClick={() => {
              onFormatChange("html");
            }}
            className={`block w-full py-[3px] text-left ${
              format === "html"
                ? "text-fg-primary"
                : "text-fg-tertiary hover:text-fg-primary"
            }`}
          >
            <b className="font-medium text-accent">html</b> · read here
          </button>
          <button
            type="button"
            onClick={() => {
              onFormatChange("md");
            }}
            className={`block w-full py-[3px] text-left ${
              format === "md"
                ? "text-fg-primary"
                : "text-fg-tertiary hover:text-fg-primary"
            }`}
          >
            <b className="font-medium text-accent">md</b> · source
          </button>
          <a
            href={jsonUrl}
            className="block truncate py-[3px] text-fg-tertiary hover:text-fg-primary"
          >
            <b className="font-medium text-accent">json</b> · {jsonUrl}
          </a>
        </div>
      </section>
    </>
  );
}
