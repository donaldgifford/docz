/*
 * Everything about the "updated" column of a listing.
 *
 * Since spec 1.5.0 (docz-api v0.9.1/v0.10.0) `SearchHit.updated_at` is
 * a typed, required property, so callers read it directly — the
 * defensive accessor this module used to export existed only to hide an
 * untyped probe of a field the schema did not have, and keeping it would
 * have preserved the shape of a problem that no longer exists.
 *
 * Both record kinds carry a stamp. `updated_at` is populated on doc AND
 * page hits; `created` is the field that is `""` on pages, because a
 * published page has no authored frontmatter date. Note that
 * `updated_at` is ingest-observed — when docz-api last saw the content
 * change — not git commit time, so a fresh database restamps a whole
 * repo at onboard.
 */

/**
 * Absolute two-line stamp for a listing row: `Sep 10, 2026` over
 * `1:52 PM`, per the RFD-index references DESIGN-0005 amendment eight
 * cites. Returns undefined for the unset and unparseable cases so the
 * caller picks its own placeholder.
 *
 * `timeZone` exists for tests; the app passes nothing and renders in
 * the reader's own zone. The locale is pinned to en-US so month
 * abbreviations and the 12-hour clock do not drift per machine.
 *
 * Keep the `Number.isNaN` guard. Beyond rejecting a malformed stamp, it
 * is what makes a deployment pointed at a pre-1.5.0 docz-api degrade to
 * the em dash instead of rendering "Invalid Date": the property is
 * typed required, so an older API simply omits it and `undefined`
 * arrives where a string was promised.
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
