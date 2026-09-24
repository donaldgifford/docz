/*
 * Recently-opened docs and pages for the palette's empty-query state
 * (IMPL-0002 Phase 6, OQ-7a; pages per DESIGN-0004 OQ-5a).
 * localStorage-backed, cap 8, most-recent first. UI-preference data
 * ONLY — coordinates and titles, never tokens (the
 * no-tokens-in-JS-readable-storage rule). Reads are validated
 * segment-by-segment (entries build router paths) and a malformed
 * payload — including the pre-kind stored shape — resets the store
 * rather than rendering broken entries.
 */

const KEY = "docz:recent-docs";
const CAP = 8;

export type RecentDoc =
  | {
      kind: "doc";
      /** "owner/name" */
      repo: string;
      type: string;
      docId: string;
      title: string;
    }
  | {
      kind: "page";
      /** "owner/name" */
      repo: string;
      /** Published path — the pages route's splat, slash-separated. */
      path: string;
      title: string;
    };

const SEGMENT = /^[\w.-]+$/;

function isValidRepo(repo: unknown): repo is string {
  if (typeof repo !== "string") {
    return false;
  }
  const parts = repo.split("/");
  return parts.length === 2 && parts.every((part) => SEGMENT.test(part));
}

/** Dot-only segments would router-resolve upward out of the route. */
function isPathSegment(segment: string): boolean {
  return SEGMENT.test(segment) && segment !== "." && segment !== "..";
}

function isRecentDoc(value: unknown): value is RecentDoc {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const entry = value as Record<string, unknown>;
  if (
    !isValidRepo(entry.repo) ||
    typeof entry.title !== "string" ||
    entry.title.length === 0
  ) {
    return false;
  }
  if (entry.kind === "doc") {
    return (
      typeof entry.type === "string" &&
      SEGMENT.test(entry.type) &&
      typeof entry.docId === "string" &&
      SEGMENT.test(entry.docId)
    );
  }
  if (entry.kind === "page") {
    return (
      typeof entry.path === "string" &&
      entry.path.split("/").every(isPathSegment)
    );
  }
  // Unknown kind — including the pre-kind shape — is malformed.
  return false;
}

function entryKey(entry: RecentDoc): string {
  return entry.kind === "doc"
    ? `doc:${entry.repo}:${entry.type}:${entry.docId}`
    : `page:${entry.repo}:${entry.path}`;
}

export function readRecentDocs(): RecentDoc[] {
  let raw: string | null;
  try {
    raw = localStorage.getItem(KEY);
  } catch {
    return []; // storage unavailable (private mode etc.)
  }
  if (raw === null) {
    return [];
  }
  try {
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed) || !parsed.every(isRecentDoc)) {
      throw new Error("malformed recent-docs payload");
    }
    return parsed.slice(0, CAP);
  } catch {
    localStorage.removeItem(KEY);
    return [];
  }
}

export function recordRecentDoc(entry: RecentDoc): void {
  if (!isRecentDoc(entry)) {
    return;
  }
  const rest = readRecentDocs().filter(
    (existing) => entryKey(existing) !== entryKey(entry),
  );
  try {
    localStorage.setItem(KEY, JSON.stringify([entry, ...rest].slice(0, CAP)));
  } catch {
    // Best-effort: quota or private mode never breaks the reader.
  }
}
