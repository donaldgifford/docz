import type { SearchHit } from "@/api/__generated__/docz-api.schemas";

/*
 * Everything about the "updated" column of a listing.
 *
 * `SearchHit` has no timestamp property. docz-api DOES index one —
 * `internal/search/types.go` stores `updated_at` (Unix seconds) on the
 * Meilisearch record and `internal/search/client.go` declares it a
 * sortable attribute — but `decodeHits` never copies it onto the wire
 * struct, so it is absent from the OpenAPI schema and therefore from
 * the generated type. Exposing it is the open additive ask
 * (DESIGN-0001; see also DESIGN-0005 amendment eight).
 *
 * `hitUpdatedAt` reads the property defensively, the same way
 * `apiConfig`/`changelogConfig` read `config_snapshot`: the column
 * lights up the day the field ships, with no further change here.
 * Until then doc hits render the em dash — as do page hits, which have
 * no timestamp anywhere in the contract.
 */

/** RFC3339 stamp for a hit, or "" when the API sent none. */
export function hitUpdatedAt(hit: SearchHit): string {
  const value: unknown = (hit as { updated_at?: unknown }).updated_at;
  return typeof value === "string" ? value : "";
}

/**
 * Absolute two-line stamp for a listing row: `Sep 10, 2026` over
 * `1:52 PM`, per the RFD-index references DESIGN-0005 amendment eight
 * cites. Returns undefined for the unset and unparseable cases so the
 * caller picks its own placeholder.
 *
 * `timeZone` exists for tests; the app passes nothing and renders in
 * the reader's own zone. The locale is pinned to en-US so month
 * abbreviations and the 12-hour clock do not drift per machine.
 */
export function formatUpdatedStamp(
  iso: string,
  timeZone?: string,
): { date: string; time: string } | undefined {
  if (iso === "") {
    return undefined;
  }
  const parsed = new Date(iso);
  if (Number.isNaN(parsed.getTime())) {
    return undefined;
  }
  return {
    date: clean(
      parsed.toLocaleDateString("en-US", {
        month: "short",
        day: "numeric",
        year: "numeric",
        timeZone,
      }),
    ),
    time: clean(
      parsed.toLocaleTimeString("en-US", {
        hour: "numeric",
        minute: "2-digit",
        timeZone,
      }),
    ),
  };
}

// ICU emits a narrow no-break space before AM/PM in recent versions and
// a plain space in older ones. Normalize so the rendered column — and
// the assertions over it — do not depend on the runtime's ICU build.
function clean(value: string): string {
  return value.replace(/\s/gu, " ");
}

const MINUTE = 60;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;
const WEEK = 7 * DAY;
const MONTH = 30 * DAY;
const YEAR = 365 * DAY;

/*
 * Terminal-terse relative timestamps ("just now", "3h ago", "2w ago"),
 * per the mockup's updated cells. Pure function of (iso, now) so tests
 * stay deterministic; "" — the API's unset convention — renders the em
 * dash. Kept for the surfaces that want relative over absolute.
 */
export function formatRelativeTime(iso: string, now: Date): string {
  if (iso === "") {
    return "—";
  }
  const then = Date.parse(iso);
  if (Number.isNaN(then)) {
    return "—";
  }
  const seconds = (now.getTime() - then) / 1000;
  // Anything under a minute — including future timestamps from clock
  // skew — reads as "just now".
  if (seconds < MINUTE) {
    return "just now";
  }
  if (seconds < HOUR) {
    return `${String(Math.floor(seconds / MINUTE))}m ago`;
  }
  if (seconds < DAY) {
    return `${String(Math.floor(seconds / HOUR))}h ago`;
  }
  if (seconds < WEEK) {
    return `${String(Math.floor(seconds / DAY))}d ago`;
  }
  if (seconds < MONTH) {
    return `${String(Math.floor(seconds / WEEK))}w ago`;
  }
  if (seconds < YEAR) {
    return `${String(Math.floor(seconds / MONTH))}mo ago`;
  }
  return `${String(Math.floor(seconds / YEAR))}y ago`;
}
