import type { PageSummary } from "@/api/__generated__/docz-api.schemas";

/*
 * A repository's published pages as a tree (DESIGN-0004 OQ-2a), from
 * the flat path-ordered list docz-api sends. A file page is a leaf; a
 * directory page attaches to its directory's node, which then links;
 * a directory nothing publishes at is a node with no page.
 */

export interface PageTreeNode {
  /** Path segment label (directory name or file basename). */
  segment: string;
  /** Full published path up to this node. */
  path: string;
  /** Set when a page publishes exactly at this path. */
  page?: PageSummary;
  children: PageTreeNode[];
}

/** Upstream orders by path, so siblings come out already sorted. */
export function buildPageTree(pages: readonly PageSummary[]): PageTreeNode[] {
  const root: PageTreeNode = { segment: "", path: "", children: [] };
  const dirs = new Map<string, PageTreeNode>([["", root]]);
  const dirNode = (path: string): PageTreeNode => {
    const existing = dirs.get(path);
    if (existing !== undefined) {
      return existing;
    }
    const cut = path.lastIndexOf("/");
    const node: PageTreeNode = {
      segment: cut === -1 ? path : path.slice(cut + 1),
      path,
      children: [],
    };
    dirNode(cut === -1 ? "" : path.slice(0, cut)).children.push(node);
    dirs.set(path, node);
    return node;
  };
  for (const page of pages) {
    if (/\.md$/i.test(page.path)) {
      const cut = page.path.lastIndexOf("/");
      dirNode(cut === -1 ? "" : page.path.slice(0, cut)).children.push({
        segment: cut === -1 ? page.path : page.path.slice(cut + 1),
        path: page.path,
        page,
        children: [],
      });
    } else {
      // Extensionless = directory page; the node becomes a link.
      dirNode(page.path).page = page;
    }
  }
  return root.children;
}

/**
 * The page at `path` and every directory above it, nearest last: the
 * branches of the tree that hold the current page.
 */
export function pageAncestors(path: string | undefined): string[] {
  if (path === undefined || path === "") {
    return [];
  }
  const segments = path.split("/");
  return segments.map((_, index) => segments.slice(0, index + 1).join("/"));
}
