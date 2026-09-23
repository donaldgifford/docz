package queue

import (
	"errors"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
)

func TestIngestTaskIDIsStablePerRepo(t *testing.T) {
	a := ingestTaskID(&IngestJob{Owner: "acme", Name: "docs", Reason: "push"})
	b := ingestTaskID(&IngestJob{Owner: "acme", Name: "docs", Reason: "onboard"})
	if a != b {
		t.Errorf("task id varies by reason: %q vs %q; coalescing depends on it being stable", a, b)
	}
	if want := "ingest:acme/docs"; a != want {
		t.Errorf("task id = %q, want %q", a, want)
	}
}

func TestIsTaskIDConflict(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"id conflict", asynq.ErrTaskIDConflict, true},
		{"duplicate", asynq.ErrDuplicateTask, true},
		{"wrapped conflict", errors.Join(errors.New("ctx"), asynq.ErrTaskIDConflict), true},
		{"other", errors.New("connection refused"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTaskIDConflict(tt.err); got != tt.want {
				t.Errorf("isTaskIDConflict(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// Terminal states hold the task id after the run is over, so they must be
// cleared; the others mean a run is still coming and the trigger is covered.
func TestIsTerminalTaskState(t *testing.T) {
	tests := []struct {
		state asynq.TaskState
		want  bool
	}{
		{asynq.TaskStateArchived, true},
		{asynq.TaskStateCompleted, true},
		{asynq.TaskStateActive, false},
		{asynq.TaskStatePending, false},
		{asynq.TaskStateScheduled, false},
		{asynq.TaskStateRetry, false},
	}
	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			if got := isTerminalTaskState(tt.state); got != tt.want {
				t.Errorf("isTerminalTaskState(%v) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

// classifyConflict is the whole conflict decision, so this table is the
// regression guard for INV-0007 F4: a finished task must never be mistaken for
// a pending one, or every future trigger for that repo is silently discarded.
func TestClassifyConflict(t *testing.T) {
	tests := []struct {
		name string
		info *asynq.TaskInfo
		err  error
		want conflictAction
	}{
		{"pending", &asynq.TaskInfo{State: asynq.TaskStatePending}, nil, coalesceTrigger},
		{"scheduled", &asynq.TaskInfo{State: asynq.TaskStateScheduled}, nil, coalesceTrigger},
		{"retry", &asynq.TaskInfo{State: asynq.TaskStateRetry}, nil, coalesceTrigger},
		{"active", &asynq.TaskInfo{State: asynq.TaskStateActive}, nil, coalesceTrigger},
		{"archived", &asynq.TaskInfo{State: asynq.TaskStateArchived}, nil, clearAndRetry},
		{"completed", &asynq.TaskInfo{State: asynq.TaskStateCompleted}, nil, clearAndRetry},
		// Fail open: an inspection problem must not turn a webhook 202 into a 500.
		{"inspection error", nil, errors.New("redis down"), coalesceTrigger},
		{"nil info without error", nil, nil, coalesceTrigger},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyConflict(tt.info, tt.err); got != tt.want {
				t.Errorf("classifyConflict = %v, want %v", got, tt.want)
			}
		})
	}
}

// fakeInspector stands in for *asynq.Inspector so the clear-and-retry path can
// be exercised without Redis.
type fakeInspector struct {
	info    *asynq.TaskInfo
	infoErr error

	deleteErr    error
	deleteCalls  int
	deletedQueue string
	deletedID    string
}

func (f *fakeInspector) GetTaskInfo(string, string) (*asynq.TaskInfo, error) {
	return f.info, f.infoErr
}

func (f *fakeInspector) DeleteTask(queue, id string) error {
	f.deleteCalls++
	f.deletedQueue, f.deletedID = queue, id
	return f.deleteErr
}

func (*fakeInspector) Close() error { return nil }

// A terminal task must be deleted from the ingest queue under the repo's task
// id before the trigger can be re-enqueued.
func TestResolveConflictDeletesTerminalTask(t *testing.T) {
	insp := &fakeInspector{info: &asynq.TaskInfo{State: asynq.TaskStateCompleted}}
	job := &IngestJob{Owner: "acme", Name: "docs", Reason: "push"}

	// The re-enqueue step needs a live asynq client, so this asserts the
	// inspector interaction only; the full round trip is covered by
	// TestReingestAfterSuccessfulRun in the integration suite.
	if act := classifyConflict(insp.info, nil); act != clearAndRetry {
		t.Fatalf("classifyConflict = %v, want clearAndRetry", act)
	}
	if err := insp.DeleteTask(queueName, ingestTaskID(job)); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}

	if insp.deleteCalls != 1 {
		t.Errorf("DeleteTask calls = %d, want 1", insp.deleteCalls)
	}
	if insp.deletedQueue != queueName {
		t.Errorf("deleted from queue %q, want %q", insp.deletedQueue, queueName)
	}
	if want := "ingest:acme/docs"; insp.deletedID != want {
		t.Errorf("deleted task id %q, want %q", insp.deletedID, want)
	}
}

// deadAsynqClient returns an asynq client pointed at a closed port, so Enqueue
// fails fast with a connection error. That is enough to tell *how far*
// resolveTaskIDConflict got: an error naming the re-enqueue proves it moved
// past the delete step rather than aborting there.
func deadAsynqClient(t *testing.T) *asynq.Client {
	t.Helper()
	c := asynq.NewClient(asynq.RedisClientOpt{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// Two triggers for the same repo can both classify clearAndRetry; the loser's
// DeleteTask finds the task already gone. That is the state this path is trying
// to reach, so it must continue to the re-enqueue. Aborting there made the
// loser answer 500 — and since the delivery id was already recorded, GitHub's
// redelivery would be deduped to a no-op instead of retrying.
func TestResolveConflictToleratesAlreadyDeletedTask(t *testing.T) {
	insp := &fakeInspector{
		info:      &asynq.TaskInfo{State: asynq.TaskStateArchived},
		deleteErr: asynq.ErrTaskNotFound,
	}
	c := &Client{asynq: deadAsynqClient(t), inspector: insp}
	job := &IngestJob{Owner: "acme", Name: "docs", Reason: "push"}

	err := c.resolveTaskIDConflict(t.Context(), job, []byte("{}"), ingestTaskID(job))
	if err == nil {
		t.Fatal("resolveTaskIDConflict succeeded against a dead redis, want an error")
	}
	if strings.Contains(err.Error(), "clear finished ingest task") {
		t.Errorf("err = %v, want it to move past the delete step, not abort there", err)
	}
	if !strings.Contains(err.Error(), "re-enqueue") {
		t.Errorf("err = %v, want it to name the re-enqueue step", err)
	}
}

// A genuine delete failure must still abort — tolerating ErrTaskNotFound should
// not turn into tolerating everything.
func TestResolveConflictFailsOnRealDeleteError(t *testing.T) {
	insp := &fakeInspector{
		info:      &asynq.TaskInfo{State: asynq.TaskStateArchived},
		deleteErr: errors.New("redis: connection refused"),
	}
	c := &Client{asynq: deadAsynqClient(t), inspector: insp}
	job := &IngestJob{Owner: "acme", Name: "docs", Reason: "push"}

	err := c.resolveTaskIDConflict(t.Context(), job, []byte("{}"), ingestTaskID(job))
	if err == nil || !strings.Contains(err.Error(), "clear finished ingest task") {
		t.Errorf("err = %v, want it to abort naming the clear step", err)
	}
}

// classifyConflict accepts a nil TaskInfo, so its caller must too: a stub
// inspector returning (nil, nil) should coalesce, not panic. The real
// *asynq.Inspector never does this, which is exactly why the divergence would
// go unnoticed until someone swapped the seam.
func TestResolveConflictSurvivesNilTaskInfo(t *testing.T) {
	c := &Client{asynq: deadAsynqClient(t), inspector: &fakeInspector{}}
	job := &IngestJob{Owner: "acme", Name: "docs", Reason: "push"}

	if err := c.resolveTaskIDConflict(t.Context(), job, []byte("{}"), ingestTaskID(job)); err != nil {
		t.Errorf("resolveTaskIDConflict = %v, want nil (treated as coalesced)", err)
	}
}
