package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
)

// TaskTypeExport is the asynq task type for a repository's Confluence export
// (DESIGN-0021 §5).
const TaskTypeExport = "export:confluence"

// exportQueueName is the asynq queue holding export jobs, weighted below
// ingest so an export never starves one.
const exportQueueName = "export"

// ExportJob is the payload for one Confluence export task. The ingest that
// enqueues it has already written the repository's row, so the job names it
// by id; Owner and Name are for the task id and the logs.
type ExportJob struct {
	RepoID int64  `json:"repo_id"`
	Owner  string `json:"owner"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
	// Trace context captured at the enqueue site, as IngestJob carries.
	TraceParent string `json:"traceparent,omitempty"`
	TraceState  string `json:"tracestate,omitempty"`
}

func (j *ExportJob) repoLabel() string { return j.Owner + "/" + j.Name }
func (j *ExportJob) jobReason() string { return j.Reason }
func (*ExportJob) spec() taskSpec      { return exportSpec }

// options carries no ProcessIn: the ingest that enqueues an export was
// already debounced.
func (*ExportJob) options(*Client) []asynq.Option {
	return []asynq.Option{asynq.MaxRetry(maxRetry)}
}

// exportTaskID is the asynq task id for a repository's export, stable per
// repository so a burst of ingests coalesces onto one export.
func exportTaskID(job *ExportJob) string {
	return "export:" + job.repoLabel()
}

// unmarshalExportJob decodes an asynq task payload into an ExportJob.
func unmarshalExportJob(payload []byte) (*ExportJob, error) {
	var j ExportJob
	if err := json.Unmarshal(payload, &j); err != nil {
		return nil, fmt.Errorf("unmarshal export job: %w", err)
	}
	return &j, nil
}

// ExportEnqueuer is the consumer-side interface for enqueueing an export.
// *Client satisfies it.
type ExportEnqueuer interface {
	EnqueueExport(ctx context.Context, job *ExportJob) error
}

var _ ExportEnqueuer = (*Client)(nil)

// EnqueueExport schedules a Confluence export of job.Owner/job.Name. The task
// id is "export:<owner>/<name>": a second enqueue while one is pending
// coalesces, and a finished task's id is cleared, exactly as EnqueueIngest
// does (resolveTaskIDConflict). An export enqueued while one is active is
// coalesced onto it, which is why the export reruns itself when the head
// moved while it ran.
func (c *Client) EnqueueExport(ctx context.Context, job *ExportJob) error {
	job.TraceParent, job.TraceState = injectTrace(ctx)

	payload, err := marshalJob(job)
	if err != nil {
		return err
	}
	taskID := exportTaskID(job)

	err = c.enqueue(ctx, job, payload, taskID)
	switch {
	case err == nil:
		slog.InfoContext(ctx, "export job enqueued", "repo", job.repoLabel(), "reason", job.Reason)
		return nil
	case isTaskIDConflict(err):
		return c.resolveTaskIDConflict(ctx, job, payload, taskID)
	default:
		return fmt.Errorf("enqueue export for %s: %w", job.repoLabel(), err)
	}
}
