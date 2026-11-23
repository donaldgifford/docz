/*
 * Type and status colours (DESIGN-0001 "Type and status color system",
 * DESIGN-0028 §8): curated map first, deterministic fallback second,
 * neutral default last. `.docz.yaml` types and statuses are free-form,
 * so there is deliberately no hardcoded "valid" list.
 *
 * A type's colour is a CSS custom-property reference: docz's own
 * `--color-t-*` (src/theme/docz-tokens.css in the chrome, the document's
 * theme inside a .docz-page), or one of eight `--color-hash-*` for a
 * custom type, which are the design system's `--category-1` to
 * `--category-8`. A status has no colour here: it takes one of the
 * design system's Badge variants.
 */

const CURATED_TYPES: Readonly<Record<string, string | undefined>> = {
  rfc: "rfc",
  adr: "adr",
  design: "design",
  impl: "impl",
  investigation: "investigation",
  inv: "investigation",
  runbook: "runbook",
  rb: "runbook",
  mandate: "mandate",
  guide: "guide",
  principle: "principle",
  policy: "policy",
  framework: "framework",
  docs: "docs",
};

const HASH_PALETTE_SIZE = 8;

// FNV-1a — stable across sessions and platforms; the fallback color for
// a custom type must never change between visits. Also exported as a
// general-purpose cheap fingerprint (e.g. xref cache keys).
export function fnv1a(value: string): number {
  let hash = 0x811c9dc5;
  for (let i = 0; i < value.length; i++) {
    hash ^= value.charCodeAt(i);
    hash = Math.imul(hash, 0x01000193);
  }
  return hash >>> 0;
}

function normalize(value: string): string {
  return value.trim().toLowerCase().replace(/\s+/g, " ");
}

export function typeColor(type: string): string {
  const key = normalize(type);
  const curated = CURATED_TYPES[key];
  if (curated !== undefined) {
    return `var(--color-t-${curated})`;
  }
  return `var(--color-hash-${String(fnv1a(key) % HASH_PALETTE_SIZE)})`;
}

type StatusGroup =
  "draft" | "proposed" | "accepted" | "rejected" | "superseded" | "deprecated";

const STATUS_GROUPS: readonly [token: StatusGroup, statuses: string[]][] = [
  ["draft", ["draft", "open"]],
  ["proposed", ["proposed", "in review", "in progress"]],
  [
    "accepted",
    [
      "accepted",
      "active",
      "approved",
      "adopted",
      "completed",
      "implemented",
      "concluded",
    ],
  ],
  ["rejected", ["rejected", "cancelled", "canceled", "abandoned"]],
  ["superseded", ["superseded"]],
  // Ended without a verdict, or parked: the muted token, so these never
  // read as either success or failure.
  ["deprecated", ["deprecated", "archived", "paused", "inconclusive"]],
];

const STATUS_TOKENS: Readonly<Record<string, StatusGroup | undefined>> =
  Object.fromEntries(
    STATUS_GROUPS.flatMap(([token, statuses]) =>
      statuses.map((status) => [status, token]),
    ),
  );

/**
 * The design system's `Badge` variants a status can take (DESIGN-0028
 * §8). Each is one of the package's status roles or states, so a status
 * carries no colour of docz's own in the chrome.
 */
export type StatusVariant =
  "warning" | "info" | "success" | "destructive" | "ignored" | "outline";

const STATUS_VARIANTS: Readonly<Record<StatusGroup, StatusVariant>> = {
  draft: "warning",
  proposed: "info",
  accepted: "success",
  rejected: "destructive",
  superseded: "ignored",
  // Ended without a verdict, or parked: no fill, so these never read as
  // either success or failure.
  deprecated: "outline",
};

/** Neutral for anything outside the convention, never an error. */
export const NEUTRAL_STATUS_VARIANT: StatusVariant = "outline";

/**
 * The `Badge` variant for a status. `.docz.yaml` statuses are free-form,
 * so an unknown one is neutral, and matching ignores case and extra
 * whitespace.
 */
export function statusVariant(status: string): StatusVariant {
  const group = STATUS_TOKENS[normalize(status)];
  return group !== undefined ? STATUS_VARIANTS[group] : NEUTRAL_STATUS_VARIANT;
}
