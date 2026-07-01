// Package consumer is a standalone-module smoke test: it imports the public
// pkg/doczcore surface exactly as an external consumer (docz-api) would, so a
// regression that narrows the promoted visibility or breaks the document ->
// config typed-field link fails here at compile time (DESIGN-0007 Testing
// Strategy; IMPL-0013 Phase 3).
package consumer
