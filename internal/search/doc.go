// Package search is the Meilisearch access layer for docz-api. It configures
// the documents index, indexes and deletes documents keyed off the same
// content-hash gate as Postgres, and serves faceted full-text search with the
// authorize seam's repo filter injected. Callers depend on the narrow Indexer
// (ingest) and Searcher (httpapi) interfaces, both satisfied by *Client.
package search
