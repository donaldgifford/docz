/*
 * Pre-pipeline cleanup of docz `raw_md`. Two things never reach the
 * renderer: the YAML frontmatter block (its fields arrive structured on
 * the Document DTO) and the docz-generated
 * `<!--toc:start-->…<!--toc:end-->` block (the site builds its own ToC
 * from the rendered headings).
 */

// Frontmatter: a `---` fence on the very first line, closed by the next
// line consisting of `---`. Lazy body match keeps a later thematic
// break from being swallowed when no closing fence exists.
const FRONTMATTER_RE = /^---\r?\n[\s\S]*?\r?\n---[ \t]*\r?\n?/;

// docz ToC markers, including everything between them. Global: docz
// writes one block, but stray extras shouldn't survive either.
const TOC_BLOCK_RE =
  /[ \t]*<!--toc:start-->[\s\S]*?<!--toc:end-->[ \t]*\r?\n?/g;

// The first ATX h1 when it's the document's opening content — the
// reader header already renders the structured title, so keeping the
// markdown h1 would print it twice. Real docz output puts HTML comments
// (markdownlint pragmas) between frontmatter and the h1; those are
// skipped over and preserved. The comment body is `(?:[^-]|-(?!->))*`,
// NOT a lazy [\s\S]*?: raw_md is untrusted, and a lazy body overlaps
// the `-->` terminator, which backtracks exponentially on inputs like
// "<!--" + "--><!--"×N (js/redos). This shape can't match the
// terminator, so the first `-->` always ends the comment — linear.
const LEADING_H1_RE =
  /^(\s*(?:<!--(?:[^-]|-(?!->))*-->\s*)*)#[ \t][^\n]*\r?\n?/;

export interface PreprocessOptions {
  stripLeadingH1?: boolean;
  /**
   * Blank what would be cut instead of cutting it, so every remaining
   * line keeps its line number and the parser's `position` is the
   * document line (the docz-review render mode, DESIGN-0024 section 5).
   * Each removed line becomes an empty one; the line count is unchanged.
   */
  preserveLines?: boolean;
}

// Keeps a match's line breaks and drops everything else, so a removed
// span leaves as many empty lines as it occupied.
function blankLines(match: string): string {
  return match.replace(/[^\r\n]+/g, "");
}

export function preprocessDoczMarkdown(
  raw: string,
  options: PreprocessOptions = {},
): string {
  const cut = options.preserveLines === true ? blankLines : () => "";
  let out = raw.replace(FRONTMATTER_RE, cut).replace(TOC_BLOCK_RE, cut);
  if (options.stripLeadingH1 === true) {
    out = out.replace(LEADING_H1_RE, (match, lead: string) =>
      options.preserveLines === true
        ? lead + blankLines(match.slice(lead.length))
        : lead,
    );
  }
  return out;
}
