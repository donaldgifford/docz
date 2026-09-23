package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hibiken/asynq"

	"github.com/donaldgifford/docz-api/internal/store"
)

// fakeIngestor records Run calls and returns a fixed result/error.
type fakeIngestor struct {
	calls  []ingestCall
	result store.ReconcileResult
	err    error
}

type ingestCall struct {
	installationID int64
	owner, name    string
}

func (f *fakeIngestor) Run(
	_ context.Context, installationID int64, owner, name string,
) (store.ReconcileResult, error) {
	f.calls = append(f.calls, ingestCall{installationID: installationID, owner: owner, name: name})
	return f.result, f.err
}

// makeTask builds an asynq task carrying job's JSON payload.
func makeTask(t *testing.T, job *IngestJob) *asynq.Task {
	t.Helper()
	payload, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("marshal job: %v", err)
	}
	return asynq.NewTask(TaskTypeIngest, payload)
}

func TestHandleIngestSuccess(t *testing.T) {
	ing := &fakeIngestor{result: store.ReconcileResult{DocsUpserted: 3}}
	w := &Worker{ingestor: ing}

	job := &IngestJob{InstallationID: 42, Owner: "acme", Name: "platform", Reason: "test"}
	if err := w.handleIngest(t.Context(), makeTask(t, job)); err != nil {
		t.Fatalf("handleIngest: %v", err)
	}
	if len(ing.calls) != 1 {
		t.Fatalf("Run called %d times, want 1", len(ing.calls))
	}
	got := ing.calls[0]
	if got.installationID != 42 || got.owner != "acme" || got.name != "platform" {
		t.Errorf("call = %+v, want 42/acme/platform", got)
	}
}

func TestHandleIngestMalformedPayloadSkipsRetry(t *testing.T) {
	w := &Worker{ingestor: &fakeIngestor{}}
	task := asynq.NewTask(TaskTypeIngest, []byte("not json"))

	err := w.handleIngest(t.Context(), task)
	if err == nil {
		t.Fatal("want an error for a malformed payload")
	}
	if !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("want SkipRetry in the error chain, got: %v", err)
	}
}

func TestHandleIngestTransientErrorRetries(t *testing.T) {
	ing := &fakeIngestor{err: errors.New("postgres timeout")}
	w := &Worker{ingestor: ing}

	err := w.handleIngest(t.Context(), makeTask(t, &IngestJob{InstallationID: 1, Owner: "a", Name: "b"}))
	if err == nil {
		t.Fatal("want an error for a transient failure")
	}
	// A transient error must NOT carry SkipRetry so asynq retries it.
	if errors.Is(err, asynq.SkipRetry) {
		t.Errorf("transient error should not carry SkipRetry: %v", err)
	}
}

func TestIsFailureIgnoresCancellation(t *testing.T) {
	if isFailure(context.Canceled) {
		t.Error("context.Canceled should not count as a failure")
	}
	if !isFailure(errors.New("real failure")) {
		t.Error("a real error should count as a failure")
	}
}
