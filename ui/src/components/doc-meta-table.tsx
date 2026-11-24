import {
  DetailsGrid,
  type Detail,
} from "@donaldgifford/design-system/blocks/details-grid";
import {
  SegmentedControl,
  type SegmentOption,
} from "@donaldgifford/design-system/blocks/segmented-control";
import { buttonVariants } from "@donaldgifford/design-system/components/button";
import { useId } from "react";
import { Link } from "react-router";

import { getGetDocUrl } from "@/api/__generated__/docz-api";

import type { Document } from "@/api/__generated__/docz-api.schemas";
import type { DocFormat } from "@/components/doc-rail";

/*
 * Reader metadata header (IMPL-0002 Phase 5, OQ-3a/OQ-5a): the
 * document's facts directly under the doc header — long right-rail
 * ToCs were burying metadata and the format switch below the rail's
 * fold. Status/author/updated stay on the header meta line; this
 * carries the rest of what the Document DTO exposes today, as the
 * design system's DetailsGrid in its `list` layout, where a value
 * wraps (a source path is long). Relationship rows, tags, and
 * lifecycle dates slot in here when the DESIGN-0001 additive API asks
 * land. Empty-string fields are omitted — "" means unset in the
 * docz-api contract.
 *
 * The format switch is the design system's SegmentedControl
 * (DESIGN-0028 §7): two views of one document. `json` is not a third
 * view but a link out to the API's own answer, so it sits beside it.
 */

const FORMATS: readonly SegmentOption<DocFormat>[] = [
  { value: "html", label: "html" },
  { value: "md", label: "md" },
];

export function DocMetaTable({
  doc,
  format,
  onFormatChange,
}: {
  doc: Document;
  format: DocFormat;
  onFormatChange: (format: DocFormat) => void;
}) {
  const headingId = useId();
  const [owner = "", name = ""] = doc.repo.split("/", 2);
  const jsonUrl = getGetDocUrl(owner, name, doc.type, doc.doc_id);

  const details: Detail[] = [
    { label: "Type", value: `${doc.type} · ${doc.doc_id}` },
    {
      label: "Repo",
      value: (
        <Link
          to={`/${doc.repo}`}
          className="text-primary underline decoration-primary/40 underline-offset-3 hover:decoration-primary"
        >
          {doc.repo}
        </Link>
      ),
    },
    {
      label: "Source",
      value: <span className="font-mono text-xs">{doc.path}</span>,
    },
  ];
  if (doc.created !== "") {
    details.push({ label: "Created", value: doc.created });
  }

  return (
    <section aria-labelledby={headingId} className="mb-6">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <span
          id={headingId}
          className="text-label font-medium tracking-label text-muted-foreground uppercase"
        >
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
      <DetailsGrid
        layout="list"
        items={details}
        className="rounded-lg border bg-card px-3 py-2.5"
      />
    </section>
  );
}
