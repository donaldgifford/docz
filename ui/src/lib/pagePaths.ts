/*
 * Published-path → source-path reconstruction (DESIGN-0004 Component
 * 3). The pages wire deliberately omits the source file's repo path,
 * but the site can invert the upstream mapping with data it already
 * holds: `docs_dir` (getRepo) and `additional_docs` (apiConfig). The
 * source paths are what author-written relative links resolve against
 * — both as `byPath` keys (link targets) and as a page body's own
 * resolution base.
 */

/**
 * Every repo path the published path may have come from. One entry for
 * file pages; two for directory pages (README.md wins a directory
 * upstream, but a lone index.md serves it too — resolution only needs
 * the right directory, so both keys map to the same href).
 */
export function pageSourceKeys(
  publishedPath: string,
  docsDir: string,
  additionalDocs: readonly string[],
): string[] {
  if (additionalDocs.includes(publishedPath)) {
    // additional_docs publish at their repo-relative path unchanged.
    return [publishedPath];
  }
  if (/\.md$/i.test(publishedPath)) {
    return [`${docsDir}/${publishedPath}`];
  }
  // Extensionless = directory page.
  return [
    `${docsDir}/${publishedPath}/README.md`,
    `${docsDir}/${publishedPath}/index.md`,
  ];
}

/** The canonical source path — a page body's relative-link base. */
export function pageSourcePath(
  publishedPath: string,
  docsDir: string,
  additionalDocs: readonly string[],
): string {
  return (
    pageSourceKeys(publishedPath, docsDir, additionalDocs)[0] ?? publishedPath
  );
}
