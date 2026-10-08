package queue

import (
	"errors"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
)

// The export twins of client_test.go: the conflict path is shared, so each
// case must hold for an export job on the export queue as it does for an
// ingest job on the ingest queue.

func TestExportTaskIDIsStablePerRepo(t *testing.T) {
	a := exportTaskID(&ExportJob{RepoID: 1, Owner: "acme", Name: "docs", Reason: "push"})
	b := exportTaskID(&ExportJob{RepoID: 1, Owner: "acme", Name: "docs", Reason: "manual"})
	if a != b {
		t.Errorf("task id varies by reason: %q vs %q", a, b)
	}
	if want := "export:acme/docs"; a != want {
		t.Errorf("task id = %q, want %q", a, want)
	}
}

func TestExportJobSpec(t *testing.T) {
	sp := (&ExportJob{}).spec()
	if sp.taskType != TaskTypeExport || sp.queue != exportQueueName || sp.kind != "export" {
		t.Errorf("spec = %+v", sp)
	}
	if got := len((&ExportJob{}).options(&Client{})); got != 1 {
		t.Errorf("export options = %d, want MaxRetry only (no ProcessIn)", got)
	}
}

func TestResolveExportConflictDeletesTerminalTaskFromExportQueue(t *testing.T) {
	insp := &fakeInspector{
		info:      &asynq.TaskInfo{State: asynq.TaskStateCompleted},
		deleteErr: errors.New("stop here"),
	}
	c := &Client{asynq: deadAsynqClient(t), inspector: insp}
	job := &ExportJob{RepoID: 1, Owner: "acme", Name: "docs", Reason: "push"}

	err := c.resolveTaskIDConflict(t.Context(), job, []byte("{}"), exportTaskID(job))
	if err == nil || !strings.Contains(err.Error(), "clear finished export task") {
		t.Errorf("err = %v, want it to abort naming the export clear step", err)
	}
	if insp.deletedQueue != exportQueueName {
		t.Errorf("deleted from queue %q, want %q", insp.deletedQueue, exportQueueName)
	}
	if want := "export:acme/docs"; insp.deletedID != want {
		t.Errorf("deleted task id %q, want %q", insp.deletedID, want)
	}
}

func TestResolveExportConflictToleratesAlreadyDeletedTask(t *testing.T) {
	insp := &fakeInspector{
		info:      &asynq.TaskInfo{State: asynq.TaskStateArchived},
		deleteErr: asynq.ErrTaskNotFound,
	}
	c := &Client{asynq: deadAsynqClient(t), inspector: insp}
	job := &ExportJob{RepoID: 1, Owner: "acme", Name: "docs", Reason: "push"}

	err := c.resolveTaskIDConflict(t.Context(), job, []byte("{}"), exportTaskID(job))
	if err == nil || !strings.Contains(err.Error(), "re-enqueue export") {
		t.Errorf("err = %v, want it to reach the export re-enqueue", err)
	}
}

func TestResolveExportConflictCoalescesALiveTask(t *testing.T) {
	for _, state := range []asynq.TaskState{
		asynq.TaskStatePending, asynq.TaskStateActive, asynq.TaskStateRetry, asynq.TaskStateScheduled,
	} {
		t.Run(state.String(), func(t *testing.T) {
			insp := &fakeInspector{info: &asynq.TaskInfo{State: state}}
			c := &Client{asynq: deadAsynqClient(t), inspector: insp}
			job := &ExportJob{RepoID: 1, Owner: "acme", Name: "docs"}

			if err := c.resolveTaskIDConflict(t.Context(), job, []byte("{}"), exportTaskID(job)); err != nil {
				t.Errorf("resolveTaskIDConflict = %v, want nil (coalesced)", err)
			}
			if insp.deleteCalls != 0 {
				t.Errorf("DeleteTask calls = %d, want 0", insp.deleteCalls)
			}
		})
	}
}

func TestResolveExportConflictSurvivesNilTaskInfo(t *testing.T) {
	c := &Client{asynq: deadAsynqClient(t), inspector: &fakeInspector{}}
	job := &ExportJob{RepoID: 1, Owner: "acme", Name: "docs"}

	if err := c.resolveTaskIDConflict(t.Context(), job, []byte("{}"), exportTaskID(job)); err != nil {
		t.Errorf("resolveTaskIDConflict = %v, want nil (treated as coalesced)", err)
	}
}
