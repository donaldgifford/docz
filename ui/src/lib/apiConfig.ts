/*
 * Defensive reader for the `.docz.yaml` `api:` block inside a repo's
 * config_snapshot (docz v1.2.0, served with the .docz.yaml key
 * spellings since docz v1.2.2 / docz-api spec 1.4.1). The snapshot is
 * untyped JSON (`additionalProperties: true`) authored by repo owners,
 * so any wrong shape — including a stale pre-v1.2.2 snapshot with
 * capitalized Go field names — must read as "no pages surface": the
 * Pages nav, the pages route, and the link index all gate on this
 * returning a config, firing zero pages requests otherwise
 * (DESIGN-0004 Component 2).
 */

export interface ApiBlockConfig {
  /** Resolved landing-page path (docz backfills at load); "" only on
   *  wrong shapes — consumers fall back with `||`, not `??`. */
  landingPage: string;
  /** Normalized repo-relative additional_docs paths; null list → []. */
  additionalDocs: string[];
}

export function apiConfig(snapshot: unknown): ApiBlockConfig | undefined {
  if (typeof snapshot !== "object" || snapshot === null) {
    return undefined;
  }
  const block = (snapshot as Record<string, unknown>).api;
  if (typeof block !== "object" || block === null) {
    return undefined;
  }
  const record = block as Record<string, unknown>;
  if (record.enabled !== true) {
    return undefined;
  }
  const landingPage =
    typeof record.landing_page === "string" ? record.landing_page : "";
  // The wire's nil-slice-as-null gotcha (arr() semantics): null, absent,
  // and non-array all read as empty; non-string entries are dropped.
  const additionalDocs = Array.isArray(record.additional_docs)
    ? record.additional_docs.filter(
        (entry): entry is string => typeof entry === "string",
      )
    : [];
  return { landingPage, additionalDocs };
}
