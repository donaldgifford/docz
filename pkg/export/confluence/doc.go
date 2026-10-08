// Package confluence exports docz documents to Confluence Cloud as pages.
//
// EXPERIMENTAL until v2.0.0: the surface may change between betas
// (ADR-0002 Decision 7). The five packages frozen at v1.0.0 are not
// affected; this one is not among them.
//
// It is an integration above the core, a sibling of pkg/wiki and the type
// packages rather than a member of pkg/doczcore (DESIGN-0020 §1): it imports
// the core, and the core never imports it. It is also the one package under
// pkg/ allowed a third-party markdown parser, goldmark, which the layer test
// permits for pkg/export/... alone.
//
// A page is one document. Render turns a document's markdown into
// Confluence storage format, bytes in and bytes out, with no filesystem and
// no network, so a server holding fetched blobs can call it as readily as
// the CLI. The page is titled "ID: Title", carries a docz content property
// naming the document and the hash of its rendered body, and is never
// overwritten once somebody has edited it in Confluence unless the caller
// forces it: content flows from the markdown to Confluence and never back.
//
// Three layers sit on top of each other:
//
//   - Render is pure: one document in, one storage-format page out.
//   - Client is the Confluence surface Export needs. HTTPClient implements
//     it over net/http through Atlassian's gateway, api.atlassian.com/ex/
//     confluence/<cloudId>, which takes scoped and unscoped API tokens
//     alike. Its errors are typed (AuthError, ConflictError, RequestError,
//     TitleError) and never carry a credential.
//   - Export takes its configuration from a repo.Repo and every byte from
//     ExportOptions.FS (os.DirFS of the root when nil), so a server with a
//     fetched tree and no checkout plans the same tree. It plans the page
//     tree (a home page, one page per type carrying the type's README index,
//     the documents under their type, the api: pages when
//     sync.confluence.api_pages is on), and reconciles each page: create,
//     update, leave unchanged, or skip one edited in Confluence. On a full
//     run, docz pages whose documents are gone move under an Archive page;
//     nothing is ever deleted.
//
// The tree has two layouts (DESIGN-0021 §2). In the folder layout, the
// default, a repository's pages live in a Confluence folder titled
// sync.confluence.folder or the repository's name, and every title but the
// home page's starts with the folder name and a colon, so any number of
// repositories can share a space without a title colliding. The folder and
// every page carry a docz property naming the repository, and a run never
// writes, adopts, or archives what another repository's property names,
// whatever ExportOptions.Force says. The page layout is Phase A's: the
// parent page is the root and titles are unprefixed. A caller that records
// page ids passes them back in ExportOptions.Pages, so a renamed document's
// page is renamed in place; ExportOptions.Overwrite updates a page edited
// in Confluence instead of skipping it.
//
// An update carries the inline comments readers left on a page: each
// comment's marker moves into the new body wherever the text it was
// anchored to survives, and the ones that cannot be placed are reported in
// PageResult.Comments.Lost rather than dropped silently (#158).
//
// Export prints nothing and holds no logger. Hooks, carried in the context
// by WithHooks, report each page and each HTTP response to a caller that
// wants to narrate.
//
// Package confluencetest is an in-memory Confluence site for tests of code
// built on this one: a Client itself, and over HTTP the endpoints
// HTTPClient calls.
package confluence
