import {
  Dot,
  type DotTone,
} from "@donaldgifford/design-system/blocks/list-row";
import { Badge } from "@donaldgifford/design-system/components/badge";
import type { ReactNode } from "react";

import { type StatusVariant, statusVariant, typeColor } from "@/lib/colors";

/*
 * Status and type marks, drawn by the design system's Badge and Dot
 * (DESIGN-0028 §8). docz supplies what a status or a type means and the
 * package supplies the colour:
 *
 *   - a status takes one of the package's variants through
 *     statusVariant(), so an unknown custom status works with zero
 *     configuration and reads as neutral;
 *   - a type takes a `tone`, the CSS colour typeColor() returns, which
 *     is docz's own (docz-tokens.css) or one of the package's eight
 *     category colours for a custom type.
 *
 * Listings carry no type badge. The doc id (DESIGN-0001, IMPL-0005)
 * already spells the type out, and a second coloured chip beside the
 * status made every row read as two competing signals; the DESIGN-0005
 * dial-in dropped it. The type marks here are for the places a type
 * stands alone: a filter chip, a sidebar row.
 */

/** The Dot beside a status label, in the tone of the status's variant. */
const STATUS_DOT: Readonly<Record<StatusVariant, DotTone>> = {
  warning: "warn",
  info: "info",
  success: "ok",
  destructive: "bad",
  ignored: "neutral",
  outline: "neutral",
};

/** Dot and label, for a row that already has a badge's worth of weight. */
export function StatusBadge({ status }: { status: string }) {
  const variant = statusVariant(status);
  return (
    <span
      data-slot="status-label"
      data-variant={variant}
      className="inline-flex items-center gap-1.5 font-mono text-xs/none text-muted-foreground"
    >
      <Dot tone={STATUS_DOT[variant]} />
      {status}
    </span>
  );
}

/**
 * The document's state as a tinted badge, and the one coloured thing in
 * a listing row. Used next to the reader's title and in every hit.
 */
export function StatusPill({ status }: { status: string }) {
  return <Badge variant={statusVariant(status)}>{status}</Badge>;
}

/** A type's colour as a dot: the sidebar's type rows, a legend. */
export function TypeDot({ type }: { type: string }) {
  return <Dot tone={typeColor(type)} />;
}

/** A type as a tinted badge in its own colour, labelled with its name. */
export function TypeBadge({
  type,
  children,
}: {
  type: string;
  /** Replaces the type's name as the label: its id prefix, say. */
  children?: ReactNode;
}) {
  return <Badge tone={typeColor(type)}>{children ?? type}</Badge>;
}
