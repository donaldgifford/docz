package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/donaldgifford/docz/v2/internal/export"
	"github.com/donaldgifford/docz/v2/internal/store"
	"github.com/donaldgifford/docz/v2/internal/telemetry"
)

// tracer is the instrumentation scope for the ingest worker spans.
var tracer = otel.Tracer("github.com/donaldgifford/docz/v2/internal/queue")

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

// Exporter is the narrow surface the worker needs to run one Confluence
// export. It matches export.Service.Run.
type Exporter interface {
	Run(ctx context.Context, repoID int64) (export.Result, error)
}

// Worker runs the asynq server that drains ingest and export jobs. Callers
// Start it (non-blocking) and Shutdown it (drains in-flight jobs). The pointer
// receiver is required: Worker holds *asynq.Server, which must not be copied.
type Worker struct {
	srv      *asynq.Server
	ingestor Ingestor
	exporter Exporter
}

// queueWeights serves ingest twice as often as export, so an export never
// starves an ingest at the worker's small concurrency.
var queueWeights = map[string]int{queueName: 2, exportQueueName: 1}

// NewWorker builds a Worker that processes ingest jobs with ing and, when exp
// is not nil, export jobs with exp. concurrency bounds the number of parallel
// jobs (2–4 suits a homelab single binary: each job holds a pool connection
// and issues GitHub or Confluence API calls). The asynq server connects to
// Redis only on Start, so NewWorker never blocks on Redis.
func NewWorker(redisURL string, concurrency int, ing Ingestor, exp Exporter) (*Worker, error) {
	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url for worker: %w", err)
	}
	// Logger/LogLevel route asynq's own diagnostics into slog, and ErrorHandler
	// reports failed attempts: asynq drops a handler's returned error unless this
	// hook is set (INV-0007 F1). Both read the configured default slog logger,
	// which main() installs before building the worker.
	srv := asynq.NewServer(opt, asynq.Config{
		Concurrency:              concurrency,
		Queues:                   queueWeights,
		IsFailure:                isFailure,
		DelayedTaskCheckInterval: delayedTaskCheckInterval,
		Logger:                   newAsynqLogger(nil),
		LogLevel:                 asynqLogLevel(nil),
		ErrorHandler:             asynq.ErrorHandlerFunc(logTaskFailure),
	})
	return &Worker{srv: srv, ingestor: ing, exporter: exp}, nil
}

// Start registers the handlers and starts the asynq server (non-blocking).
func (w *Worker) Start() error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskTypeIngest, w.handleIngest)
	if w.exporter != nil {
		mux.HandleFunc(TaskTypeExport, w.handleExport)
	}
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

	// Continue the trace started at the enqueue site (the webhook/HTTP request),
	// so the whole trigger → ingest flow is one trace.
	ctx = extractTrace(ctx, job.TraceParent, job.TraceState)
	ctx, span := tracer.Start(ctx, "queue.ingest",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("repo", job.repoLabel()),
			attribute.String("reason", job.Reason),
		),
	)
	defer span.End()

	slog.Info("processing ingest job", "repo", job.repoLabel(), "reason", job.Reason)

	start := time.Now()
	res, err := w.ingestor.Run(ctx, job.InstallationID, job.Owner, job.Name)
	if err != nil {
		// The returned error reaches asynq, which hands it to the ErrorHandler
		// registered in NewWorker (logIngestFailure) — asynq itself never logs it
		// (INV-0007 F1). That hook owns the log line for every attempt, so this
		// site records the span and metric only, and does not log twice.
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		telemetry.ObserveIngest(job.Reason, "failure", time.Since(start))
		return fmt.Errorf("ingest %s: %w", job.repoLabel(), err)
	}

	telemetry.ObserveIngest(job.Reason, "success", time.Since(start))
	slog.Info("ingest job complete",
		"repo", job.repoLabel(),
		"docs_upserted", res.DocsUpserted,
		"docs_deleted", res.DocsDeleted,
		"docs_unchanged", res.DocsUnchanged,
	)
	return nil
}

// handleExport is the asynq handler for TaskTypeExport. Like handleIngest it
// drops a malformed payload and returns any other error, which asynq retries
// unless the exporter wrapped asynq.SkipRetry. Logging a failure is
// logTaskFailure's.
func (w *Worker) handleExport(ctx context.Context, task *asynq.Task) error {
	job, err := unmarshalExportJob(task.Payload())
	if err != nil {
		slog.Error("export job has a malformed payload; dropping", "err", err)
		return fmt.Errorf("%w: %w", asynq.SkipRetry, err)
	}

	ctx = extractTrace(ctx, job.TraceParent, job.TraceState)
	ctx, span := tracer.Start(ctx, "queue.export",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("repo", job.repoLabel()),
			attribute.String("reason", job.Reason),
		),
	)
	defer span.End()

	slog.Info("processing export job", "repo", job.repoLabel(), "reason", job.Reason)

	start := time.Now()
	res, err := w.exporter.Run(ctx, job.RepoID)
	observeExport(job.Reason, &res, time.Since(start))

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("export %s: %w", job.repoLabel(), err)
	}

	slog.Info("export job complete",
		"repo", job.repoLabel(),
		"status", res.Status,
		"reason", res.Reason,
		"folder", res.Folder,
		"runs", res.Runs,
		"created", res.Counts.Created,
		"updated", res.Counts.Updated,
		"unchanged", res.Counts.Unchanged,
		"skipped", res.Counts.Skipped,
		"archived", res.Counts.Archived,
		"failed", res.Counts.Failed,
	)
	return nil
}

// observeExport records an export's metrics. A run that never reached a
// status (the snapshot could not be read) counts as failed.
func observeExport(reason string, res *export.Result, d time.Duration) {
	status := string(res.Status)
	if status == "" {
		status = string(export.StatusFailed)
	}

	telemetry.ObserveExport(reason, status, d)

	for action, n := range map[string]int{
		"created": res.Counts.Created, "updated": res.Counts.Updated, "unchanged": res.Counts.Unchanged,
		"skipped": res.Counts.Skipped, "archived": res.Counts.Archived, "failed": res.Counts.Failed,
	} {
		telemetry.ObserveExportPages(action, n)
	}
}

// isFailure is the asynq Config.IsFailure predicate: a context cancellation
// (e.g. process shutdown) is not a failure, so asynq re-queues the task for
// another worker rather than counting a retry against it.
func isFailure(err error) bool {
	return !errors.Is(err, context.Canceled)
}
