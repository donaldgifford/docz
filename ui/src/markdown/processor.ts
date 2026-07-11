import rehypeShikiFromHighlighter from "@shikijs/rehype/core";
import { toString as hastToString } from "hast-util-to-string";
import { toJsxRuntime } from "hast-util-to-jsx-runtime";
import type { ReactNode } from "react";
import { Fragment, jsx, jsxs } from "react/jsx-runtime";
import rehypeRaw from "rehype-raw";
import rehypeSanitize from "rehype-sanitize";
import rehypeSlug from "rehype-slug";
import remarkGfm from "remark-gfm";
import remarkParse from "remark-parse";
import remarkRehype from "remark-rehype";
import { createHighlighterCore, type HighlighterCore } from "shiki/core";
import { createJavaScriptRegexEngine } from "shiki/engine/javascript";
import { unified } from "unified";
import { visit } from "unist-util-visit";

import { MarkdownAnchor } from "@/markdown/markdown-anchor";
import { sanitizeSchema } from "@/markdown/schema";
import { linkifyDocIds, type XrefResolver } from "@/markdown/xrefs";

import type { Root } from "hast";

export interface TocEntry {
  depth: 2 | 3 | 4;
  text: string;
  id: string;
}

export interface RenderedMarkdown {
  content: ReactNode;
  toc: TocEntry[];
}

/*
 * Pipeline order is a security invariant (DESIGN-0001 Decision 6):
 * rehype-raw must parse embedded HTML BEFORE rehype-sanitize so the
 * sanitizer sees real nodes, and Shiki must run AFTER sanitize so its
 * (trusted, generated) styling survives. Never reorder; never render
 * markdown outside this module.
 */

// Slim grammar set per the design; each import is its own lazy chunk.
// The pure-JS regex engine avoids shipping the oniguruma wasm.
let highlighterPromise: Promise<HighlighterCore> | undefined;

function getHighlighter(): Promise<HighlighterCore> {
  highlighterPromise ??= createHighlighterCore({
    themes: [import("shiki/themes/tokyo-night.mjs")],
    langs: [
      import("shiki/langs/yaml.mjs"),
      import("shiki/langs/go.mjs"),
      import("shiki/langs/typescript.mjs"),
      import("shiki/langs/javascript.mjs"),
      import("shiki/langs/bash.mjs"),
      import("shiki/langs/json.mjs"),
      import("shiki/langs/hcl.mjs"),
      import("shiki/langs/sql.mjs"),
      import("shiki/langs/python.mjs"),
    ],
    engine: createJavaScriptRegexEngine(),
  });
  return highlighterPromise;
}

// remark-rehype already emits footnote ids with the `user-content-`
// prefix; the sanitizer's clobber step then prefixes every id again,
// so trusted footnote anchors end up `user-content-user-content-*`
// while their hrefs keep a single prefix. Collapsing the repeat
// restores the round trip without weakening clobbering — user-authored
// ids are never pre-prefixed, so they can't hit this path.
function rehypeCollapseDoubleClobber() {
  return (tree: Root): void => {
    visit(tree, "element", (node) => {
      const id = node.properties.id;
      if (
        typeof id === "string" &&
        id.startsWith("user-content-user-content-")
      ) {
        node.properties.id = id.replace(/^(?:user-content-)+/, "user-content-");
      }
    });
  };
}

const TOC_TAGS: Readonly<Record<string, 2 | 3 | 4 | undefined>> = {
  h2: 2,
  h3: 3,
  h4: 4,
};

function rehypeCollectToc(toc: TocEntry[]) {
  return (tree: Root): void => {
    visit(tree, "element", (node) => {
      const depth = TOC_TAGS[node.tagName];
      const id = node.properties.id;
      if (depth !== undefined && typeof id === "string") {
        toc.push({ depth, text: hastToString(node), id });
      }
    });
  };
}

export interface RenderOptions {
  /** Sibling doc-id resolver; tokens that resolve become router links. */
  xrefs?: XrefResolver;
}

export async function renderMarkdown(
  raw: string,
  options?: RenderOptions,
): Promise<RenderedMarkdown> {
  const toc: TocEntry[] = [];
  const highlighter = await getHighlighter();

  const processor = unified()
    .use(remarkParse)
    .use(remarkGfm)
    .use(remarkRehype, { allowDangerousHtml: true })
    .use(rehypeRaw)
    .use(rehypeSanitize, sanitizeSchema)
    .use(rehypeCollapseDoubleClobber)
    .use(rehypeSlug)
    .use(() => rehypeCollectToc(toc))
    .use(() =>
      // HighlighterCore is HighlighterGeneric<never, never>; the plugin
      // asks for <any, any>. Same object, incompatible variance — adapt
      // through the plugin's own parameter type.
      rehypeShikiFromHighlighter(
        highlighter as Parameters<typeof rehypeShikiFromHighlighter>[0],
        {
          theme: "tokyo-night",
          // Languages outside the slim set fall back to plain text.
          fallbackLanguage: "text",
        },
      ),
    );

  /* eslint-disable @typescript-eslint/no-unsafe-assignment --
     typescript-eslint's checker computes an error type in this
     unified/hast chain (the flagged statement shifts as imports change)
     while tsc and the raw TS compiler API both report zero diagnostics
     for the file (verified against TS 5.9.3). The explicit annotations
     keep every downstream use fully typed. */
  const hast: Root = await processor.run(processor.parse(raw));
  if (options?.xrefs !== undefined) {
    // After sanitize (structure is trusted); hrefs come from API data.
    linkifyDocIds(hast, options.xrefs);
  }
  const content: ReactNode = toJsxRuntime(hast, {
    Fragment,
    jsx,
    jsxs,
    components: { a: MarkdownAnchor },
  });
  /* eslint-enable @typescript-eslint/no-unsafe-assignment */
  return { content, toc };
}
