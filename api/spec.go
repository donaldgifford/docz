// Package api holds the OpenAPI contract for docz-api's HTTP surface.
//
// Spec is the embedded contents of openapi.yaml (OpenAPI 3.1.0) — the single
// source of truth for the wire contract. It is served as-is at GET
// /openapi.yaml by cmd/docz-api and loaded via kin-openapi's LoadFromData in
// internal/httpapi's contract test, so served bytes and tested bytes are
// provably identical. Treat Spec as read-only.
package api

import _ "embed"

// Spec is the embedded OpenAPI 3.1.0 contract (openapi.yaml) — the single
// source of truth for docz-api's wire contract. It is served verbatim at GET
// /openapi.yaml and loaded via kin-openapi's LoadFromData in the contract test,
// so served bytes and tested bytes are provably identical. Treat as read-only.
//
//go:embed openapi.yaml
var Spec []byte
