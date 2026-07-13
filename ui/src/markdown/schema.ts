import { defaultSchema } from "rehype-sanitize";

type Schema = typeof defaultSchema;

/*
 * The sanitize schema is the security boundary for every byte of
 * doc/markdown HTML. It is GitHub's default schema with one explicit
 * pin: fenced-code `language-*` classes must survive so @shikijs/rehype
 * (which runs AFTER sanitize) can pick a grammar. rehype-slug also runs
 * after sanitize, so heading ids need no allowance here — slug ids are
 * generated, not user input, while user-authored ids get the
 * `user-content-` clobber prefix from the default schema.
 *
 * Do not widen this schema without extending processor.xss.test.ts.
 */
export const sanitizeSchema: Schema = {
  ...defaultSchema,
  attributes: {
    ...defaultSchema.attributes,
    code: [
      ["className", /^language-./],
      // Fence meta → codeblock caption (IMPL-0002 Phase 2, OQ-6a):
      // conservative charset + length cap. A failing value drops the
      // caption; the code block itself always survives. The value is
      // only ever rendered as text (span.caption / data-caption).
      ["metastring", /^[\w ./#+:=@()-]{1,120}$/],
    ],
  },
};
