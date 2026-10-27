import { diffLines, type ChangeObject } from "diff";
import { useState } from "react";

import { ReviewButton } from "@/review/controls";

/*
 * What changed since the previous round (DESIGN-0024 section 5). The
 * baseline is a whole-source line diff with unchanged runs collapsed;
 * around it, the frontmatter fields that changed, the regions that were
 * deleted, and a "changed" mark on every region of the rendered document
 * whose own text differs, matched by kind and heading (a nested parent
 * counts only its own lines, not its children's). Diff text is marked
 * `data-no-annotate`, so it cannot be annotated, and past a size bound
 * the line diff is skipped and only the marks are shown.
 */

/** Past this many characters on either side, only the marks are shown. */
export const DIFF_CHAR_LIMIT = 512 * 1024;
/** Past this many edits the diff gives up, and only the marks are shown. */
export const DIFF_EDIT_LIMIT = 4000;
/** Unchanged lines kept around each change before a run collapses. */
const CONTEXT_LINES = 3;

const MARKER = /^\s*<!--\s*docz:([a-z0-9][a-z0-9-]*):(start|end)\s*-->\s*$/;
const FENCE = /^\s*```/;
const HEADING = /^#{1,6}\s+(.*?)\s*#*\s*$/;

export interface SourceRegion {
  kind: string;
  /** The first heading inside the region, or "". */
  heading: string;
  /** Which region of its kind, in document order. */
  ordinal: number;
  /** The region's own lines: inside it and outside every child region. */
  own: string;
}

/** Reads the canonical `docz:` regions straight from a source's markers. */
export function sourceRegions(source: string): SourceRegion[] {
  const lines = source.split("\n");
  interface Open {
    kind: string;
    start: number;
    children: [number, number][];
  }
  const stack: Open[] = [];
  const spans: {
    kind: string;
    start: number;
    end: number;
    children: [number, number][];
  }[] = [];
  let fenced = false;
  lines.forEach((line, i) => {
    if (FENCE.test(line)) {
      fenced = !fenced;
      return;
    }
    const m = fenced ? null : MARKER.exec(line);
    if (m === null) {
      return;
    }
    const [, kind = "", role] = m;
    if (role === "start") {
      stack.push({ kind, start: i, children: [] });
      return;
    }
    const at = stack.map((o) => o.kind).lastIndexOf(kind);
    if (at === -1) {
      return;
    }
    const open = stack[at];
    stack.splice(at);
    if (open === undefined) {
      return;
    }
    spans.push({ ...open, end: i });
    stack.at(-1)?.children.push([open.start, i]);
  });

  spans.sort((a, b) => a.start - b.start);
  const seen = new Map<string, number>();
  return spans.map((span) => {
    const inner = lines.slice(span.start + 1, span.end);
    const own = inner
      .filter((_, j) => {
        const n = span.start + 1 + j;
        return !span.children.some(([s, e]) => n >= s && n <= e);
      })
      .join("\n");
    const heading =
      inner.map((l) => HEADING.exec(l)?.[1]).find((h) => h !== undefined) ?? "";
    const ordinal = seen.get(span.kind) ?? 0;
    seen.set(span.kind, ordinal + 1);
    return { kind: span.kind, heading, ordinal, own };
  });
}

function frontmatter(source: string): Map<string, string> {
  const out = new Map<string, string>();
  const m = /^---\n([\s\S]*?)\n---(?:\n|$)/.exec(source);
  for (const line of m?.[1]?.split("\n") ?? []) {
    const kv = /^([A-Za-z0-9_-]+):\s*(.*)$/.exec(line);
    if (kv?.[1] !== undefined) {
      out.set(kv[1], kv[2] ?? "");
    }
  }
  return out;
}

export interface FieldChange {
  key: string;
  before: string;
  after: string;
}

export interface RoundComparison {
  /** `kind:ordinal` of every current region whose own text changed or is new. */
  changed: Set<string>;
  /** Previous regions with no counterpart now. */
  deleted: { kind: string; heading: string }[];
  frontmatter: FieldChange[];
  /** The line diff, or null when it was past the size bound. */
  lines: ChangeObject<string>[] | null;
}

// Pairs regions by kind and heading, the nth with the nth.
function keyed(regions: SourceRegion[]): Map<string, SourceRegion> {
  const count = new Map<string, number>();
  const out = new Map<string, SourceRegion>();
  for (const r of regions) {
    const base = `${r.kind}\u0000${r.heading}`;
    const n = count.get(base) ?? 0;
    count.set(base, n + 1);
    out.set(`${base}\u0000${String(n)}`, r);
  }
  return out;
}

export function compareRounds(
  previous: string,
  current: string,
): RoundComparison {
  const before = keyed(sourceRegions(previous));
  const after = keyed(sourceRegions(current));
  const changed = new Set<string>();
  for (const [key, region] of after) {
    if (before.get(key)?.own !== region.own) {
      changed.add(`${region.kind}:${String(region.ordinal)}`);
    }
  }
  const deleted = [...before]
    .filter(([key]) => !after.has(key))
    .map(([, r]) => ({ kind: r.kind, heading: r.heading }));

  const fmBefore = frontmatter(previous);
  const fmAfter = frontmatter(current);
  const fields: FieldChange[] = [];
  for (const key of new Set([...fmBefore.keys(), ...fmAfter.keys()])) {
    const b = fmBefore.get(key) ?? "";
    const a = fmAfter.get(key) ?? "";
    if (a !== b) {
      fields.push({ key, before: b, after: a });
    }
  }

  const small =
    previous.length <= DIFF_CHAR_LIMIT && current.length <= DIFF_CHAR_LIMIT;
  const lines = small
    ? (diffLines(previous, current, { maxEditLength: DIFF_EDIT_LIMIT }) ?? null)
    : null;
  return { changed, deleted, frontmatter: fields, lines };
}

/** Marks each changed region section in the rendered document. */
export function markChangedRegions(
  root: HTMLElement,
  changed: Set<string>,
): void {
  for (const section of root.querySelectorAll<HTMLElement>(
    "section[data-region]",
  )) {
    const key = `${section.dataset.region ?? ""}:${section.dataset.ordinal ?? ""}`;
    if (changed.has(key)) {
      section.dataset.changed = "true";
    } else {
      delete section.dataset.changed;
    }
  }
}

interface Hunk {
  kind: "added" | "removed" | "same";
  lines: string[];
}

function hunks(changes: readonly ChangeObject<string>[]): Hunk[] {
  return changes.map((c) => ({
    kind: c.added ? "added" : c.removed ? "removed" : "same",
    lines: c.value.replace(/\n$/, "").split("\n"),
  }));
}

function SameRun({
  lines,
  first,
  last,
}: {
  lines: string[];
  first: boolean;
  last: boolean;
}) {
  const [open, setOpen] = useState(false);
  const head = first ? 0 : CONTEXT_LINES;
  const tail = last ? 0 : CONTEXT_LINES;
  if (open || lines.length <= head + tail + 1) {
    return (
      <>
        {lines.map((l, i) => (
          <DiffLine key={i} kind="same" text={l} />
        ))}
      </>
    );
  }
  const hidden = lines.length - head - tail;
  return (
    <>
      {lines.slice(0, head).map((l, i) => (
        <DiffLine key={`h${String(i)}`} kind="same" text={l} />
      ))}
      <ReviewButton
        className="my-1"
        onClick={() => {
          setOpen(true);
        }}
      >
        Show {hidden} unchanged lines
      </ReviewButton>
      {lines.slice(lines.length - tail).map((l, i) => (
        <DiffLine key={`t${String(i)}`} kind="same" text={l} />
      ))}
    </>
  );
}

function DiffLine({ kind, text }: { kind: Hunk["kind"]; text: string }) {
  const look = {
    added:
      "bg-[color-mix(in_srgb,var(--color-st-active)_12%,transparent)] text-fg-primary",
    removed:
      "bg-[color-mix(in_srgb,var(--color-st-rejected)_12%,transparent)] text-fg-secondary",
    same: "text-fg-tertiary",
  }[kind];
  const sign = { added: "+", removed: "-", same: " " }[kind];
  return (
    <div data-diff={kind} className={`whitespace-pre-wrap ${look}`}>
      {sign} {text}
    </div>
  );
}

export function RoundDiff({ comparison }: { comparison: RoundComparison }) {
  const { lines, deleted, frontmatter: fields, changed } = comparison;
  const parts = lines === null ? [] : hunks(lines);
  return (
    <section
      data-no-annotate
      aria-label="Changes since the previous round"
      className="flex flex-col gap-2 border border-border-hairline bg-bg-raised p-3"
    >
      <p className="font-mono text-[12.5px] text-fg-secondary">
        {changed.size === 0
          ? "No region changed since the previous round."
          : `${String(changed.size)} region${changed.size === 1 ? "" : "s"} changed since the previous round.`}
      </p>
      {fields.length > 0 && (
        <ul
          aria-label="Frontmatter changes"
          className="font-mono text-[12.5px]"
        >
          {fields.map((f) => (
            <li key={f.key}>
              {f.key}: {f.before === "" ? "(none)" : f.before} →{" "}
              {f.after === "" ? "(removed)" : f.after}
            </li>
          ))}
        </ul>
      )}
      {deleted.length > 0 && (
        <ul aria-label="Deleted regions" className="font-mono text-[12.5px]">
          {deleted.map((d, i) => (
            <li key={i}>
              deleted: {d.kind}
              {d.heading === "" ? "" : ` (${d.heading})`}
            </li>
          ))}
        </ul>
      )}
      {lines === null ? (
        <p className="font-mono text-[12.5px] text-fg-tertiary">
          The change is too large to show line by line; the changed regions are
          marked.
        </p>
      ) : (
        <div className="max-h-[50vh] overflow-auto bg-code-bg p-2 font-mono text-[12.5px]">
          {parts.map((h, i) =>
            h.kind === "same" ? (
              <SameRun
                key={i}
                lines={h.lines}
                first={i === 0}
                last={i === parts.length - 1}
              />
            ) : (
              h.lines.map((l, j) => (
                <DiffLine
                  key={`${String(i)}-${String(j)}`}
                  kind={h.kind}
                  text={l}
                />
              ))
            ),
          )}
        </div>
      )}
    </section>
  );
}
