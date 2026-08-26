//go:build integration

// Package queue_test integration tests run the asynq queue against a real Redis
// (testcontainers): a job drains through the worker, a burst coalesces to one
// run at the latest HEAD, shutdown drains an in-flight job without loss, and
// every failure — the handler's and asynq's own — reaches the structured log.
package queue_test

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/donaldgifford/docz-api/internal/queue"
	"github.com/donaldgifford/docz-api/internal/store"
)

// redisURL points at the shared Redis container started in TestMain.
var redisURL string

func TestMain(m *testing.M) {
	os.Exit(runMain(m))
}

func runMain(m *testing.M) int {
	ctx := context.Background()

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		log.Printf("start redis: %v", err)
		return 1
	}
	defer func() {
		if terr := ctr.Terminate(ctx); terr != nil {
			log.Printf("terminate redis: %v", terr)
		}
	}()

	host, err := ctr.Host(ctx)
	if err != nil {
		log.Printf("redis host: %v", err)
		return 1
	}
	port, err := ctr.MappedPort(ctx, "6379/tcp")
	if err != nil {
		log.Printf("redis port: %v", err)
		return 1
	}
	redisURL = "redis://" + host + ":" + port.Port()

	return m.Run()
}

// countingIngestor counts Run calls; it stands in for the real ingest pipeline.
type countingIngestor struct{ count atomic.Int64 }

func (c *countingIngestor) Run(
	_ context.Context, _ int64, _, _ string,
) (store.ReconcileResult, error) {
	c.count.Add(1)
	return store.ReconcileResult{}, nil
}

// slowIngestor signals when a job starts and marks completion after a delay, so
// a test can assert shutdown drains an in-flight job.
type slowIngestor struct {
	started   chan struct{}
	delay     time.Duration
	completed atomic.Bool
}

func (s *slowIngestor) Run(
	_ context.Context, _ int64, _, _ string,
) (store.ReconcileResult, error) {
	close(s.started)
	time.Sleep(s.delay)
	s.completed.Store(true)
	return store.ReconcileResult{}, nil
}

// waitForCount polls until ing reaches want or the deadline passes.
func waitForCount(ing *countingIngestor, want int64, within time.Duration) int64 {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if ing.count.Load() >= want {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return ing.count.Load()
}

func TestEnqueueAndDrain(t *testing.T) {
	ing := &countingIngestor{}
	client, err := queue.NewClient(redisURL, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			t.Logf("close client: %v", cerr)
		}
	}()

	worker, err := queue.NewWorker(redisURL, 1, ing)
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	if err := worker.Start(); err != nil {
		t.Fatalf("start worker: %v", err)
	}
	defer worker.Shutdown()

	job := &queue.IngestJob{InstallationID: 42, Owner: "acme", Name: "drain", Reason: "test"}
	if err := client.EnqueueIngest(t.Context(), job); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if got := waitForCount(ing, 1, 4*time.Second); got != 1 {
		t.Errorf("Run called %d times, want 1", got)
	}
}

func TestDebounceCoalesces(t *testing.T) {
	ing := &countingIngestor{}
	const debounce = 500 * time.Millisecond
	client, err := queue.NewClient(redisURL, debounce)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			t.Logf("close client: %v", cerr)
		}
	}()

	worker, err := queue.NewWorker(redisURL, 1, ing)
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	if err := worker.Start(); err != nil {
		t.Fatalf("start worker: %v", err)
	}
	defer worker.Shutdown()

	// Fire five triggers for one repo within the debounce window: the first
	// schedules the job, the rest coalesce onto it.
	job := &queue.IngestJob{InstallationID: 42, Owner: "acme", Name: "coalesce", Reason: "burst"}
	for range 5 {
		if err := client.EnqueueIngest(t.Context(), job); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}

	// Wait past the debounce window plus processing time, then assert exactly one run.
	got := waitForCount(ing, 1, 4*time.Second)
	time.Sleep(300 * time.Millisecond) // give any erroneous extra runs a chance to appear
	if final := ing.count.Load(); got != 1 || final != 1 {
		t.Errorf("Run called %d times for a 5-trigger burst, want 1 (coalesced)", final)
	}
}

func TestShutdownDrainsInFlight(t *testing.T) {
	ing := &slowIngestor{started: make(chan struct{}), delay: 800 * time.Millisecond}
	client, err := queue.NewClient(redisURL, 0) // no debounce: process immediately
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			t.Logf("close client: %v", cerr)
		}
	}()

	worker, err := queue.NewWorker(redisURL, 1, ing)
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	if err := worker.Start(); err != nil {
		t.Fatalf("start worker: %v", err)
	}

	job := &queue.IngestJob{InstallationID: 1, Owner: "acme", Name: "drain-inflight"}
	if err := client.EnqueueIngest(t.Context(), job); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case <-ing.started:
	case <-time.After(5 * time.Second):
		t.Fatal("job did not start within 5s")
	}

	// Shutdown must block until the in-flight handler returns.
	worker.Shutdown()
	if !ing.completed.Load() {
		t.Error("in-flight job did not complete before Shutdown returned")
	}
}

