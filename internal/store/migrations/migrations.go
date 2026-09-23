// Package migrations holds the embedded goose SQL migrations for the docz-api
// Postgres schema. The .sql files are the single source of truth; the Migrate
// runner in internal/store applies them via goose. sqlc also reads these files
// as its schema source.
package migrations

import "embed"

// FS holds the embedded goose migration files.
//
//go:embed *.sql
var FS embed.FS
