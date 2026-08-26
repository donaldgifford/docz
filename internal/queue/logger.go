package queue

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
)

// asynqLogger adapts asynq's Logger interface onto slog so the library's own
// diagnostics join the structured log stream.
//
// Without this, asynq falls back to a stdlib log.Logger writing plain text to
// stderr. Its internal failures — most importantly the poller's "Dequeue error"
// line, which is how a Redis outage in the worker surfaces — would then be
// unparseable in a JSON log pipeline and effectively invisible (INV-0007 F7).
//
// Every line carries component=asynq so library diagnostics are filterable
// apart from the handler's own logging.
type asynqLogger struct {
	logger *slog.Logger
}

// asynqLogger implements the interface asynq.Config.Logger expects.
var _ asynq.Logger = (*asynqLogger)(nil)

// newAsynqLogger wraps logger, defaulting to slog's default when nil.
func newAsynqLogger(logger *slog.Logger) *asynqLogger {
	if logger == nil {
		logger = slog.Default()
	}
	return &asynqLogger{logger: logger}
}

func (l *asynqLogger) Debug(args ...any) { l.log(slog.LevelDebug, args) }
func (l *asynqLogger) Info(args ...any)  { l.log(slog.LevelInfo, args) }
func (l *asynqLogger) Warn(args ...any)  { l.log(slog.LevelWarn, args) }
func (l *asynqLogger) Error(args ...any) { l.log(slog.LevelError, args) }

// Fatal maps to Error rather than exiting. asynq's interface documents Fatal as
// terminating the process, but v0.26.0 never calls it (only its own default
// logger defines the method), and a logging adapter has no business taking the
// service down: an ingest-queue diagnostic must not kill the HTTP server.
func (l *asynqLogger) Fatal(args ...any) { l.log(slog.LevelError, args) }

// log renders asynq's fmt.Print-style variadic args into one message. asynq
// formats internally via Debugf/Errorf/… before calling these methods, so args
// is nearly always a single pre-formatted string.
func (l *asynqLogger) log(level slog.Level, args []any) {
	l.logger.Log(context.Background(), level, fmt.Sprint(args...), "component", "asynq")
}

// asynqLogLevel derives asynq's own level filter from what logger already
// accepts, so LOG_LEVEL governs library and application logging alike through a
// single source of truth. asynq defaults to InfoLevel, so without this a
// LOG_LEVEL=debug deployment would silently miss asynq's debug diagnostics.
func asynqLogLevel(logger *slog.Logger) asynq.LogLevel {
	if logger == nil {
		logger = slog.Default()
	}
	ctx := context.Background()
	switch {
	case logger.Enabled(ctx, slog.LevelDebug):
		return asynq.DebugLevel
	case logger.Enabled(ctx, slog.LevelInfo):
		return asynq.InfoLevel
	case logger.Enabled(ctx, slog.LevelWarn):
		return asynq.WarnLevel
	default:
		return asynq.ErrorLevel
	}
}

// logIngestFailure is the asynq ErrorHandler: it reports every failed attempt,
// including the retries and the final one before archiving.
//
// asynq never logs a handler's returned error itself — it hands the error to
// this hook if one is registered and otherwise drops it, logging only a bare
// "Retry exhausted for task id=…" at the end (INV-0007 F1). Without this
// handler an ingest could fail all five attempts leaving no diagnosable trace
// outside the Redis task record.
func logIngestFailure(ctx context.Context, task *asynq.Task, err error) {
	attrs := []any{"type", task.Type(), "err", err}
	if id, ok := asynq.GetTaskID(ctx); ok {
		attrs = append(attrs, "task_id", id)
	}
	if retried, ok := asynq.GetRetryCount(ctx); ok {
		attrs = append(attrs, "retried", retried)
	}
	if maxRetries, ok := asynq.GetMaxRetry(ctx); ok {
		attrs = append(attrs, "max_retry", maxRetries)
	}
	// Best-effort enrichment: a malformed payload is itself a reported failure,
	// so decode errors are ignored rather than masking the real one.
	if job, jerr := unmarshalJob(task.Payload()); jerr == nil {
		attrs = append(attrs, "repo", job.repoLabel(), "reason", job.Reason)
	}
	slog.ErrorContext(ctx, "ingest job attempt failed", attrs...)
}