// failingIngestor always fails, so the worker's error path runs for real.
type failingIngestor struct {
	attempts atomic.Int64
	err      error
}

func (f *failingIngestor) Run(
	_ context.Context, _ int64, _, _ string,
) (store.ReconcileResult, error) {
	f.attempts.Add(1)
	return store.ReconcileResult{}, f.err
}

// logRecorder is a slog.Handler capturing records for assertions. The
// integration tests live in package queue_test, so they observe logging through
// the same public surface an operator does.
type logRecorder struct {
	mu      sync.Mutex
	records []map[string]string
}

func (*logRecorder) Enabled(context.Context, slog.Level) bool { return true }

//nolint:gocritic // hugeParam: signature mandated by slog.Handler.
func (r *logRecorder) Handle(_ context.Context, rec slog.Record) error {
	entry := map[string]string{"__msg": rec.Message, "__level": rec.Level.String()}
	rec.Attrs(func(a slog.Attr) bool {
		entry[a.Key] = a.Value.String()
		return true
	})
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, entry)
	return nil
}

func (r *logRecorder) WithAttrs([]slog.Attr) slog.Handler { return r }
func (r *logRecorder) WithGroup(string) slog.Handler      { return r }

// find returns the captured records whose message equals msg.
func (r *logRecorder) find(msg string) []map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []map[string]string
	for _, rec := range r.records {
		if rec["__msg"] == msg {
			out = append(out, rec)
		}
	}
	return out
}

// findAttr returns the captured records carrying key=value.
func (r *logRecorder) findAttr(key, value string) []map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []map[string]string
	for _, rec := range r.records {
		if rec[key] == value {
			out = append(out, rec)
		}
	}
	return out
}

// captureDefaultLogs swaps the slog default for a recorder for the duration of
// the test. These tests must not run in parallel: the default logger is global.
func captureDefaultLogs(t *testing.T) *logRecorder {
	t.Helper()
	rec := &logRecorder{}
	prev := slog.Default()
	slog.SetDefault(slog.New(rec))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return rec
}

// waitFor polls cond until it holds or the deadline passes.
func waitFor(cond func() bool, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

// A failing ingest must leave a diagnosable log line. Before the ErrorHandler
// was registered, asynq swallowed the returned error entirely and the cause was
// recoverable only from the Redis task record (INV-0007 F1).
//
// This asserts the first attempt only: asynq's default retry backoff is tens of
// seconds, so waiting for all five would make the test minutes long. One real
// attempt through real Redis proves the hook is wired.
func TestFailedIngestLogsTheError(t *testing.T) {
	rec := captureDefaultLogs(t)
	ing := &failingIngestor{err: errors.New("fetch exploded")}

	client, err := queue.NewClient(redisURL, 0)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			t.Logf("close client: %v", cerr)
		}
	}()

	worker, err := queue.NewWorker(redisURL, 1, ing)
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	if err := worker.Start(); err != nil {
		t.Fatalf("start worker: %v", err)
	}
	defer worker.Shutdown()

	job := &queue.IngestJob{InstallationID: 9, Owner: "acme", Name: "fails", Reason: "push"}
	if err := client.EnqueueIngest(t.Context(), job); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if !waitFor(func() bool { return len(rec.find("ingest job attempt failed")) > 0 }, 10*time.Second) {
		t.Fatal("no 'ingest job attempt failed' record after a failing ingest")
	}

	got := rec.find("ingest job attempt failed")[0]
	if got["repo"] != "acme/fails" {
		t.Errorf("repo = %q, want %q", got["repo"], "acme/fails")
	}
	if got["reason"] != "push" {
		t.Errorf("reason = %q, want %q", got["reason"], "push")
	}
	if !strings.Contains(got["err"], "fetch exploded") {
		t.Errorf("err = %q, want it to name the underlying cause", got["err"])
	}
	if got["__level"] != "ERROR" {
		t.Errorf("level = %q, want ERROR", got["__level"])
	}
}

