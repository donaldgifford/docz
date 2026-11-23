import {
  SegmentedControl,
  type SegmentOption,
} from "@donaldgifford/design-system/blocks/segmented-control";
import { buttonVariants } from "@donaldgifford/design-system/components/button";
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
 *
 * The format switch is the design system's SegmentedControl
 * (DESIGN-0028 §7): two views of one document. `json` is not a third
 * view but a link out to the API's own answer, so it sits beside it.
 */

const FORMATS: readonly SegmentOption<DocFormat>[] = [
  { value: "html", label: "html" },
  { value: "md", label: "md" },
];

function MetaRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <tr className="border-b border-border last:border-b-0">
      <th
        scope="row"
        className="w-[110px] py-[7px] pr-4 pl-3 text-left align-top font-mono text-[11px] font-medium tracking-[0.1em] text-muted-foreground uppercase"
      >
        {label}
      </th>
      <td className="min-w-0 py-[7px] pr-3 text-[13.5px] break-words text-foreground">
        {children}
      </td>
    </tr>
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
        <span className="text-[11px] font-medium tracking-[0.08em] text-muted-foreground uppercase">
          Metadata
        </span>
        <div className="flex items-center gap-2 font-mono">
          <SegmentedControl
            label="Document format"
            options={FORMATS}
            value={format}
            onValueChange={onFormatChange}
          />
          <a
            href={jsonUrl}
            className={buttonVariants({ variant: "outline", size: "sm" })}
          >
            json
          </a>
        </div>
      </div>
      <table
        aria-label="Document metadata"
        className="w-full border-collapse border border-border bg-card"
      >
        <tbody>
          <MetaRow label="Type">
            {doc.type} · {doc.doc_id}
          </MetaRow>
          <MetaRow label="Repo">
            <Link
              to={`/${doc.repo}`}
              className="text-primary underline underline-offset-3 decoration-primary/40 hover:decoration-primary"
            >
              {doc.repo}
            </Link>
          </MetaRow>
          <MetaRow label="Source">
            <span className="font-mono text-[12.5px]">{doc.path}</span>
          </MetaRow>
          {doc.created !== "" && (
            <MetaRow label="Created">{doc.created}</MetaRow>
          )}
        </tbody>
      </table>
    </div>
  );
}
