// Package consumer is a standalone-module smoke test: it imports all five
// public pkg/doczcore subpackages — config, document, docparse, docwrite,
// toc — exactly as an external consumer (docz-api, sdk-booty-sh) would, so
// a regression that narrows the promoted visibility or breaks the
// document -> config typed-field link fails here at compile time
// (DESIGN-0007 Testing Strategy; IMPL-0013 Phase 3; IMPL-0014 Phase 5).
package consumer
