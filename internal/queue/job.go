// Package queue is the Redis-backed async ingestion layer for docz-api. It
// enqueues per-repo ingest jobs (asynq over Redis), coalesces bursts within a
// debounce window so the latest HEAD wins, and runs a worker pool that drives
// the Phase-2 ingest pipeline. Triggers (the manual onboard flag now, webhooks
// in Phase 5) enqueue through the Enqueuer interface and return promptly; the
// worker processes jobs with at-least-once delivery and retry, relying on the
// store's content-hash gate to keep re-runs cheap and idempotent.
package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// TaskTypeIngest is the asynq task type for a repository ingest job. The worker
// registers one handler for it.
const TaskTypeIngest = "ingest:repo"

// IngestJob is the payload for one repository ingest task. Reason is a
// human-readable label ("onboard", "webhook", "manual") for logging; it does
// not affect processing.
//
// The job carries no HEAD SHA: the worker always fetches current HEAD at process
// time, so a job delayed by the debounce window sees the truly latest state.
// This is the "latest-HEAD wins" property — it falls out for free.
type IngestJob struct {
	InstallationID int64  `json:"installation_id"`
	Owner          string `json:"owner"`
	Name           string `json:"name"`
	Reason         string `json:"reason"`
	// Trace context (W3C traceparent/tracestate) captured at the enqueue site so
	// the worker's span links back to the request that triggered it. Empty when
	// the trigger ran outside a trace (e.g. the -onboard CLI); safe either way.
	TraceParent string `json:"traceparent,omitempty"`
	TraceState  string `json:"tracestate,omitempty"`
}

// repoLabel is the "owner/name" identifier, used both for logging and as the
// coalesce key. owner/name is known before the repo row exists, so it works for
// a first-time onboard where no numeric repo id is available yet.
func (j *IngestJob) repoLabel() string { return j.Owner + "/" + j.Name }

// marshalJob encodes j to JSON for the asynq task payload.
func marshalJob(j *IngestJob) ([]byte, error) {
	b, err := json.Marshal(j)
	if err != nil {
		return nil, fmt.Errorf("marshal ingest job: %w", err)
	}
	return b, nil
}

// unmarshalJob decodes an asynq task payload into an IngestJob.
func unmarshalJob(payload []byte) (*IngestJob, error) {
	var j IngestJob
	if err := json.Unmarshal(payload, &j); err != nil {
		return nil, fmt.Errorf("unmarshal ingest job: %w", err)
	}
	return &j, nil
}

// injectTrace records the active span's W3C trace context onto job using the
// global propagator, so a downstream worker can continue the trace. It is a
// no-op when ctx carries no span (the fields stay empty).
func injectTrace(ctx context.Context, job *IngestJob) {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	job.TraceParent = carrier["traceparent"]
	job.TraceState = carrier["tracestate"]
}

// extractTrace returns a context carrying the remote span referenced by job's
// trace fields, so a worker span becomes a child of the enqueue-time span. It
// returns ctx unchanged when the job carries no trace context.
func extractTrace(ctx context.Context, job *IngestJob) context.Context {
	if job.TraceParent == "" {
		return ctx
	}
	carrier := propagation.MapCarrier{"traceparent": job.TraceParent}
	if job.TraceState != "" {
		carrier["tracestate"] = job.TraceState
	}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}
