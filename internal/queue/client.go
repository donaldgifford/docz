package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

const (
	// queueName is the asynq queue holding ingest jobs. A dedicated queue lets
	// the worker prioritize ingest over future job types without config changes.
	queueName = "ingest"
	// maxRetry is the number of automatic retries for a failed ingest task. Five
	// retries with asynq's default exponential backoff covers transient GitHub
	// rate limits and Postgres blips; the content-hash gate makes each retry cheap.
	maxRetry = 5
)

// Enqueuer is the consumer-side interface for enqueueing an ingest job. Declared
// here (matching ingest.Indexer / httpapi.Searcher) so callers depend on the
// interface, not the Redis implementation. *Client satisfies it.
type Enqueuer interface {
	EnqueueIngest(ctx context.Context, job *IngestJob) error
}

// *Client is the production Enqueuer.
var _ Enqueuer = (*Client)(nil)

// taskInspector is the narrow asynq.Inspector surface the task-id conflict path
// needs. Declared consumer-side so the conflict dispatch is unit-testable
// without Redis; *asynq.Inspector satisfies it.
type taskInspector interface {
	GetTaskInfo(queue, id string) (*asynq.TaskInfo, error)
	DeleteTask(queue, id string) error
	Close() error
}

// Client is the Redis-backed enqueue client. One Client serves the whole
// process; it is safe for concurrent use. It also holds a plain go-redis client
// so /readyz can probe Redis reachability, and an asynq Inspector so a task id
// left behind by a finished run cannot block future triggers.
type Client struct {
	asynq     *asynq.Client
	inspector taskInspector
	redis     *redis.Client
	debounce  time.Duration
}

// NewClient builds a Client from a redis:// URL and a debounce window. The URL
// is parsed for the asynq client, the asynq inspector, and a go-redis client
// used by Ping.
func NewClient(redisURL string, debounce time.Duration) (*Client, error) {
	asynqOpt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url for queue: %w", err)
	}
	redisOpt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url for probe: %w", err)
	}
	return &Client{
		asynq:     asynq.NewClient(asynqOpt),
		inspector: asynq.NewInspector(asynqOpt),
		redis:     redis.NewClient(redisOpt),
		debounce:  debounce,
	}, nil
}

// ingestTaskID is the asynq task id for a repo's ingest job. It is deliberately
// stable per repo: that is what makes a burst of triggers coalesce into one run.
func ingestTaskID(job *IngestJob) string {
	return "ingest:" + job.repoLabel()
}

// EnqueueIngest schedules an ingest job for job.Owner/job.Name. The task id is
// "ingest:<owner>/<name>", so a second enqueue for the same repo while a run is
// pending returns ErrTaskIDConflict — treated as coalesced, since the pending
// job already covers the trigger. ProcessIn(debounce) delays execution so a
// burst of triggers collapses to one run at the latest HEAD.
//
// A conflicting id does not always mean a run is pending, so the conflict is
// inspected rather than assumed: see resolveTaskIDConflict.
//
// Known gap: a trigger arriving while the job is ACTIVE is dropped, because
// asynq holds the task id until the active run completes. The next trigger
// re-enqueues once the run finishes, and the content-hash gate makes any
// redundant re-run a cheap no-op.
func (c *Client) EnqueueIngest(ctx context.Context, job *IngestJob) error {
	// Capture the caller's trace context into the payload so the worker span
	// continues this trace across the Redis boundary.
	injectTrace(ctx, job)

	payload, err := marshalJob(job)
	if err != nil {
		return err
	}
	taskID := ingestTaskID(job)

	err = c.enqueue(ctx, payload, taskID)
	switch {
	case err == nil:
		slog.InfoContext(ctx, "ingest job enqueued",
			"repo", job.repoLabel(), "reason", job.Reason, "debounce", c.debounce)
		return nil
	case isTaskIDConflict(err):
		return c.resolveTaskIDConflict(ctx, job, payload, taskID)
	default:
		return fmt.Errorf("enqueue ingest for %s: %w", job.repoLabel(), err)
	}
}

// enqueue submits the task, returning asynq's error unwrapped so the caller can
// classify it.
func (c *Client) enqueue(ctx context.Context, payload []byte, taskID string) error {
	_, err := c.asynq.EnqueueContext(ctx,
		asynq.NewTask(TaskTypeIngest, payload),
		asynq.TaskID(taskID),
		asynq.Queue(queueName),
		asynq.ProcessIn(c.debounce),
		asynq.MaxRetry(maxRetry),
	)
	return err
}

// isTaskIDConflict reports whether err means the task id is already taken.
func isTaskIDConflict(err error) bool {
	return errors.Is(err, asynq.ErrTaskIDConflict) || errors.Is(err, asynq.ErrDuplicateTask)
}

