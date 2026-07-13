import { Link } from "react-router";

import { getGetDocUrl } from "@/api/__generated__/docz-api";

import type { Document } from "@/api/__generated__/docz-api.schemas";
import type { DocFormat } from "@/components/doc-rail";
import type { ReactNode } from "react";

/*
 * Reader metadata header (IMPL-0002 Phase 5, OQ-3a/OQ-5a): the
 * mockup's bordered key/value table directly under the doc header —
 * long right-rail ToCs were burying metadata and the format switch
 * below the rail's fold. Status/author/updated stay on the header
 * meta line; this table carries the rest of what the Document DTO
 * exposes today. Relationship rows, tags, and lifecycle dates slot in
 * here when the DESIGN-0001 additive API asks land. Empty-string
 * fields are omitted — "" means unset in the docz-api contract.
 */

function MetaRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <tr className="border-b border-border-hairline last:border-b-0">
      <th
        scope="row"
        className="w-[110px] py-[7px] pr-4 pl-3 text-left align-top font-mono text-[10px] font-medium tracking-[0.1em] text-fg-muted uppercase"
      >
        {label}
      </th>
      <td className="min-w-0 py-[7px] pr-3 text-[12.5px] break-words text-fg-primary">
        {children}
      </td>
    </tr>
  );
}

function FormatButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: string;
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={`border px-2 py-[2px] ${
        active
          ? "border-(--color-accent-border) bg-(--color-accent-bg) text-accent"
          : "border-border-default text-fg-tertiary hover:text-fg-primary"
      }`}
    >
      {children}
    </button>
  );
}

export function DocMetaTable({
  doc,
  format,
  onFormatChange,
}: {
  doc: Document;
  format: DocFormat;
  onFormatChange: (format: DocFormat) => void;
}) {
  const [owner = "", name = ""] = doc.repo.split("/", 2);
  const jsonUrl = getGetDocUrl(owner, name, doc.type, doc.doc_id);

  return (
    <div className="mb-6">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <span className="font-mono text-[10px] tracking-[0.14em] text-fg-muted uppercase">
          Metadata
        </span>
        <div
          role="group"
          aria-label="Document format"
          className="flex items-center gap-1 font-mono text-[11px]"
        >
          <FormatButton
            active={format === "html"}
            onClick={() => {
              onFormatChange("html");
            }}
          >
            html
          </FormatButton>
          <FormatButton
            active={format === "md"}
            onClick={() => {
              onFormatChange("md");
            }}
          >
            md
          </FormatButton>
          <a
            href={jsonUrl}
            className="border border-border-default px-2 py-[2px] text-fg-tertiary hover:text-fg-primary"
          >
            json
          </a>
        </div>
      </div>
      <table
        aria-label="Document metadata"
        className="w-full border-collapse border border-border-hairline bg-bg-raised"
      >
        <tbody>
          <MetaRow label="Type">
            {doc.type} · {doc.doc_id}
          </MetaRow>
          <MetaRow label="Repo">
            <Link
              to={`/${doc.repo}`}
              className="text-accent underline [text-underline-offset:3px] decoration-(--color-accent-border) hover:decoration-(--color-accent)"
            >
              {doc.repo}
            </Link>
          </MetaRow>
          <MetaRow label="Source">
            <span className="font-mono text-[11.5px]">{doc.path}</span>
          </MetaRow>
          {doc.created !== "" && (
            <MetaRow label="Created">{doc.created}</MetaRow>
          )}
          {doc.git_sha !== "" && (
            <MetaRow label="Commit">
              <span className="font-mono">{doc.git_sha.slice(0, 7)}</span>
            </MetaRow>
          )}
        </tbody>
      </table>
    </div>
  );
}
