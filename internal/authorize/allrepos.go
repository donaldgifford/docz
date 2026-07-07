package authorize

import (
	"context"
	"fmt"
	"net/http"

	"github.com/donaldgifford/docz-api/internal/store"
)

// repoLister is the narrow store surface AllReposAuthorizer needs. *store.Store
// satisfies it.
type repoLister interface {
	ListRepos(ctx context.Context) ([]store.Repo, error)
}

// AllReposAuthorizer is the Phase 2 stub: every request is allowed to read all
// onboarded repos, regardless of session. Phase 5 replaces it with a per-user
// implementation of Authorizer.
type AllReposAuthorizer struct {
	repos repoLister
}

var _ Authorizer = (*AllReposAuthorizer)(nil)

// NewAllReposAuthorizer builds the stub over a repo lister.
func NewAllReposAuthorizer(repos repoLister) *AllReposAuthorizer {
	return &AllReposAuthorizer{repos: repos}
}

// Allowed returns the ids of every onboarded repo.
func (a *AllReposAuthorizer) Allowed(ctx context.Context, _ *http.Request) (AllowedRepos, error) {
	repos, err := a.repos.ListRepos(ctx)
	if err != nil {
		return nil, fmt.Errorf("list repos for authorize: %w", err)
	}
	ids := make(AllowedRepos, len(repos))
	for i := range repos {
		ids[i] = repos[i].ID
	}
	return ids, nil
}
