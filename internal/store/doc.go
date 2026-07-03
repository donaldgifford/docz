// Package store is the Postgres access layer for docz-api.
//
// It owns the schema migrations (embedded goose SQL, applied via Migrate) and,
// once generated, the sqlc-typed queries plus the transactional ReconcileRepo
// operation the ingest pipeline uses to upsert a repo's config, doc types, and
// documents in one transaction. The service runs on a pgxpool; goose migrations
// use a separate database/sql connection via pgx's stdlib adapter.
package store
