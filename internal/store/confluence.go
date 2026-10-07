package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ExportInputs is everything a Confluence export of one repository reads,
// taken from one snapshot (DESIGN-0021 §5 step 1).
type ExportInputs struct {
	Repo      Repo
	Documents []Document
	Pages     []RepoPage
	// Sync is the last run's record, nil when the repository has none.
	Sync *ConfluenceSync
	// PageIDs maps a property key to the page id the last export wrote it to.
	PageIDs map[string]string
}

// ExportInputs reads a repository's row, documents with their markdown,
// published pages, last sync, and recorded page ids in one REPEATABLE READ
// read-only transaction, so an ingest committing mid-read cannot hand the
// export half of each. A missing repository is pgx.ErrNoRows.
func (s *Store) ExportInputs(ctx context.Context, repoID int64) (in ExportInputs, err error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return in, fmt.Errorf("begin export read: %w", err)
	}

	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) && err == nil {
			err = fmt.Errorf("end export read: %w", rerr)
		}
	}()

	q := s.q.WithTx(tx)

	if in.Repo, err = q.GetRepoByID(ctx, repoID); err != nil {
		return in, fmt.Errorf("get repo %d: %w", repoID, err)
	}

	if in.Documents, err = q.ListDocumentsForExport(ctx, repoID); err != nil {
		return in, fmt.Errorf("list documents for export of repo %d: %w", repoID, err)
	}

	if in.Pages, err = q.ListRepoPagesForExport(ctx, repoID); err != nil {
		return in, fmt.Errorf("list pages for export of repo %d: %w", repoID, err)
	}

	sync, err := q.GetConfluenceSync(ctx, repoID)

	switch {
	case err == nil:
		in.Sync = &sync
	case !errors.Is(err, pgx.ErrNoRows):
		return in, fmt.Errorf("get confluence sync of repo %d: %w", repoID, err)
	}

	ids, err := q.ListConfluencePageIDs(ctx, repoID)
	if err != nil {
		return in, fmt.Errorf("list confluence page ids of repo %d: %w", repoID, err)
	}

	in.PageIDs = make(map[string]string, len(ids))
	for _, r := range ids {
		in.PageIDs[r.Key] = r.PageID
	}

	return in, nil
}

// UpsertConfluenceSync writes a repository's last export run.
func (s *Store) UpsertConfluenceSync(ctx context.Context, p *UpsertConfluenceSyncParams) error {
	if err := s.q.UpsertConfluenceSync(ctx, *p); err != nil {
		return fmt.Errorf("upsert confluence sync of repo %d: %w", p.RepoID, err)
	}

	return nil
}

// GetConfluenceSync returns a repository's last export run. A repository
// never exported is pgx.ErrNoRows.
func (s *Store) GetConfluenceSync(ctx context.Context, repoID int64) (ConfluenceSync, error) {
	sync, err := s.q.GetConfluenceSync(ctx, repoID)
	if err != nil {
		return ConfluenceSync{}, fmt.Errorf("get confluence sync of repo %d: %w", repoID, err)
	}

	return sync, nil
}

// UpsertConfluencePage writes one page's result. An empty page id or URL
// keeps the row's previous one.
func (s *Store) UpsertConfluencePage(ctx context.Context, p *UpsertConfluencePageParams) error {
	if err := s.q.UpsertConfluencePage(ctx, *p); err != nil {
		return fmt.Errorf("upsert confluence page %q of repo %d: %w", p.Key, p.RepoID, err)
	}

	return nil
}

// ListConfluencePages returns a repository's recorded pages, by key.
func (s *Store) ListConfluencePages(ctx context.Context, repoID int64) ([]ConfluencePage, error) {
	pages, err := s.q.ListConfluencePages(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("list confluence pages of repo %d: %w", repoID, err)
	}

	return pages, nil
}

// RecordConfluenceExport writes a run's sync row and every page row in one
// transaction, so the API never serves a sync from one run with pages from
// another.
func (s *Store) RecordConfluenceExport(
	ctx context.Context, sync *UpsertConfluenceSyncParams, pages []UpsertConfluencePageParams,
) (err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin export record: %w", err)
	}

	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) && err == nil {
			err = fmt.Errorf("rollback export record: %w", rerr)
		}
	}()

	q := s.q.WithTx(tx)

	if err := q.UpsertConfluenceSync(ctx, *sync); err != nil {
		return fmt.Errorf("upsert confluence sync of repo %d: %w", sync.RepoID, err)
	}

	for i := range pages {
		if err := q.UpsertConfluencePage(ctx, pages[i]); err != nil {
			return fmt.Errorf("upsert confluence page %q of repo %d: %w", pages[i].Key, pages[i].RepoID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit export record: %w", err)
	}

	return nil
}
