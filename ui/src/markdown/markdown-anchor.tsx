import { Link } from "react-router";

import type { ComponentPropsWithoutRef } from "react";

/*
 * Anchor mapping for rendered markdown: xref tokens (data-xref, hrefs
 * we build from API data in xrefs.ts) become router Links so sibling
 * docs swap without a reload; every other anchor — heading slugs,
 * author-written links — stays a native <a>.
 */
export function MarkdownAnchor(
  props: ComponentPropsWithoutRef<"a"> & { "data-xref"?: string | boolean },
) {
  const { "data-xref": xref, href, children, ...rest } = props;
  if (xref !== undefined && href !== undefined) {
    return (
      <Link to={href} {...rest} data-xref>
        {children}
      </Link>
    );
  }
  return (
    <a href={href} {...rest}>
      {children}
    </a>
  );
}
