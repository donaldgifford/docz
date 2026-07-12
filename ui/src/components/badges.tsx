import type { CSSProperties } from "react";

import { statusColor, typeColor } from "@/lib/colors";

/*
 * Badge trio per the mockup styles. The mockup hardcodes per-type
 * data-attribute rules; here the color system supplies `--c` inline, so
 * unknown custom types and statuses work with zero configuration
 * (DESIGN-0001: "the color system, not the type system, interprets").
 */

function colorVar(color: string): CSSProperties {
  return { "--c": color } as CSSProperties;
}

/** Uppercase mono chip carrying the doc type color. */
export function TypeBadge({ type }: { type: string }) {
  return (
    <span
      style={colorVar(typeColor(type))}
      className="w-max border border-[color-mix(in_srgb,var(--c)_35%,transparent)] bg-[color-mix(in_srgb,var(--c)_9%,transparent)] px-[7px] py-[2px] font-mono text-[10.5px] tracking-[0.05em] uppercase text-(--c)"
    >
      {type}
    </span>
  );
}

/** Dot + label, used in listings and metadata rows. */
export function StatusBadge({ status }: { status: string }) {
  return (
    <span
      style={colorVar(statusColor(status))}
      className="inline-flex items-center gap-[5px] font-mono text-[11px] tracking-[0.03em] text-(--c)"
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

/** Bordered uppercase pill, used next to the doc title in the reader. */
export function StatusPill({ status }: { status: string }) {
  return (
    <span
      style={colorVar(statusColor(status))}
      className="border border-[color-mix(in_srgb,var(--c)_38%,transparent)] bg-[color-mix(in_srgb,var(--c)_8%,transparent)] px-[9px] py-[2px] font-mono text-[11px] tracking-[0.06em] uppercase text-(--c)"
    >
      {status}
    </span>
  );
}
