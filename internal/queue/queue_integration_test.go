//go:build integration

// Package queue_test integration tests run the asynq queue against a real Redis
// (testcontainers): a job drains through the worker, a burst coalesces to one
// run at the latest HEAD, and shutdown drains an in-flight job without loss.
package queue_test

import (
	"context"
	"log"
	"os"
	"sync/atomic"
	"testing"
	"time"

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
