package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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

// DeleteInstallation removes an installation row. The ON DELETE CASCADE on
// repos (and transitively doc_types and documents) wipes every subordinate row
// in one statement. To also purge the search index, collect the installation's
// repo ids via ListRepoIDsByInstallation before calling this.
func (s *Store) DeleteInstallation(ctx context.Context, id int64) error {
	if err := s.q.DeleteInstallation(ctx, id); err != nil {
		return fmt.Errorf("delete installation %d: %w", id, err)
	}
	return nil
}

// ListRepoIDsByInstallation returns the ids of every repo under an installation.
// It is called before DeleteInstallation so the caller can purge each repo's
// documents from the search index once the CASCADE has removed the rows.
func (s *Store) ListRepoIDsByInstallation(ctx context.Context, installationID int64) ([]int64, error) {
	ids, err := s.q.ListRepoIDsByInstallation(ctx, installationID)
	if err != nil {
		return nil, fmt.Errorf("list repo ids for installation %d: %w", installationID, err)
	}
	return ids, nil
}

// DeleteRepo removes one repo by owner/name (CASCADE wipes its doc_types and
// documents) and returns the deleted repo's id so the caller can purge the same
// documents from the search index. A missing repo surfaces as pgx.ErrNoRows for
// the caller to treat as already-absent.
func (s *Store) DeleteRepo(ctx context.Context, owner, name string) (int64, error) {
	id, err := s.q.DeleteRepoByOwnerName(ctx, DeleteRepoByOwnerNameParams{Owner: owner, Name: name})
	if err != nil {
		return 0, fmt.Errorf("delete repo %s/%s: %w", owner, name, err)
	}
	return id, nil
}

// RecordDelivery records a webhook delivery id for idempotency. It returns
// isNew=true when the delivery was newly inserted and false when it was already
// present (a replayed or duplicate delivery), so the webhook handler can treat
// a repeat as a no-op. The underlying INSERT ... ON CONFLICT DO NOTHING returns
// no row on conflict, surfacing as pgx.ErrNoRows.
func (s *Store) RecordDelivery(ctx context.Context, deliveryID, event string) (bool, error) {
	_, err := s.q.RecordDelivery(ctx, RecordDeliveryParams{DeliveryID: deliveryID, Event: event})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("record delivery %s: %w", deliveryID, err)
	}
	return true, nil
}

// UpsertUser creates or refreshes the durable user row for a provider identity
// and returns its id. Email/login are optional (mapped to SQL NULL when empty).
func (s *Store) UpsertUser(ctx context.Context, in UserInput) (int64, error) {
	id, err := s.q.UpsertUser(ctx, UpsertUserParams{
		Provider: in.Provider,
		Subject:  in.Subject,
		Email:    textOrNull(in.Email),
		Login:    textOrNull(in.Login),
	})
	if err != nil {
		return 0, fmt.Errorf("upsert user %s/%s: %w", in.Provider, in.Subject, err)
	}
	return id, nil
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

	// UserInput is the desired state for a site-user row, populated at login
	// time from a provider Identity. Email/Login are optional.
	UserInput struct {
		Provider string
		Subject  string
		Email    string
		Login    string
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
