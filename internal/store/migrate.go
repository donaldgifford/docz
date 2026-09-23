package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"

	"github.com/donaldgifford/docz-api/internal/store/migrations"
)

// Migrate applies all pending Up migrations against the database at dsn. It is
// safe to call on every startup: goose skips migrations already recorded in
// goose_db_version.
func Migrate(ctx context.Context, dsn string) error {
	return withProvider(ctx, dsn, func(ctx context.Context, p *goose.Provider) error {
		_, err := p.Up(ctx)
		return err
	})
}

// MigrateDown rolls back every applied migration. It is for integration tests
// and explicit ops use only, never called on startup.
func MigrateDown(ctx context.Context, dsn string) error {
	return withProvider(ctx, dsn, func(ctx context.Context, p *goose.Provider) error {
		_, err := p.DownTo(ctx, 0)
		return err
	})
}

// withProvider opens a dedicated database/sql connection via pgx's stdlib
// adapter (goose needs database/sql, not the pgxpool the service runs on),
// builds a goose Provider over the embedded migrations, runs fn, and closes
// the connection.
func withProvider(
	ctx context.Context,
	dsn string,
	fn func(context.Context, *goose.Provider) error,
) (err error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open migration db: %w", err)
	}
	defer func() {
		// Prefer a real migration error; surface a close failure only if the
		// run otherwise succeeded.
		if cerr := db.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close migration db: %w", cerr)
		}
	}()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("create goose provider: %w", err)
	}
	if err := fn(ctx, provider); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