// resolveTaskIDConflict decides what a taken task id actually means.
//
// asynq's uniqueness check is a bare EXISTS on the task key, which spans every
// state — including the terminal ones. A finished run therefore keeps owning
// the repo's id: archived for 90 days after retries are exhausted, completed
// for the retention TTL after a success. Treating every conflict as "a pending
// job already covers this" silently discarded those triggers (INV-0007 F4).
// Only a task that will still run counts as coalescing; a terminal one is
// cleared and the trigger re-enqueued.
func (c *Client) resolveTaskIDConflict(
	ctx context.Context, job *IngestJob, payload []byte, taskID string,
) error {
	info, ierr := c.inspector.GetTaskInfo(queueName, taskID)
	if classifyConflict(info, ierr) == coalesceTrigger {
		// classifyConflict already folds a nil info into coalescing, so this
		// branch must tolerate one: the real *asynq.Inspector always pairs a nil
		// info with an error, but the seam exists to be swapped and info is
		// dereferenced just below.
		if ierr != nil || info == nil {
			slog.WarnContext(ctx,
				"could not inspect the conflicting ingest task; treating as coalesced",
				"repo", job.repoLabel(), "reason", job.Reason, "err", ierr)
			return nil
		}
		// Info, not Debug: this branch also absorbs the active-window drop
		// documented on EnqueueIngest, so it is the one place a trigger can
		// legitimately go nowhere. It must be visible at the default log level.
		slog.InfoContext(ctx, "ingest job coalesced into an existing task",
			"repo", job.repoLabel(), "reason", job.Reason, "state", info.State.String())
		return nil
	}

	// ErrTaskNotFound means a concurrent trigger cleared it first — that is the
	// state this path is trying to reach, so it is success, not failure. Without
	// this, two pushes seconds apart make the loser answer 500, and because the
	// delivery id was already recorded, GitHub's redelivery is deduped to a
	// no-op instead of retrying.
	if derr := c.inspector.DeleteTask(queueName, taskID); derr != nil &&
		!errors.Is(derr, asynq.ErrTaskNotFound) {
		return fmt.Errorf("clear finished ingest task for %s: %w", job.repoLabel(), derr)
	}
	// Re-enqueued exactly once: if this conflicts again another enqueue won the
	// race, which means a run is now pending and the trigger is covered.
	if eerr := c.enqueue(ctx, payload, taskID); eerr != nil {
		if isTaskIDConflict(eerr) {
			slog.InfoContext(ctx, "ingest job coalesced after clearing a finished task",
				"repo", job.repoLabel(), "reason", job.Reason)
			return nil
		}
		return fmt.Errorf("re-enqueue ingest for %s: %w", job.repoLabel(), eerr)
	}

	slog.InfoContext(ctx, "cleared a finished ingest task and re-enqueued",
		"repo", job.repoLabel(), "reason", job.Reason, "cleared_state", info.State.String())
	return nil
}

// conflictAction is what a taken ingest task id means for the new trigger.
type conflictAction int

const (
	// coalesceTrigger: a run is still coming (or the state could not be read),
	// so this trigger is already covered and is safely dropped.
	coalesceTrigger conflictAction = iota
	// clearAndRetry: the id belongs to a task that has finished for good, so it
	// must be deleted before the trigger can be enqueued.
	clearAndRetry
)

// classifyConflict decides what to do about a taken task id. It is the whole
// decision in one pure function so the table of states is testable directly.
//
// An unreadable state fails open to coalescing: an inspection problem must not
// turn a webhook 202 into a 500, and the next trigger retries the whole dance.
func classifyConflict(info *asynq.TaskInfo, err error) conflictAction {
	if err != nil || info == nil {
		return coalesceTrigger
	}
	if isTerminalTaskState(info.State) {
		return clearAndRetry
	}
	return coalesceTrigger
}

// isTerminalTaskState reports whether a task in this state has finished for
// good and will never run again, so its hold on the task id is stale.
//
// Both terminal states are reachable in normal operation: completed after every
// successful ingest (for the retention TTL) and archived after retries are
// exhausted (for 90 days).
func isTerminalTaskState(state asynq.TaskState) bool {
	return state == asynq.TaskStateArchived || state == asynq.TaskStateCompleted
}

// Ping verifies Redis is reachable; it backs the /readyz probe.
func (c *Client) Ping(ctx context.Context) error {
	if err := c.redis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}
	return nil
}

// Close releases the asynq client, the inspector, and the go-redis client. Safe
// to call once at shutdown, after the worker has drained.
func (c *Client) Close() error {
	return errors.Join(
		wrapClose("asynq client", c.asynq.Close()),
		wrapClose("asynq inspector", c.inspector.Close()),
		wrapClose("redis client", c.redis.Close()),
	)
}

// wrapClose annotates a close error, or returns nil when there is none.
func wrapClose(what string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("close %s: %w", what, err)
}
