/*
 * Shared left-rail treatment for the repo nav's in-group rows (type
 * drawers and the pages tree), from the portal reference: the group
 * draws a hairline down its left edge, every row draws a 2px border
 * over it, and only the active row colors that border in.
 *
 * The bar is what makes the current document findable in a long
 * drawer — an accent text color alone disappears among the muted
 * siblings. Both files that render rows import from here rather than
 * from each other, which would be a cycle (repo-nav renders the pages
 * section).
 */

export const NAV_GROUP_RAIL = "ml-[0.45rem] border-l border-border-hairline";

/** @param extra - row-specific classes, typically the indent. */
export function navRowClass(isActive: boolean, extra = ""): string {
  return `-ml-px block min-w-0 overflow-hidden border-l-2 py-[0.17rem] pr-[0.45rem] text-[12.5px] text-ellipsis whitespace-nowrap ${extra} ${
    isActive
      ? "border-accent bg-(--color-accent-bg) text-accent"
      : "border-transparent text-fg-muted hover:text-fg-primary"
  }`;
}
