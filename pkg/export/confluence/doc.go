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
//     alike. Its errors are typed (AuthError, ConflictError, RequestError)
//     and never carry a credential.
//   - Export reads a repository through repo.Repo, plans the page tree
//     (the parent page, one page per type carrying the type's README index,
//     the documents under their type, the api: pages when
//     sync.confluence.api_pages is on), and reconciles each page: create,
//     update, leave unchanged, or skip one edited in Confluence. On a full
//     run, docz pages whose documents are gone move under an Archive page;
//     nothing is ever deleted.
//
// Export prints nothing and holds no logger. Hooks, carried in the context
// by WithHooks, report each page and each HTTP response to a caller that
// wants to narrate.
package confluence
