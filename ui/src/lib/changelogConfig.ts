/*
 * Defensive reader for the `.docz.yaml` `changelog:` block inside a
 * repo's config_snapshot (docz-api spec 1.2.0). The snapshot is
 * untyped JSON (`additionalProperties: true`) authored by repo owners,
 * so any shape mistake must read as "no changelog" — the RepoNav row
 * and the changelog route both gate on this returning a config
 * (DESIGN-0002 Component 3, INV-0005 OQ-1a).
 */

const DEFAULT_FILE = "CHANGELOG.md";

export interface ChangelogConfig {
  /** Repo-relative path of the configured changelog file. */
  file: string;
}

export function changelogConfig(
  snapshot: unknown,
): ChangelogConfig | undefined {
  if (typeof snapshot !== "object" || snapshot === null) {
    return undefined;
  }
  const block = (snapshot as Record<string, unknown>).changelog;
  if (typeof block !== "object" || block === null) {
    return undefined;
  }
  const record = block as Record<string, unknown>;
  if (record.enabled !== true) {
    return undefined;
  }
  const file =
    typeof record.file === "string" && record.file.trim() !== ""
      ? record.file
      : DEFAULT_FILE;
  return { file };
}

/** Last path segment — the nav row's hint text (INV-0005 OQ-5a). */
export function changelogBasename(file: string): string {
  return file.split("/").at(-1) ?? file;
}
