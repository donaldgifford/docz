/*
 * Recently-opened docs for the palette's empty-query state (IMPL-0002
 * Phase 6, OQ-7a). localStorage-backed, cap 8, most-recent first.
 * UI-preference data ONLY — doc coordinates and titles, never tokens
 * (the no-tokens-in-JS-readable-storage rule). Reads are validated
 * segment-by-segment (entries build router paths) and a malformed
 * payload resets the store rather than rendering broken entries.
 */

const KEY = "docz:recent-docs";
const CAP = 8;

export interface RecentDoc {
  /** "owner/name" */
  repo: string;
  type: string;
  docId: string;
  title: string;
}

const SEGMENT = /^[\w.-]+$/;

function isRecentDoc(value: unknown): value is RecentDoc {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const entry = value as Record<string, unknown>;
  if (
    typeof entry.repo !== "string" ||
    typeof entry.type !== "string" ||
    typeof entry.docId !== "string" ||
    typeof entry.title !== "string"
  ) {
    return false;
  }
  const repoParts = entry.repo.split("/");
  return (
    repoParts.length === 2 &&
    repoParts.every((part) => SEGMENT.test(part)) &&
    SEGMENT.test(entry.type) &&
    SEGMENT.test(entry.docId) &&
    entry.title.length > 0
  );
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
    (doc) =>
      doc.repo !== entry.repo ||
      doc.type !== entry.type ||
      doc.docId !== entry.docId,
  );
  try {
    localStorage.setItem(KEY, JSON.stringify([entry, ...rest].slice(0, CAP)));
  } catch {
    // Best-effort: quota or private mode never breaks the reader.
  }
}
