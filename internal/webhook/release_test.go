package webhook

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-github/v88/github"
)

func TestLogReleaseDoesNotPanic(_ *testing.T) {
	// Release handling is log-only (versions feature deferred); this just proves
	// the wired subscription runs without touching the store or index.
	logRelease(&github.ReleaseEvent{
		Action:  github.Ptr("published"),
		Release: &github.RepositoryRelease{TagName: github.Ptr("v1.2.3")},
		Repo:    &github.Repository{FullName: github.Ptr("acme/widgets")},
	})
}

// erroringPurger fails every purge, exercising purgeIndex's best-effort branch.
type erroringPurger struct{}

func (erroringPurger) DeleteRepoDocuments(context.Context, int64) error {
	return errors.New("meili down")
}

func TestPurgeIndexIsBestEffort(_ *testing.T) {
	// A purge failure must not propagate: Postgres is the source of truth and a
	// stale index entry is harmless. The call simply returns.
	h := &Handler{purger: erroringPurger{}}
	h.purgeIndex(context.Background(), 42)

	// A nil purger is a no-op (index cleanup disabled, e.g. in tests).
	(&Handler{}).purgeIndex(context.Background(), 42)
}
