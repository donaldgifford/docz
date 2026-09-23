package store

import (
	"context"
	"fmt"
)

// ReconcileRepo drives the DB to match the desired state of one repo at HEAD,
// in a single transaction: upsert the repo row, reconcile its doc types
// (upsert present, delete absent), then reconcile its documents (upsert
// new/changed, skip unchanged via the content-hash gate, delete absent).
//
// The installation referenced by in.Repo.InstallationID must already exist
// (see UpsertInstallation). On any error the whole transaction rolls back.
func (s *Store) ReconcileRepo(ctx context.Context, in *ReconcileInput) (res ReconcileResult, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return res, fmt.Errorf("begin tx: %w", err)
	}
	// Roll back on any early return; once Commit succeeds we skip it (a
	// rollback after commit would just return ErrTxClosed).
	var committed bool
	defer func() {
		if committed {
			return
		}
		if rerr := tx.Rollback(ctx); rerr != nil && err == nil {
			err = fmt.Errorf("rollback tx: %w", rerr)
		}
	}()

	q := s.q.WithTx(tx)

	repoID, err := q.UpsertRepo(ctx, UpsertRepoParams{
		InstallationID:    in.Repo.InstallationID,
		Owner:             in.Repo.Owner,
		Name:              in.Repo.Name,
		DefaultBranch:     in.Repo.DefaultBranch,
		DocsDir:           in.Repo.DocsDir,
		ConfigSnapshot:    in.Repo.ConfigSnapshot,
		LastSyncedSha:     textOrNull(in.Repo.LastSyncedSHA),
		ChangelogMd:       textOrNull(in.Repo.ChangelogMD),
		ChangelogSha:      textOrNull(in.Repo.ChangelogSHA),
		ChangelogFile:     textOrNull(in.Repo.ChangelogFile),
		IndexMd:           textOrNull(in.Repo.IndexMD),
		IndexSha:          textOrNull(in.Repo.IndexSHA),
		ApiLandingPage:    textOrNull(in.Repo.APILandingPage),
		ApiAdditionalDocs: jsonStringsOrNull(in.Repo.APIAdditionalDocs),
	})
	if err != nil {
		return res, fmt.Errorf("upsert repo %s/%s: %w", in.Repo.Owner, in.Repo.Name, err)
	}
	res.RepoID = repoID

	// Returning a non-nil error sets the named return before the deferred
	// rollback runs, so the transaction unwinds correctly.
	if derr := reconcileDocTypes(ctx, q, repoID, in.DocTypes, &res); derr != nil {
		return res, derr
	}
	if derr := reconcileDocuments(ctx, q, repoID, in.Documents, &res); derr != nil {
		return res, derr
	}
	if derr := reconcileRepoPages(ctx, q, repoID, in.Pages, &res); derr != nil {
		return res, derr
	}

	if err = tx.Commit(ctx); err != nil {
		return res, fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return res, nil
}

// reconcileDocTypes upserts every desired doc type and deletes any existing
// type whose name is absent from the desired set.
func reconcileDocTypes(
	ctx context.Context, q *Queries, repoID int64, desired []DocTypeInput, res *ReconcileResult,
) error {
	existing, err := q.ListDocTypes(ctx, repoID)
	if err != nil {
		return fmt.Errorf("list doc types for repo %d: %w", repoID, err)
	}

	keep := make(map[string]struct{}, len(desired))
	for _, dt := range desired {
		keep[dt.Name] = struct{}{}
		if uerr := q.UpsertDocType(ctx, UpsertDocTypeParams{
			RepoID:      repoID,
			Name:        dt.Name,
			Dir:         dt.Dir,
			IDPrefix:    dt.IDPrefix,
			PluralLabel: dt.PluralLabel,
			Statuses:    dt.Statuses,
			Aliases:     dt.Aliases,
		}); uerr != nil {
			return fmt.Errorf("upsert doc type %q: %w", dt.Name, uerr)
		}
		res.TypesUpserted++
	}

	for i := range existing {
		name := existing[i].Name
		if _, ok := keep[name]; ok {
			continue
		}
		if derr := q.DeleteDocType(ctx, DeleteDocTypeParams{RepoID: repoID, Name: name}); derr != nil {
			return fmt.Errorf("delete doc type %q: %w", name, derr)
		}
		res.TypesDeleted++
	}
	return nil
}

