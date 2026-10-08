package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hibiken/asynq"

	"github.com/donaldgifford/docz/v2/internal/export"
)

// fakeExporter records Run calls and returns a fixed result and error.
type fakeExporter struct {
	repoIDs []int64
	result  export.Result
	err     error
}

func (f *fakeExporter) Run(_ context.Context, repoID int64) (export.Result, error) {
	f.repoIDs = append(f.repoIDs, repoID)
	return f.result, f.err
}

func exportTask(t *testing.T, job *ExportJob) *asynq.Task {
	t.Helper()
	payload, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("marshal job: %v", err)
	}
	return asynq.NewTask(TaskTypeExport, payload)
}

func TestHandleExportSuccess(t *testing.T) {
	exp := &fakeExporter{result: export.Result{Status: export.StatusSucceeded}}
	w := &Worker{exporter: exp}

	if err := w.handleExport(t.Context(), exportTask(t, &ExportJob{RepoID: 9, Owner: "a", Name: "b"})); err != nil {
		t.Fatalf("handleExport: %v", err)
	}
	if len(exp.repoIDs) != 1 || exp.repoIDs[0] != 9 {
		t.Errorf("Run calls = %v, want [9]", exp.repoIDs)
	}
}

func TestHandleExportMalformedPayloadSkipsRetry(t *testing.T) {
	w := &Worker{exporter: &fakeExporter{}}

	err := w.handleExport(t.Context(), asynq.NewTask(TaskTypeExport, []byte("nope")))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("err = %v, want SkipRetry", err)
	}
}

// The exporter decides what is retried: its SkipRetry passes through, and an
// error without one is retried.
func TestHandleExportPassesTheExportersRetryDecision(t *testing.T) {
	job := &ExportJob{RepoID: 1, Owner: "a", Name: "b"}

	retried := &Worker{exporter: &fakeExporter{err: errors.New("503")}}
	if err := retried.handleExport(t.Context(), exportTask(t, job)); err == nil || errors.Is(err, asynq.SkipRetry) {
		t.Errorf("transient: err = %v, want a retryable error", err)
	}

	hopeless := &Worker{exporter: &fakeExporter{err: errors.Join(asynq.SkipRetry, errors.New("401"))}}
	if err := hopeless.handleExport(t.Context(), exportTask(t, job)); !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("auth: err = %v, want SkipRetry", err)
	}
}

func TestQueueWeightsFavourIngest(t *testing.T) {
	if queueWeights[queueName] <= queueWeights[exportQueueName] || queueWeights[exportQueueName] == 0 {
		t.Errorf("weights = %v, want ingest above a non-zero export", queueWeights)
	}
}

func TestLogTaskFailureDispatchesExport(t *testing.T) {
	rec := captureLogs(t)
	job := &ExportJob{RepoID: 3, Owner: "acme", Name: "docs", Reason: "push"}

	logTaskFailure(context.Background(), exportTask(t, job), errors.New("boom"))

	got := rec.only(t)
	if got.msg != "export job attempt failed" || got.attrs["repo"] != "acme/docs" || got.attrs["reason"] != "push" {
		t.Errorf("logged %q %v", got.msg, got.attrs)
	}
}
