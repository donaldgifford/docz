import { InlineLoading } from "@donaldgifford/design-system/blocks/inline-state";
import { useTreeOpen } from "@donaldgifford/design-system/hooks/use-tree-open";
import { ShellSidebarGroup } from "@donaldgifford/design-system/shell/sidebar";
import {
  ShellSidebarTreeItem,
  ShellSidebarTreeNote,
} from "@donaldgifford/design-system/shell/sidebar-tree";
import { IconHistory, IconHome } from "@tabler/icons-react";
import { useMemo } from "react";
import { Link } from "react-router";

import {
  useGetRepo,
  useListDocs,
  useListRepoPages,
} from "@/api/__generated__/docz-api";
import { TypeDot } from "@/components/badges";
import { usePrefetchChangelog } from "@/hooks/usePrefetchChangelog";
import { usePrefetchDoc } from "@/hooks/usePrefetchDoc";
import { usePrefetchPage } from "@/hooks/usePrefetchPage";
import { useRepoFacts } from "@/hooks/useRepoFacts";
import { apiConfig } from "@/lib/apiConfig";
import { changelogBasename, changelogConfig } from "@/lib/changelogConfig";
import { resolveDocType } from "@/lib/docTypes";
import {
  buildPageTree,
  pageAncestors,
  type PageTreeNode,
} from "@/lib/pageTree";
import { arr } from "@/lib/wire";

/*
 * A repository's groups in the shell's sidebar (DESIGN-0028 §6), on
 * that repository's routes only: its own group (Home, Changelog), its
 * published pages as a tree, and its documents as a tree with a row per
 * type. Every row is the design system's ShellSidebarTreeItem, so the
 * icon rail, its popover, and the sheet on a narrow screen come with
 * it.
 *
 * What docz decides is what a row means. A type's row links to the
 * type's index and carries its colour as a dot and its count; its
 * documents are fetched when the row first opens, which the package
 * also reports for the icon rail's popover. The branch holding the
 * current page is open, a toggle by hand overrides that, and moving to
 * another branch drops the overrides (the package's useTreeOpen).
 * Counts come from useRepoFacts, the source the repositories grid and
 * the directory's facets use, so the numbers agree everywhere.
 */

const FIVE_MINUTES = 5 * 60_000;

export interface RepoRoute {
  owner: string;
  name: string;
  /** The route's `:type` segment, possibly an id prefix or an alias. */
  type?: string | undefined;
  docId?: string | undefined;
  /** The pages route's splat: a published path. */
  pagePath?: string | undefined;
  /** The whole pathname, to tell Home and Changelog apart. */
  pathname: string;
}

/** A type's documents, fetched when the branch that holds them mounts. */
function TypeDocuments({
  owner,
  name,
  type,
  activeDocId,
}: {
  owner: string;
  name: string;
  type: string;
  activeDocId: string | undefined;
}) {
  // The package keeps a closed branch out of the DOM, so this mounts
  // when the row opens (or the icon rail's popover does) and a closed
  // sidebar costs no listDocs at all.
  const docsQuery = useListDocs(owner, name, type, {
    query: { staleTime: FIVE_MINUTES },
  });
  const prefetchDoc = usePrefetchDoc();

  if (docsQuery.isPending) {
    return (
      <ShellSidebarTreeNote>
        <InlineLoading className="p-0" />
      </ShellSidebarTreeNote>
    );
  }
  if (docsQuery.data?.status !== 200) {
    return (
      <ShellSidebarTreeNote>Could not load documents</ShellSidebarTreeNote>
    );
  }
  const docs = arr(docsQuery.data.data.docs);
  if (docs.length === 0) {
    return <ShellSidebarTreeNote>No documents</ShellSidebarTreeNote>;
  }
  return docs.map((doc) => {
    const prefetch = (): void => {
      prefetchDoc(owner, name, type, doc.doc_id);
    };
    return (
      <ShellSidebarTreeItem
        key={doc.doc_id}
        label={`${doc.doc_id} · ${doc.title}`}
        active={doc.doc_id.toLowerCase() === activeDocId?.toLowerCase()}
        render={
          <Link
            to={`/${owner}/${name}/${type}/${doc.doc_id}`}
            onMouseEnter={prefetch}
            onFocus={prefetch}
          />
        }
      />
    );
  });
}

function PageRow({
  owner,
  name,
  node,
  activePath,
  tree,
}: {
  owner: string;
  name: string;
  node: PageTreeNode;
  activePath: string | undefined;
  tree: ReturnType<typeof useTreeOpen>;
}) {
  const prefetchPage = usePrefetchPage();
  const prefetch = (): void => {
    prefetchPage(owner, name, node.path);
  };
  const isBranch = node.children.length > 0;
  return (
    <ShellSidebarTreeItem
      label={
        node.page !== undefined && !isBranch
          ? node.page.title
          : `${node.segment}/`
      }
      active={node.page !== undefined && node.path === activePath}
      // A directory nothing publishes at has no link: its row toggles.
      render={
        node.page === undefined ? undefined : (
          <Link
            to={`/${owner}/${name}/pages/${node.path}`}
            onMouseEnter={prefetch}
            onFocus={prefetch}
          />
        )
      }
      {...(isBranch ? tree.item(node.path) : {})}
    >
      {node.children.map((child) => (
        <PageRow
          key={child.path}
          owner={owner}
          name={name}
          node={child}
          activePath={activePath}
          tree={tree}
        />
      ))}
    </ShellSidebarTreeItem>
  );
}

