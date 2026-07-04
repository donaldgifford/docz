package store

import (
	"context"
	"fmt"
)

// ListRepos returns every onboarded repo, ordered by owner then name. It backs
// the repo-list endpoint and the authorize seam's allowed-repo set.
func (s *Store) ListRepos(ctx context.Context) ([]Repo, error) {
	repos, err := s.q.ListRepos(ctx)
	if err != nil {
		return nil, fmt.Errorf("list repos: %w", err)
	}
	return repos, nil
}

// GetRepo returns one repo by owner and name. A missing row surfaces as
// pgx.ErrNoRows for the caller to map to 404.
func (s *Store) GetRepo(ctx context.Context, owner, name string) (Repo, error) {
	repo, err := s.q.GetRepoByOwnerName(ctx, GetRepoByOwnerNameParams{Owner: owner, Name: name})
	if err != nil {
		return Repo{}, fmt.Errorf("get repo %s/%s: %w", owner, name, err)
	}
	return repo, nil
}

// GetDocTypesForRepo returns a repo's doc types. It wraps the same query the
// reconcile path uses, presenting one coherent store surface to the read API.
func (s *Store) GetDocTypesForRepo(ctx context.Context, repoID int64) ([]DocType, error) {
	types, err := s.q.ListDocTypes(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("list doc types for repo %d: %w", repoID, err)
	}
	return types, nil
}

// ListDocumentsByType returns a repo's documents of one canonical type, metadata
// only (no raw markdown), ordered by doc id.
func (s *Store) ListDocumentsByType(
	ctx context.Context, repoID int64, typeName string,
) ([]ListDocumentsByTypeRow, error) {
	docs, err := s.q.ListDocumentsByType(ctx, ListDocumentsByTypeParams{RepoID: repoID, Type: typeName})
	if err != nil {
		return nil, fmt.Errorf("list documents for repo %d type %q: %w", repoID, typeName, err)
	}
	return docs, nil
}

// GetDocumentByID returns one document (including raw markdown) by repo and doc
// id. A missing row surfaces as pgx.ErrNoRows for the caller to map to 404.
func (s *Store) GetDocumentByID(ctx context.Context, repoID int64, docID string) (Document, error) {
	doc, err := s.q.GetDocumentByID(ctx, GetDocumentByIDParams{RepoID: repoID, DocID: docID})
	if err != nil {
		return Document{}, fmt.Errorf("get document %q in repo %d: %w", docID, repoID, err)
	}
	return doc, nil
}

// GetDocumentsByIDs returns full document rows (including raw markdown) for the
// given doc ids within one repo, ordered by doc id. The search indexer calls it
// after a reconcile commit to build index documents for the docs that changed.
// The result may be shorter than docIDs if a doc was removed concurrently.
func (s *Store) GetDocumentsByIDs(ctx context.Context, repoID int64, docIDs []string) ([]Document, error) {
	if len(docIDs) == 0 {
		return nil, nil
	}
	docs, err := s.q.GetDocumentsByIDs(ctx, GetDocumentsByIDsParams{RepoID: repoID, DocIds: docIDs})
	if err != nil {
		return nil, fmt.Errorf("get documents by ids in repo %d: %w", repoID, err)
	}
	return docs, nil
}
