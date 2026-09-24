import type { CSSProperties } from "react";

import { statusColor } from "@/lib/colors";

/*
 * Status badges. The color system supplies `--c` inline, so unknown
 * custom statuses work with zero configuration (DESIGN-0001: "the
 * color system, not the type system, interprets").
 *
 * There is deliberately no type badge. Listings carry the doc id
 * (DESIGN-0001, IMPL-0005), which already spells the type out, and a
 * second colored chip beside the status made every row read as two
 * competing signals — the DESIGN-0005 dial-in dropped it. Type color
 * still exists and still identifies the filter chips.
 */

function colorVar(color: string): CSSProperties {
  return { "--c": color } as CSSProperties;
}

/** Dot + label, used in listings and metadata rows. */
export function StatusBadge({ status }: { status: string }) {
  return (
    <span
      style={colorVar(statusColor(status))}
      className="inline-flex items-center gap-[5px] font-mono text-[12px] tracking-[0.03em] text-(--c)"
    >
      <span
        aria-hidden
        data-testid="status-dot"
        className="size-[6px] rounded-pill bg-(--c)"
      />
      {status}
    </span>
  );
}

/**
 * Bordered uppercase pill: the doc's state, and the one colored thing
 * in a listing row. Used next to the reader's title and in every
 * directory hit.
 */
export function StatusPill({ status }: { status: string }) {
  return (
    <span
      style={colorVar(statusColor(status))}
      className="mono-chip-y w-max border border-[color-mix(in_srgb,var(--c)_38%,transparent)] bg-[color-mix(in_srgb,var(--c)_8%,transparent)] px-[9px] font-mono text-[12px] tracking-[0.06em] uppercase text-(--c)"
    >
      {status}
    </span>
  );
}
