// Package consumer is a standalone-module smoke test: it imports the public
// docz packages exactly as an external consumer (docz-api, sdk-booty-sh)
// would, so a regression that narrows the promoted visibility or breaks the
// document -> config typed-field link fails here at compile time
// (DESIGN-0007 Testing Strategy; IMPL-0013 Phase 3; IMPL-0014 Phase 5).
//
// The v1 surface is the five frozen pkg/doczcore subpackages — config,
// document, docparse, docwrite, toc. IMPL-0018 Phase 1 adds the v2 type
// layer: pkg/doczcore/kinds and pkg/doczcore/validate, plus the five type
// packages pkg/rfc, pkg/adr, pkg/design, pkg/impl, and pkg/investigation.
// Phase 2 adds the three promotions that emptied internal/:
// pkg/doczcore/doctemplate, pkg/doczcore/index, and pkg/wiki. Phase 3 adds
// pkg/doczcore/repo, the repository tier. Sixteen pkg/ packages in all.
//
// repo is the one that decides whether the v2 claim holds. Every other file
// here proves a primitive is reachable; repo proves the operation is, so a
// consumer that wants to scaffold a repo, refresh its indexes, move a
// status, validate a tree, or migrate an unmarked document does not
// reimplement cmd/ to get it.
//
// Almost every call here is bytes in and values out, because docz-api reads a
// document through the GitHub API and has no checkout to point at. The few
// that take a path take a t.TempDir(), never a path in this repo.
package consumer
