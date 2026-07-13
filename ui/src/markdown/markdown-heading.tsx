import { useEffect, useRef, useState } from "react";

import type { ComponentPropsWithoutRef } from "react";

/*
 * Heading anchor copy (IMPL-0002 Phase 6, OQ-8a): h2–h4 (the
 * ToC-collected set) get a hover/focus-revealed "#" button that
 * copies the absolute section URL. A real, labeled, keyboard-
 * reachable button — not a link — so prose links keep their
 * underline-only rule untouched. The ids come from rehype-slug
 * (post-sanitize, generated), never from document HTML.
 */

async function copySectionUrl(id: string): Promise<void> {
  const url = new URL(`#${id}`, window.location.href).toString();
  await navigator.clipboard.writeText(url);
}

function HeadingAnchor({ id }: { id: string }) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<number>(undefined);
  useEffect(
    () => () => {
      window.clearTimeout(timer.current);
    },
    [],
  );

  return (
    <button
      type="button"
      aria-label="Copy link to section"
      className="heading-anchor"
      onClick={() => {
        copySectionUrl(id).then(
          () => {
            setCopied(true);
            window.clearTimeout(timer.current);
            timer.current = window.setTimeout(() => {
              setCopied(false);
            }, 1600);
          },
          () => {
            // Clipboard unavailable (permissions, insecure context):
            // fall back to putting the anchor in the URL bar.
            window.location.hash = id;
          },
        );
      }}
    >
      <span aria-hidden>{copied ? "✓" : "#"}</span>
      <span role="status" className="sr-only">
        {copied ? "link copied" : ""}
      </span>
    </button>
  );
}

function makeMarkdownHeading(tag: "h2" | "h3" | "h4") {
  function MarkdownHeading({
    children,
    ...rest
  }: ComponentPropsWithoutRef<typeof tag>) {
    const Tag = tag;
    const id = typeof rest.id === "string" ? rest.id : undefined;
    return (
      <Tag {...rest}>
        {children}
        {id !== undefined && <HeadingAnchor id={id} />}
      </Tag>
    );
  }
  MarkdownHeading.displayName = `Markdown${tag.toUpperCase()}`;
  return MarkdownHeading;
}

export const MarkdownH2 = makeMarkdownHeading("h2");
export const MarkdownH3 = makeMarkdownHeading("h3");
export const MarkdownH4 = makeMarkdownHeading("h4");
