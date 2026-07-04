package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the transactional Postgres access layer. It wraps a pgxpool and the
// sqlc-generated queries; callers hold one Store for the process lifetime.
type Store struct {
	pool *pgxpool.Pool
	q    *Queries
}

// NewStore builds a Store over an already-configured pool. The name avoids
// colliding with the sqlc-generated New, which takes a bare DBTX.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, q: New(pool)}
}

// Ping verifies Postgres is reachable; it backs the /readyz probe.
func (s *Store) Ping(ctx context.Context) error {
	if err := s.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	return nil
}

// Close releases the underlying pool. Safe to call once at shutdown.
func (s *Store) Close() { s.pool.Close() }

// UpsertInstallation creates or updates an installation. Repos reference it by
// foreign key, so it must exist before ReconcileRepo runs for that repo.
func (s *Store) UpsertInstallation(ctx context.Context, in InstallationInput) error {
	// InstallationInput mirrors UpsertInstallationParams field-for-field (all
	// plain scalars, no pgtype), so the direct conversion is safe and keeps the
	// generated type off the public signature.
	if err := s.q.UpsertInstallation(ctx, UpsertInstallationParams(in)); err != nil {
		return fmt.Errorf("upsert installation %d: %w", in.ID, err)
	}
	return nil
}

// The input types below are the store's public boundary. They use plain Go
// zero values for nullable columns (empty string / zero time); the store maps
// those to SQL NULL, keeping pgtype out of the ingest layer.
type (
	// InstallationInput is the desired state for an installation row, populated
	// at onboarding time (manual trigger now, webhook later).
	InstallationInput struct {
		ID           int64
		AccountLogin string
		AccountType  string
	}

	// RepoInput is the desired top-level state for a repo at a given HEAD.
	RepoInput struct {
		InstallationID int64
		Owner          string
		Name           string
		DefaultBranch  string
		DocsDir        string
		ConfigSnapshot json.RawMessage
		LastSyncedSHA  string
		ChangelogMD    string
		ChangelogSHA   string
	}

	// DocTypeInput is one desired doc-type row (from .docz.yaml).
	DocTypeInput struct {
		Name        string
		Dir         string
		IDPrefix    string
		PluralLabel string
		Statuses    json.RawMessage
		Aliases     json.RawMessage
	}

	// DocumentInput is one desired document row. ContentHash gates whether an
	// unchanged document is rewritten during reconcile.
	DocumentInput struct {
		Type        string
		DocID       string
		Title       string
		Status      string
		Author      string
		Created     time.Time
		Path        string
		GitSHA      string
		ContentHash string
		RawMD       string
	}

	// ReconcileInput is the full desired state for one repo: its top-level
	// fields, the doc types declared in its config, and every document
	// discovered at HEAD.
	ReconcileInput struct {
		Repo      RepoInput
		DocTypes  []DocTypeInput
		Documents []DocumentInput
	}

	// ReconcileResult summarizes what one ReconcileRepo call changed. It drives
	// structured logging and lets tests assert on the content-hash gate.
	// UpsertedDocIDs/DeletedDocIDs name the documents that actually changed, so
	// the search indexer can reuse the same content-hash gate instead of
	// re-indexing every document each run.
	ReconcileResult struct {
		RepoID         int64
		DocsUpserted   int
		DocsDeleted    int
		DocsUnchanged  int
		TypesUpserted  int
		TypesDeleted   int
		UpsertedDocIDs []string
		DeletedDocIDs  []string
	}
)

// textOrNull maps "" to SQL NULL and any other value to a valid pgtype.Text.
func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// dateOrNull maps the zero time to SQL NULL and any other value to a date.
func dateOrNull(t time.Time) pgtype.Date {
	if t.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}
