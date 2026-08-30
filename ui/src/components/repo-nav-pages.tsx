import { useState } from "react";
import { NavLink, useParams } from "react-router";

import { useListRepoPages } from "@/api/__generated__/docz-api";
import { usePrefetchPage } from "@/hooks/usePrefetchPage";
import { arr } from "@/lib/wire";

import type { PageSummary } from "@/api/__generated__/docz-api.schemas";

/*
 * The RepoNav Pages tree (DESIGN-0004 OQ-2a): published pages from the
 * repo's api: block, between the changelog row and the type drawers.
 * The section exists only when the caller's apiConfig gate hit (the
 * list query never fires for non-opted repos — RepoNav mounts this
 * component conditionally) AND the list is non-empty. The tree comes
 * from the flat path-ordered list: file pages are leaves, directory
 * pages attach to their directory node (which then links), and
 * directories collapse with the type-drawer caret behavior. The active
 * route's branch auto-expands; manual toggles reset on navigation.
 */

const FIVE_MINUTES = 5 * 60_000;

interface PageTreeNode {
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

function pageLinkClass({ isActive }: { isActive: boolean }): string {
  return `block min-w-0 flex-1 overflow-hidden py-[0.17rem] pr-[0.45rem] text-[11.5px] text-ellipsis whitespace-nowrap ${
    isActive ? "text-accent" : "text-fg-muted hover:text-fg-primary"
  }`;
}

function NavPageNode({
  owner,
  name,
  node,
  depth,
  expandedDirs,
  onToggle,
}: {
  owner: string;
  name: string;
  node: PageTreeNode;
  depth: number;
  expandedDirs: (path: string) => boolean;
  onToggle: (path: string) => void;
}) {
  const prefetchPage = usePrefetchPage();
  const indent = { paddingLeft: `${String(0.45 + depth * 0.7)}rem` };
  const expanded = expandedDirs(node.path);

  const link =
    node.page === undefined ? (
      // A directory nothing publishes at: a toggle, never a link.
      <button
        type="button"
        onClick={() => {
          onToggle(node.path);
        }}
        style={indent}
        className="block min-w-0 flex-1 cursor-pointer overflow-hidden py-[0.17rem] pr-[0.45rem] text-left text-[11.5px] text-ellipsis whitespace-nowrap text-fg-muted hover:text-fg-primary"
      >
        {node.segment}/
      </button>
    ) : (
      <NavLink
        to={`/${owner}/${name}/pages/${node.path}`}
        end
        title={node.page.title}
        onMouseEnter={() => {
          prefetchPage(owner, name, node.path);
        }}
        onFocus={() => {
          prefetchPage(owner, name, node.path);
        }}
        style={indent}
        className={pageLinkClass}
      >
        {node.children.length === 0 ? node.page.title : `${node.segment}/`}
      </NavLink>
    );

  return (
    <>
      <div className="flex items-stretch">
        {link}
        {node.children.length > 0 && (
          <button
            type="button"
            aria-expanded={expanded}
            aria-label={`${node.path} pages`}
            onClick={() => {
              onToggle(node.path);
            }}
            className="w-6 flex-none cursor-pointer text-center text-[10px] text-fg-muted hover:bg-bg-raised hover:text-fg-primary"
          >
            <span aria-hidden>{expanded ? "▾" : "▸"}</span>
          </button>
        )}
      </div>
      {expanded &&
        node.children.map((child) => (
          <NavPageNode
            key={child.path}
            owner={owner}
            name={name}
            node={child}
            depth={depth + 1}
            expandedDirs={expandedDirs}
            onToggle={onToggle}
          />
        ))}
    </>
  );
}

export function NavPagesSection({
  owner,
  name,
}: {
  owner: string;
  name: string;
}) {
  // Same options as useRepoDocIndex's gated query — the cache is
  // shared, so the nav costs no extra request on opted repos.
  const pagesQuery = useListRepoPages(owner, name, {
    query: { staleTime: FIVE_MINUTES },
  });
  const pages =
    pagesQuery.data?.status === 200
      ? arr(pagesQuery.data.data.pages)
      : undefined;

  // The pages route's splat auto-expands its ancestor directories;
  // manual toggles reset when navigation moves (adjust-during-render).
  const params = useParams();
  const activePath = params["*"];
  const [overrides, setOverrides] = useState<Record<string, boolean>>({});
  const [prevActive, setPrevActive] = useState(activePath);
  if (prevActive !== activePath) {
    setPrevActive(activePath);
    setOverrides({});
  }
  const autoExpanded = (path: string): boolean =>
    activePath !== undefined &&
    (activePath === path || activePath.startsWith(`${path}/`));
  const expandedDirs = (path: string): boolean =>
    overrides[path] ?? autoExpanded(path);

  if (pages === undefined || pages.length === 0) {
    return null;
  }

  return (
    <>
      <div className="mt-[1.15rem] mb-[0.45rem] border-b border-border-hairline pb-[0.4rem] text-[10px] tracking-[0.14em] text-fg-muted uppercase">
        pages
      </div>
      {buildPageTree(pages).map((node) => (
        <NavPageNode
          key={node.path}
          owner={owner}
          name={name}
          node={node}
          depth={0}
          expandedDirs={expandedDirs}
          onToggle={(path) => {
            setOverrides((prev) => ({
              ...prev,
              [path]: !expandedDirs(path),
            }));
          }}
        />
      ))}
    </>
  );
}
