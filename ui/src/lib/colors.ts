/*
 * Type and status colors (DESIGN-0001 "Type and status color system"):
 * curated map first, deterministic fallback second, neutral default
 * last. Values are CSS custom-property references so the palette stays
 * single-sourced in src/theme/tokens.css. `.docz.yaml` types/statuses
 * are free-form — there is deliberately no hardcoded "valid" list.
 */

const CURATED_TYPES: Readonly<Record<string, string | undefined>> = {
  rfc: "rfc",
  adr: "adr",
  design: "design",
  impl: "impl",
  investigation: "investigation",
  inv: "investigation",
  mandate: "mandate",
  guide: "guide",
  principle: "principle",
  policy: "policy",
  framework: "framework",
};

const HASH_PALETTE_SIZE = 8;

// FNV-1a — stable across sessions and platforms; the fallback color for
// a custom type must never change between visits.
function fnv1a(value: string): number {
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

const STATUS_GROUPS: readonly [token: string, statuses: string[]][] = [
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
  ["deprecated", ["deprecated", "archived", "paused"]],
];

const STATUS_TOKENS: Readonly<Record<string, string | undefined>> =
  Object.fromEntries(
    STATUS_GROUPS.flatMap(([token, statuses]) =>
      statuses.map((status) => [status, token]),
    ),
  );

/** Neutral for anything outside the convention — never an error. */
export const NEUTRAL_STATUS_COLOR = "var(--color-fg-muted)";

export function statusColor(status: string): string {
  const token = STATUS_TOKENS[normalize(status)];
  return token !== undefined
    ? `var(--color-st-${token})`
    : NEUTRAL_STATUS_COLOR;
}
