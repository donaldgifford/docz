package ingest

import (
	"context"
	"errors"
	"testing"

	"github.com/donaldgifford/docz/v2/internal/queue"
)

// fakeExportQueue records export enqueues.
type fakeExportQueue struct {
	jobs []queue.ExportJob
	err  error
}

func (f *fakeExportQueue) EnqueueExport(_ context.Context, job *queue.ExportJob) error {
	f.jobs = append(f.jobs, *job)
	return f.err
}

const syncEnabledConfig = `docs_dir: docs
sync:
  confluence:
    enabled: true
    site: https://example.atlassian.net
    space: DOCZ
`

func exportSnap(config string) *RepoSnapshot {
	return &RepoSnapshot{HeadSHA: "h", DefaultBranch: "main", ConfigYAML: []byte(config)}
}

func TestRunEnqueuesAnExportWhenSyncIsEnabled(t *testing.T) {
	q := &fakeExportQueue{}
	svc := NewService(&captureReconciler{}, fakeFetcher{snap: exportSnap(syncEnabledConfig)}, nil).WithExporter(q)

	if _, err := svc.Run(t.Context(), 42, "acme", "platform"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := queue.ExportJob{RepoID: 1, Owner: "acme", Name: "platform", Reason: "ingest"}
	if len(q.jobs) != 1 || q.jobs[0] != want {
		t.Errorf("enqueued %+v, want [%+v]", q.jobs, want)
	}
}

func TestRunEnqueuesNoExportWhenSyncIsDisabled(t *testing.T) {
	q := &fakeExportQueue{}
	svc := NewService(&captureReconciler{}, fakeFetcher{snap: exportSnap("docs_dir: docs\n")}, nil).WithExporter(q)

	if _, err := svc.Run(t.Context(), 42, "acme", "platform"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(q.jobs) != 0 {
		t.Errorf("enqueued %+v, want none", q.jobs)
	}
}

// A failed enqueue is logged; the ingest has committed and still succeeds.
func TestRunSurvivesAFailedExportEnqueue(t *testing.T) {
	q := &fakeExportQueue{err: errors.New("redis down")}
	svc := NewService(&captureReconciler{}, fakeFetcher{snap: exportSnap(syncEnabledConfig)}, nil).WithExporter(q)

	if _, err := svc.Run(t.Context(), 42, "acme", "platform"); err != nil {
		t.Errorf("Run = %v, want nil despite the enqueue failure", err)
	}
}
