import { visit } from "unist-util-visit";

import type { Root } from "hast";

/*
 * Relative doc-link resolution (DESIGN-0002 Component 2, from
 * INV-0004): author-written relative hrefs like `../adr/0013-….md`
 * resolve against the rendering source's own repo path and, when the
 * result is exactly a path the repo serves, become in-app links. The
 * byPath map is the whitelist — document text supplies only the lookup
 * key, and the emitted href always comes from API data. Anything that
 * misses (wrong depth, traversal past the repo root, a file docz
 * doesn't ingest) stays byte-identical to today, exactly as wrong or
 * right as it is on GitHub. Runs AFTER sanitize on trusted structure,
 * beside linkifyDocIds.
 */

export interface RelativeLinkContext {
  /** Repo-relative path of the rendering source itself ("docs/rfc/RFC-0001.md"). */
  base: string;
  /** Repo-relative doc path -> SPA href (`RepoDocIndex.byPath`). */
  byPath: ReadonlyMap<string, string>;
}

/** Scheme prefix per RFC 3986 — `https:`, `mailto:`, `javascript:`, … */
const SCHEME = /^[a-zA-Z][a-zA-Z0-9+.-]*:/;

/**
 * Posix-resolves `relative` against the directory of `base`, collapsing
 * `.`/`..`. Undefined when the path escapes the repo root — such a path
 * can never match a map key, so traversal fails closed.
 */
function resolveRepoPath(base: string, relative: string): string | undefined {
  // Empty/"." base segments (a root-level source, a "./"-prefixed
  // docs_dir) must not leak into the joined key.
  const segments = base
    .split("/")
    .slice(0, -1)
    .filter((segment) => segment !== "" && segment !== ".");
  for (const segment of relative.split("/")) {
    if (segment === "" || segment === ".") {
      continue;
    }
    if (segment === "..") {
      if (segments.length === 0) {
        return undefined;
      }
      segments.pop();
      continue;
    }
    segments.push(segment);
  }
  return segments.join("/");
}

export function resolveRelativeLinks(
  tree: Root,
  context: RelativeLinkContext,
): void {
  if (context.byPath.size === 0) {
    return;
  }
  visit(tree, "element", (node) => {
    if (node.tagName !== "a") {
      return;
    }
    const href = node.properties.href;
    if (typeof href !== "string" || href === "") {
      return;
    }
    // Only relative paths are candidates: absolute URLs, protocol- and
    // root-relative hrefs, and in-page fragments all pass through.
    if (href.startsWith("#") || href.startsWith("/") || SCHEME.test(href)) {
      return;
    }

    const hashIndex = href.indexOf("#");
    const pathPart = hashIndex === -1 ? href : href.slice(0, hashIndex);
    const fragment = hashIndex === -1 ? "" : href.slice(hashIndex);
    if (pathPart === "") {
      return;
    }
    // Authors percent-encode spaces etc. while the API serves literal
    // paths; decode before resolving (encoded `..` then normalizes and
    // fails closed like any traversal). Malformed encoding: miss.
    let decoded: string;
    try {
      decoded = decodeURIComponent(pathPart);
    } catch {
      return;
    }

    const resolved = resolveRepoPath(context.base, decoded);
    if (resolved === undefined) {
      return;
    }
    // Exact normalized-path match only (DESIGN-0002 OQ-3a) — no fuzzy
    // basename rescue; a link wrong on GitHub stays wrong here.
    const target = context.byPath.get(resolved);
    if (target === undefined) {
      return;
    }
    node.properties.href = target + fragment;
    // MarkdownAnchor renders data-xref anchors as router Links.
    node.properties.dataXref = true;
  });
}