// asynq's own diagnostics must reach slog rather than its default stderr
// logger, or a Redis outage in the worker poller is invisible to a JSON log
// pipeline (INV-0007 F7.5). An unreachable Redis drives that error path without
// disturbing the container shared by the other tests.
func TestAsynqInternalErrorsReachSlog(t *testing.T) {
	rec := captureDefaultLogs(t)

	// Port 1 is reserved and never listening, so the poller fails every tick.
	worker, err := queue.NewWorker("redis://127.0.0.1:1", 1, &countingIngestor{})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	if err := worker.Start(); err != nil {
		t.Fatalf("start worker: %v", err)
	}
	defer worker.Shutdown()

	if !waitFor(func() bool { return len(rec.findAttr("component", "asynq")) > 0 }, 15*time.Second) {
		t.Fatal("no component=asynq record: asynq is not logging through slog")
	}
}

// A repo must be ingestable more than once. asynq's task-id uniqueness check is
// a bare EXISTS spanning every state, so a finished task keeps owning the id:
// completed for the retention TTL after a success, archived for 90 days after
// retries are exhausted. Treating those conflicts as "already covered" silently
// discarded every later trigger — capping a repo at one ingest per retention
// window (INV-0007 F4 and the completed-state case found implementing it).
func TestReingestAfterSuccessfulRun(t *testing.T) {
	ing := &countingIngestor{}
	client, err := queue.NewClient(redisURL, 0)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			t.Logf("close client: %v", cerr)
		}
	}()

	worker, err := queue.NewWorker(redisURL, 1, ing)
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	if err := worker.Start(); err != nil {
		t.Fatalf("start worker: %v", err)
	}
	defer worker.Shutdown()

	first := &queue.IngestJob{InstallationID: 1, Owner: "acme", Name: "reingest", Reason: "onboard"}
	if err := client.EnqueueIngest(t.Context(), first); err != nil {
		t.Fatalf("enqueue first: %v", err)
	}
	if got := waitForCount(ing, 1, 8*time.Second); got != 1 {
		t.Fatalf("first ingest ran %d times, want 1", got)
	}

	// A later push for the same repo, after the first run finished.
	second := &queue.IngestJob{InstallationID: 1, Owner: "acme", Name: "reingest", Reason: "push"}
	if err := client.EnqueueIngest(t.Context(), second); err != nil {
		t.Fatalf("enqueue second: %v", err)
	}
	if got := waitForCount(ing, 2, 8*time.Second); got != 2 {
		t.Errorf("ingest ran %d times after a second trigger, want 2: the finished task's id swallowed it", got)
	}
}

// The same must hold after a run exhausts its retries and is archived: the next
// trigger has to clear the archived id rather than be discarded by it.
func TestReingestAfterArchivedRun(t *testing.T) {
	rec := captureDefaultLogs(t)
	failing := &failingIngestor{err: errors.New("permanent failure")}

	client, err := queue.NewClient(redisURL, 0)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			t.Logf("close client: %v", cerr)
		}
	}()

	worker, err := queue.NewWorker(redisURL, 1, failing)
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	if err := worker.Start(); err != nil {
		t.Fatalf("start worker: %v", err)
	}

	job := &queue.IngestJob{InstallationID: 2, Owner: "acme", Name: "archived", Reason: "push"}
	if err := client.EnqueueIngest(t.Context(), job); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if !waitFor(func() bool { return failing.attempts.Load() > 0 }, 10*time.Second) {
		t.Fatal("ingest never attempted")
	}
	// Stop the worker so the task stays put instead of being retried mid-assert.
	worker.Shutdown()

	// Archive the task directly: waiting out five retries of asynq's default
	// backoff would make this test minutes long.
	insp := asynq.NewInspector(asynq.RedisClientOpt{Addr: strings.TrimPrefix(redisURL, "redis://")})
	defer func() {
		if cerr := insp.Close(); cerr != nil {
			t.Logf("close inspector: %v", cerr)
		}
	}()
	taskID := "ingest:acme/archived"
	if err := insp.ArchiveTask("ingest", taskID); err != nil {
		t.Fatalf("archive task: %v", err)
	}
	info, err := insp.GetTaskInfo("ingest", taskID)
	if err != nil {
		t.Fatalf("get task info: %v", err)
	}
	if info.State != asynq.TaskStateArchived {
		t.Fatalf("task state = %v, want archived", info.State)
	}

	// The next trigger must clear the archived id rather than be swallowed by it.
	if err := client.EnqueueIngest(t.Context(), job); err != nil {
		t.Fatalf("enqueue after archive: %v", err)
	}
	after, err := insp.GetTaskInfo("ingest", taskID)
	if err != nil {
		t.Fatalf("get task info after re-enqueue: %v", err)
	}
	if after.State == asynq.TaskStateArchived {
		t.Error("task is still archived: the trigger was swallowed instead of clearing it")
	}
	if len(rec.find("cleared a finished ingest task and re-enqueued")) == 0 {
		t.Error("no log record explaining that the archived task was cleared")
	}
}
