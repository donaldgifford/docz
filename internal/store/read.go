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

// GetDocTypesForRepo returns a repo's doc types. It wraps the same query the
// reconcile path uses, presenting one coherent store surface to the read API.
func (s *Store) GetDocTypesForRepo(ctx context.Context, repoID int64) ([]DocType, error) {
	types, err := s.q.ListDocTypes(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("list doc types for repo %d: %w", repoID, err)
	}
	return types, nil
}