/**
 * The pages tree (DESIGN-0004 OQ-2a). Mounted only when the repo's
 * `api:` block gate hit, so a repo that has not opted in never fires
 * the list; an empty list draws no group.
 */
function PagesGroup({
  owner,
  name,
  activePath,
}: {
  owner: string;
  name: string;
  activePath: string | undefined;
}) {
  // Same options as useRepoDocIndex's gated query — the cache is
  // shared, so the sidebar costs no extra request on opted repos.
  const pagesQuery = useListRepoPages(owner, name, {
    query: { staleTime: FIVE_MINUTES },
  });
  const pages =
    pagesQuery.data?.status === 200
      ? arr(pagesQuery.data.data.pages)
      : undefined;
  const nodes = useMemo(
    () => (pages === undefined ? [] : buildPageTree(pages)),
    [pages],
  );
  const ancestors = useMemo(() => pageAncestors(activePath), [activePath]);
  const tree = useTreeOpen(ancestors);

  if (nodes.length === 0) {
    return null;
  }
  return (
    <ShellSidebarGroup label="Pages">
      {nodes.map((node) => (
        <PageRow
          key={node.path}
          owner={owner}
          name={name}
          node={node}
          activePath={activePath}
          tree={tree}
        />
      ))}
    </ShellSidebarGroup>
  );
}

export function RepoSidebar({
  owner,
  name,
  type,
  docId,
  pagePath,
  pathname,
}: RepoRoute) {
  const repoId = `${owner}/${name}`;
  const repoQuery = useGetRepo(owner, name);
  const detail =
    repoQuery.data?.status === 200 ? repoQuery.data.data : undefined;
  const { facts } = useRepoFacts(repoId);

  // Changelog row (DESIGN-0002 Component 3): gated on the repo's own
  // config_snapshot — data this query already carries, zero probes.
  const changelogCfg = changelogConfig(detail?.config_snapshot);
  const prefetchChangelogFor = usePrefetchChangelog();
  const prefetchChangelog = (): void => {
    prefetchChangelogFor(owner, name);
  };

  // The route's `:type` segment (possibly an alias) picks the type
  // whose branch follows navigation.
  const types = arr(detail?.types);
  const activeType =
    type === undefined ? undefined : resolveDocType(types, type)?.name;
  const tree = useTreeOpen(activeType);

  // A repository docz-api does not know has no navigation to offer;
  // the route draws its own not-found panel.
  if (repoQuery.isError) {
    return null;
  }

  return (
    <>
      <ShellSidebarGroup label={repoId}>
        <ShellSidebarTreeItem
          label="Home"
          leading={<IconHome />}
          badge="index.md"
          active={pathname === `/${repoId}`}
          render={<Link to={`/${repoId}`} />}
        />
        {changelogCfg !== undefined && (
          <ShellSidebarTreeItem
            label="Changelog"
            leading={<IconHistory />}
            badge={changelogBasename(changelogCfg.file)}
            active={pathname === `/${repoId}/changelog`}
            render={
              <Link
                to={`/${repoId}/changelog`}
                // The full subpath as the tooltip only when the hint
                // shortens it (charts/<name>/CHANGELOG.md configs).
                title={
                  changelogCfg.file === changelogBasename(changelogCfg.file)
                    ? undefined
                    : changelogCfg.file
                }
                onMouseEnter={prefetchChangelog}
                onFocus={prefetchChangelog}
              />
            }
          />
        )}
      </ShellSidebarGroup>

      {apiConfig(detail?.config_snapshot) !== undefined && (
        <PagesGroup owner={owner} name={name} activePath={pagePath} />
      )}

      {types.length > 0 && (
        <ShellSidebarGroup label="Documents">
          {types.map((docType) => {
            // Facets omit zero-hit types, so once facts have loaded a
            // missing key IS a zero: it is the count, and it closes
            // the row.
            const count =
              facts === undefined
                ? undefined
                : (facts.typeCounts[docType.name] ?? 0);
            return (
              <ShellSidebarTreeItem
                key={docType.name}
                label={docType.name}
                leading={<TypeDot type={docType.name} />}
                badge={count}
                // The row is the type's index; a document under it is
                // the current page instead.
                active={docType.name === activeType && docId === undefined}
                // Links use the canonical type name; the API also
                // resolves id_prefix/alias URLs.
                render={<Link to={`/${repoId}/${docType.name}`} />}
                expandable
                disabled={count === 0}
                {...tree.item(docType.name)}
              >
                <TypeDocuments
                  owner={owner}
                  name={name}
                  type={docType.name}
                  activeDocId={docType.name === activeType ? docId : undefined}
                />
              </ShellSidebarTreeItem>
            );
          })}
        </ShellSidebarGroup>
      )}
    </>
  );
}
