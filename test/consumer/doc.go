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
// Every call here is bytes in and values out, because docz-api reads a
// document through the GitHub API and has no checkout to point at.
package consumer
