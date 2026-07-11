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
const TOC_BLOCK_RE = /[ \t]*<!--toc:start-->[\s\S]*?<!--toc:end-->[ \t]*\r?\n?/g;

export function preprocessDoczMarkdown(raw: string): string {
  return raw.replace(FRONTMATTER_RE, "").replace(TOC_BLOCK_RE, "");
}
