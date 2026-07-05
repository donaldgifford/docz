package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"

	"github.com/donaldgifford/docz-api/internal/store"
)

// delayedTaskCheckInterval is how often the asynq server forwards scheduled
// (debounced) and retry tasks to the pending queue. asynq defaults to 5s; 1s
// keeps ingestion snappy — a debounced job runs within ~1s of its window
// closing rather than up to 5s later.
const delayedTaskCheckInterval = time.Second

// Ingestor is the narrow surface the worker needs to run one ingest. It matches
// ingest.Service.Run; the production implementation (in the composition root)
// builds a per-installation GitHub client per job. Declared here (consumer
// side) so the worker is testable with a fake.
type Ingestor interface {
	Run(ctx context.Context, installationID int64, owner, name string) (store.ReconcileResult, error)
}

// Worker runs the asynq server that drains ingest jobs. Callers Start it
// (non-blocking) and Shutdown it (drains in-flight jobs). The pointer receiver
// is required: Worker holds *asynq.Server, which must not be copied.
type Worker struct {
	srv      *asynq.Server
	ingestor Ingestor
}

// NewWorker builds a Worker that processes ingest jobs with ing. concurrency
// bounds the number of parallel ingests (2–4 suits a homelab single binary:
// each job holds a pool connection and issues GitHub API calls). The asynq
// server connects to Redis only on Start, so NewWorker never blocks on Redis.
func NewWorker(redisURL string, concurrency int, ing Ingestor) (*Worker, error) {
	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url for worker: %w", err)
	}
	srv := asynq.NewServer(opt, asynq.Config{
		Concurrency:              concurrency,
		Queues:                   map[string]int{queueName: 1},
		IsFailure:                isFailure,
		DelayedTaskCheckInterval: delayedTaskCheckInterval,
	})
	return &Worker{srv: srv, ingestor: ing}, nil
}

// Start registers the ingest handler and starts the asynq server (non-blocking).
func (w *Worker) Start() error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskTypeIngest, w.handleIngest)
	if err := w.srv.Start(mux); err != nil {
		return fmt.Errorf("start asynq worker: %w", err)
	}
	return nil
}

// Shutdown stops the asynq server gracefully: it stops accepting new tasks and
// blocks until in-flight handlers return. Call it after the HTTP server has
// drained, so no new enqueues arrive during the drain.
func (w *Worker) Shutdown() { w.srv.Shutdown() }

// handleIngest is the asynq handler for TaskTypeIngest. It decodes the payload
// and runs the ingest pipeline. A malformed payload is unfixable, so it is
// dropped via asynq.SkipRetry; any ingest error is returned so asynq retries
// with backoff (the content-hash gate makes retries idempotent and cheap).
func (w *Worker) handleIngest(ctx context.Context, task *asynq.Task) error {
	job, err := unmarshalJob(task.Payload())
	if err != nil {
		slog.Error("ingest job has a malformed payload; dropping", "err", err)
		return fmt.Errorf("%w: %w", asynq.SkipRetry, err)
	}

	slog.Info("processing ingest job", "repo", job.repoLabel(), "reason", job.Reason)

	res, err := w.ingestor.Run(ctx, job.InstallationID, job.Owner, job.Name)
	if err != nil {
		slog.Warn("ingest job failed; will retry", "repo", job.repoLabel(), "err", err)
		return fmt.Errorf("ingest %s: %w", job.repoLabel(), err)
	}

	slog.Info("ingest job complete",
		"repo", job.repoLabel(),
		"docs_upserted", res.DocsUpserted,
		"docs_deleted", res.DocsDeleted,
		"docs_unchanged", res.DocsUnchanged,
	)
	return nil
}

// isFailure is the asynq Config.IsFailure predicate: a context cancellation
// (e.g. process shutdown) is not a failure, so asynq re-queues the task for
// another worker rather than counting a retry against it.
func isFailure(err error) bool {
	return !errors.Is(err, context.Canceled)
}
