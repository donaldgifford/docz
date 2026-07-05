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
	// retentionTTL keeps a completed task record in Redis for observability via
	// the asynq inspector.
	retentionTTL = 24 * time.Hour
)

// Enqueuer is the consumer-side interface for enqueueing an ingest job. Declared
// here (matching ingest.Indexer / httpapi.Searcher) so callers depend on the
// interface, not the Redis implementation. *Client satisfies it.
type Enqueuer interface {
	EnqueueIngest(ctx context.Context, job *IngestJob) error
}

// *Client is the production Enqueuer.
var _ Enqueuer = (*Client)(nil)

// Client is the Redis-backed enqueue client. One Client serves the whole
// process; it is safe for concurrent use. It also holds a plain go-redis client
// so /readyz can probe Redis reachability.
type Client struct {
	asynq    *asynq.Client
	redis    *redis.Client
	debounce time.Duration
}

// NewClient builds a Client from a redis:// URL and a debounce window. The URL
// is parsed for both the asynq client and a go-redis client used by Ping.
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
		asynq:    asynq.NewClient(asynqOpt),
		redis:    redis.NewClient(redisOpt),
		debounce: debounce,
	}, nil
}

// EnqueueIngest schedules an ingest job for job.Owner/job.Name. The task id is
// "ingest:<owner>/<name>", so a second enqueue for the same repo within the
// debounce window returns ErrTaskIDConflict — treated as coalesced (the pending
// job already covers the trigger). ProcessIn(debounce) delays execution so a
// burst of triggers collapses to one run at the latest HEAD.
//
// Known gap: a trigger arriving while the job is ACTIVE (not scheduled/pending)
// is dropped, because asynq holds the task id until the active run completes.
// That is acceptable for Phase 4: the next trigger re-enqueues once the run
// finishes, and the content-hash gate makes any redundant re-run a cheap no-op.
func (c *Client) EnqueueIngest(ctx context.Context, job *IngestJob) error {
	payload, err := marshalJob(job)
	if err != nil {
		return err
	}

	taskID := "ingest:" + job.repoLabel()
	_, err = c.asynq.EnqueueContext(ctx,
		asynq.NewTask(TaskTypeIngest, payload),
		asynq.TaskID(taskID),
		asynq.Queue(queueName),
		asynq.ProcessIn(c.debounce),
		asynq.MaxRetry(maxRetry),
		asynq.Retention(retentionTTL),
	)
	if err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) || errors.Is(err, asynq.ErrDuplicateTask) {
			slog.Debug("ingest job coalesced: a pending job already covers this trigger",
				"repo", job.repoLabel(), "reason", job.Reason)
			return nil
		}
		return fmt.Errorf("enqueue ingest for %s: %w", job.repoLabel(), err)
	}

	slog.Info("ingest job enqueued",
		"repo", job.repoLabel(), "reason", job.Reason, "debounce", c.debounce)
	return nil
}

// Ping verifies Redis is reachable; it backs the /readyz probe.
func (c *Client) Ping(ctx context.Context) error {
	if err := c.redis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}
	return nil
}

// Close releases the asynq and go-redis clients. Safe to call once at shutdown,
// after the worker has drained.
func (c *Client) Close() error {
	return errors.Join(
		wrapClose("asynq client", c.asynq.Close()),
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