// reconcileDocuments upserts documents that are new or whose content hash
// changed, skips those whose hash is unchanged, and deletes documents absent
// from the desired set (deleted-from-HEAD).
func reconcileDocuments(
	ctx context.Context, q *Queries, repoID int64, desired []DocumentInput, res *ReconcileResult,
) error {
	rows, err := q.ListDocumentHashes(ctx, repoID)
	if err != nil {
		return fmt.Errorf("list document hashes for repo %d: %w", repoID, err)
	}
	current := make(map[string]string, len(rows))
	for _, r := range rows {
		current[r.DocID] = r.ContentHash
	}

	keep := make(map[string]struct{}, len(desired))
	for i := range desired {
		d := &desired[i]
		keep[d.DocID] = struct{}{}
		if hash, ok := current[d.DocID]; ok && hash == d.ContentHash {
			res.DocsUnchanged++
			continue
		}
		if uerr := q.UpsertDocument(ctx, UpsertDocumentParams{
			RepoID:      repoID,
			Type:        d.Type,
			DocID:       d.DocID,
			Title:       d.Title,
			Status:      textOrNull(d.Status),
			Author:      textOrNull(d.Author),
			Created:     dateOrNull(d.Created),
			Path:        d.Path,
			GitSha:      d.GitSHA,
			ContentHash: d.ContentHash,
			RawMd:       d.RawMD,
		}); uerr != nil {
			return fmt.Errorf("upsert document %q: %w", d.DocID, uerr)
		}
		res.DocsUpserted++
		res.UpsertedDocIDs = append(res.UpsertedDocIDs, d.DocID)
	}

	for docID := range current {
		if _, ok := keep[docID]; ok {
			continue
		}
		if derr := q.DeleteDocument(ctx, DeleteDocumentParams{RepoID: repoID, DocID: docID}); derr != nil {
			return fmt.Errorf("delete document %q: %w", docID, derr)
		}
		res.DocsDeleted++
		res.DeletedDocIDs = append(res.DeletedDocIDs, docID)
	}
	return nil
}

// reconcileRepoPages upserts pages that are new or whose content hash
// changed, skips those whose hash is unchanged, and deletes pages absent
// from the desired set — the reconcileDocuments discipline keyed on the
// published path. A dormant api: block arrives as an empty desired set, so
// every existing row is deleted (desired state, not a cache).
func reconcileRepoPages(
	ctx context.Context, q *Queries, repoID int64, desired []PageInput, res *ReconcileResult,
) error {
	rows, err := q.ListRepoPageHashes(ctx, repoID)
	if err != nil {
		return fmt.Errorf("list page hashes for repo %d: %w", repoID, err)
	}
	current := make(map[string]string, len(rows))
	for _, r := range rows {
		current[r.Path] = r.ContentHash
	}

	keep := make(map[string]struct{}, len(desired))
	for i := range desired {
		p := &desired[i]
		keep[p.Path] = struct{}{}
		if hash, ok := current[p.Path]; ok && hash == p.ContentHash {
			res.PagesUnchanged++
			continue
		}
		if uerr := q.UpsertRepoPage(ctx, UpsertRepoPageParams{
			RepoID:      repoID,
			Path:        p.Path,
			RepoPath:    p.RepoPath,
			Title:       p.Title,
			GitSha:      p.GitSHA,
			ContentHash: p.ContentHash,
			RawMd:       p.RawMD,
		}); uerr != nil {
			return fmt.Errorf("upsert page %q: %w", p.Path, uerr)
		}
		res.PagesUpserted++
		res.UpsertedPagePaths = append(res.UpsertedPagePaths, p.Path)
	}

	for path := range current {
		if _, ok := keep[path]; ok {
			continue
		}
		if derr := q.DeleteRepoPage(ctx, DeleteRepoPageParams{RepoID: repoID, Path: path}); derr != nil {
			return fmt.Errorf("delete page %q: %w", path, derr)
		}
		res.PagesDeleted++
		res.DeletedPagePaths = append(res.DeletedPagePaths, path)
	}
	return nil
}
