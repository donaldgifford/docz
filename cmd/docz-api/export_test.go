package main

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/donaldgifford/docz/v2/internal/queue"
	"github.com/donaldgifford/docz/v2/internal/store"
)

type fakeRepos map[string]int64

func (f fakeRepos) GetRepo(_ context.Context, owner, name string) (store.Repo, error) {
	id, ok := f[owner+"/"+name]
	if !ok {
		return store.Repo{}, pgx.ErrNoRows
	}
	return store.Repo{ID: id, Owner: owner, Name: name}, nil
}

type recordingExports struct{ jobs []queue.ExportJob }

func (r *recordingExports) EnqueueExport(_ context.Context, job *queue.ExportJob) error {
	r.jobs = append(r.jobs, *job)
	return nil
}

func TestRunExport(t *testing.T) {
	repos := fakeRepos{"acme/docs": 4}

	enq := &recordingExports{}
	if err := runExport(t.Context(), repos, enq, "acme/docs"); err != nil {
		t.Fatalf("runExport: %v", err)
	}
	want := queue.ExportJob{RepoID: 4, Owner: "acme", Name: "docs", Reason: "manual"}
	if len(enq.jobs) != 1 || enq.jobs[0] != want {
		t.Errorf("enqueued %+v, want [%+v]", enq.jobs, want)
	}

	if err := runExport(t.Context(), repos, enq, "acme/missing"); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("unknown repo: err = %v, want ErrNoRows", err)
	}

	for _, bad := range []string{"acme", "/docs", "acme/", "a/b/c"} {
		if err := runExport(t.Context(), repos, enq, bad); err == nil {
			t.Errorf("runExport(%q) = nil, want a parse error", bad)
		}
	}
	if len(enq.jobs) != 1 {
		t.Errorf("enqueued %d jobs, want only the valid one", len(enq.jobs))
	}
}
