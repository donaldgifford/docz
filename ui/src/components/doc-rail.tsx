import { getGetDocUrl } from "@/api/__generated__/docz-api";
import { statusColor } from "@/lib/colors";

import type { Document } from "@/api/__generated__/docz-api.schemas";
import type { TocEntry } from "@/markdown/processor";

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
    return (
      <p className="font-mono text-[11px] text-fg-muted">no sections</p>
    );
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

function MetaRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex justify-between gap-3 py-[5px] text-[12px]">
      <span className="whitespace-nowrap text-fg-tertiary">{label}</span>
      <span className="min-w-0 text-right break-words text-fg-primary">
        {children}
      </span>
    </div>
  );
}

/**
 * Right-rail sections for the reader: "On this page" is rendered by the
 * route (sticky wide / disclosure narrow); this component carries the
 * trimmed metadata card and the formats list. Empty-string fields are
 * omitted — "" means unset in the docz-api contract.
 */
export function DocRailInfo({
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
    <>
      <section className="mb-8">
        <SidebarHeading>Metadata</SidebarHeading>
        {doc.status !== "" && (
          <MetaRow label="Status">
            <span style={{ color: statusColor(doc.status) }}>
              {doc.status}
            </span>
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
