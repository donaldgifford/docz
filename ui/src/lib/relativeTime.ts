/*
 * Terminal-terse relative timestamps for listing columns, per the
 * mockup's updated cells ("just now", "3h ago", "2w ago"). Pure function
 * of (iso, now) so tests stay deterministic; "" — the API's unset
 * convention — renders the em dash.
 */

const MINUTE = 60;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;
const WEEK = 7 * DAY;
const MONTH = 30 * DAY;
const YEAR = 365 * DAY;

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
