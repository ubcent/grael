package api_test

import (
	"errors"
	"testing"
	"time"

	"grael/internal/api"
	"grael/internal/engine"
	rt "grael/internal/runtime"
)

func TestWorkerPollCompleteFinishesRun(t *testing.T) {
	t.Parallel()

	svc := api.New(t.TempDir())
	if err := svc.RegisterWorker("worker-1", []rt.ActivityType{"hello"}); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "hello-run",
		Nodes: []rt.NodeDefinition{
			{ID: "A", ActivityType: "hello"},
		},
	}, map[string]any{"name": "grael"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	task, ok, err := svc.PollTask("worker-1", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("poll task: %v", err)
	}
	if !ok {
		t.Fatal("expected worker to receive a task")
	}
	if task.RunID != runID || task.NodeID != "A" || task.ActivityType != "hello" || task.Attempt != 1 {
		t.Fatalf("unexpected task: %+v", task)
	}

	if err := svc.CompleteTask(rt.CompleteTaskRequest{
		WorkerID: "worker-1",
		RunID:    task.RunID,
		NodeID:   task.NodeID,
		Attempt:  task.Attempt,
		Output:   map[string]any{"status": "ok"},
	}); err != nil {
		t.Fatalf("complete task: %v", err)
	}

	view, err := svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if view.State != rt.RunStateCompleted {
		t.Fatalf("expected completed run, got %s", view.State)
	}
	if got := view.Nodes["A"].State; got != rt.NodeStateCompleted {
		t.Fatalf("expected node A completed, got %s", got)
	}

	events, err := svc.ListEvents(runID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	assertEventSequence(t, events, []rt.EventType{
		rt.EventWorkflowStarted,
		rt.EventLeaseGranted,
		rt.EventNodeStarted,
		rt.EventNodeCompleted,
		rt.EventWorkflowCompleted,
	})
}

func TestLinearWorkflowRespectsDependenciesThroughWorkerSurface(t *testing.T) {
	t.Parallel()

	svc := api.New(t.TempDir())
	if err := svc.RegisterWorker("worker-1", []rt.ActivityType{"step"}); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "linear-steps",
		Nodes: []rt.NodeDefinition{
			{ID: "A", ActivityType: "step"},
			{ID: "B", ActivityType: "step", DependsOn: []string{"A"}},
			{ID: "C", ActivityType: "step", DependsOn: []string{"B"}},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	for _, nodeID := range []string{"A", "B", "C"} {
		task, ok, err := svc.PollTask("worker-1", 250*time.Millisecond)
		if err != nil {
			t.Fatalf("poll task for %s: %v", nodeID, err)
		}
		if !ok {
			t.Fatalf("expected task for node %s", nodeID)
		}
		if task.NodeID != nodeID {
			t.Fatalf("expected node %s, got %s", nodeID, task.NodeID)
		}
		if err := svc.CompleteTask(rt.CompleteTaskRequest{
			WorkerID: "worker-1",
			RunID:    task.RunID,
			NodeID:   task.NodeID,
			Attempt:  task.Attempt,
			Output:   map[string]any{"node": nodeID},
		}); err != nil {
			t.Fatalf("complete task %s: %v", nodeID, err)
		}
	}

	view, err := svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if view.State != rt.RunStateCompleted {
		t.Fatalf("expected completed run, got %s", view.State)
	}

	events, err := svc.ListEvents(runID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	assertEventSequence(t, events, []rt.EventType{
		rt.EventWorkflowStarted,
		rt.EventLeaseGranted,
		rt.EventNodeStarted,
		rt.EventNodeCompleted,
		rt.EventLeaseGranted,
		rt.EventNodeStarted,
		rt.EventNodeCompleted,
		rt.EventLeaseGranted,
		rt.EventNodeStarted,
		rt.EventNodeCompleted,
		rt.EventWorkflowCompleted,
	})
}

func TestGetRunRehydratesAttemptStateFromSnapshotAndWalDelta(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	svc := api.New(dir)
	if err := svc.RegisterWorker("worker-1", []rt.ActivityType{"hello"}); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "rehydrate-worker-run",
		Nodes: []rt.NodeDefinition{
			{ID: "A", ActivityType: "hello"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	task, ok, err := svc.PollTask("worker-1", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("poll task: %v", err)
	}
	if !ok {
		t.Fatal("expected task to be dispatched")
	}
	if err := svc.CompleteTask(rt.CompleteTaskRequest{
		WorkerID: "worker-1",
		RunID:    task.RunID,
		NodeID:   task.NodeID,
		Attempt:  task.Attempt,
		Output:   map[string]any{"status": "ok"},
	}); err != nil {
		t.Fatalf("complete task: %v", err)
	}

	svc = api.New(dir)
	view, err := svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run after recreate: %v", err)
	}
	if view.State != rt.RunStateCompleted {
		t.Fatalf("expected completed run after rehydrate, got %s", view.State)
	}
	if node := view.Nodes["A"]; node.Attempt != 1 || node.State != rt.NodeStateCompleted {
		t.Fatalf("unexpected rehydrated node view: %+v", node)
	}
}

func TestCompleteTaskRejectsMismatchedAttempt(t *testing.T) {
	t.Parallel()

	svc := api.New(t.TempDir())
	if err := svc.RegisterWorker("worker-1", []rt.ActivityType{"hello"}); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "attempt-check",
		Nodes: []rt.NodeDefinition{
			{ID: "A", ActivityType: "hello"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	task, ok, err := svc.PollTask("worker-1", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("poll task: %v", err)
	}
	if !ok {
		t.Fatal("expected task to be dispatched")
	}

	err = svc.CompleteTask(rt.CompleteTaskRequest{
		WorkerID: "worker-1",
		RunID:    runID,
		NodeID:   "A",
		Attempt:  task.Attempt + 1,
	})
	if err == nil {
		t.Fatal("expected mismatched attempt to be rejected")
	}

	view, err := svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got := view.Nodes["A"].State; got != rt.NodeStateRunning {
		t.Fatalf("expected node to remain running, got %s", got)
	}
}

func TestHeartbeatLossExpiresLeaseAndMakesNodeDispatchableAgain(t *testing.T) {
	t.Parallel()

	svc := api.New(t.TempDir())
	if err := svc.RegisterWorker("worker-1", []rt.ActivityType{"slow_step"}); err != nil {
		t.Fatalf("register worker-1: %v", err)
	}
	if err := svc.RegisterWorker("worker-2", []rt.ActivityType{"slow_step"}); err != nil {
		t.Fatalf("register worker-2: %v", err)
	}

	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "lease-expiry",
		Nodes: []rt.NodeDefinition{
			{ID: "A", ActivityType: "slow_step"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	firstTask, ok, err := svc.PollTask("worker-1", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("poll first task: %v", err)
	}
	if !ok {
		t.Fatal("expected first worker to receive a task")
	}

	waitForNodeState(t, svc, runID, "A", rt.NodeStateReady)

	secondTask, ok, err := svc.PollTask("worker-2", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("poll second task: %v", err)
	}
	if !ok {
		t.Fatal("expected redispatch after lease expiry")
	}
	if secondTask.NodeID != "A" || secondTask.Attempt != firstTask.Attempt+1 {
		t.Fatalf("expected redispatch of A on next attempt, got %+v", secondTask)
	}

	events, err := svc.ListEvents(runID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	assertEventTypesPresentInOrder(t, events, []rt.EventType{
		rt.EventWorkflowStarted,
		rt.EventLeaseGranted,
		rt.EventNodeStarted,
		rt.EventLeaseExpired,
		rt.EventLeaseGranted,
	})
}

func TestLateCompleteAfterLeaseExpiryIsRejected(t *testing.T) {
	t.Parallel()

	svc := api.New(t.TempDir())
	if err := svc.RegisterWorker("worker-1", []rt.ActivityType{"slow_step"}); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "late-complete",
		Nodes: []rt.NodeDefinition{
			{ID: "A", ActivityType: "slow_step"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	task, ok, err := svc.PollTask("worker-1", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("poll task: %v", err)
	}
	if !ok {
		t.Fatal("expected task to be dispatched")
	}

	waitForNodeState(t, svc, runID, "A", rt.NodeStateReady)

	err = svc.CompleteTask(rt.CompleteTaskRequest{
		WorkerID: "worker-1",
		RunID:    runID,
		NodeID:   "A",
		Attempt:  task.Attempt,
		Output:   map[string]any{"status": "too-late"},
	})
	if err == nil {
		t.Fatal("expected late completion to be rejected")
	}
	if !errors.Is(err, engine.ErrLeaseExpired) {
		t.Fatalf("expected lease expired error, got %v", err)
	}

	view, err := svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got := view.Nodes["A"].State; got != rt.NodeStateReady {
		t.Fatalf("expected node to remain ready after late completion, got %s", got)
	}

	events, err := svc.ListEvents(runID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	assertEventTypesPresentInOrder(t, events, []rt.EventType{
		rt.EventWorkflowStarted,
		rt.EventLeaseGranted,
		rt.EventNodeStarted,
		rt.EventLeaseExpired,
	})
	for _, event := range events {
		if event.Type == rt.EventNodeCompleted {
			t.Fatal("did not expect stale completion to be persisted")
		}
	}
}

func TestRetryableFailureSchedulesBackoffAndSucceedsOnNextAttempt(t *testing.T) {
	t.Parallel()

	svc := api.New(t.TempDir())
	if err := svc.RegisterWorker("worker-1", []rt.ActivityType{"flaky"}); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "retry-success",
		Nodes: []rt.NodeDefinition{
			{
				ID:           "A",
				ActivityType: "flaky",
				RetryPolicy: &rt.RetryPolicy{
					MaxAttempts: 2,
					Backoff:     80 * time.Millisecond,
				},
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	task, ok, err := svc.PollTask("worker-1", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("poll first task: %v", err)
	}
	if !ok {
		t.Fatal("expected first attempt")
	}
	if err := svc.FailTask(rt.FailTaskRequest{
		WorkerID:  "worker-1",
		RunID:     runID,
		NodeID:    "A",
		Attempt:   task.Attempt,
		Message:   "transient",
		Retryable: true,
	}); err != nil {
		t.Fatalf("fail retryable task: %v", err)
	}

	task, ok, err = svc.PollTask("worker-1", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("poll retry task: %v", err)
	}
	if !ok {
		t.Fatal("expected retry attempt")
	}
	if task.Attempt != 2 {
		t.Fatalf("expected retry attempt 2, got %d", task.Attempt)
	}
	if err := svc.CompleteTask(rt.CompleteTaskRequest{
		WorkerID: "worker-1",
		RunID:    runID,
		NodeID:   "A",
		Attempt:  task.Attempt,
		Output:   map[string]any{"status": "ok"},
	}); err != nil {
		t.Fatalf("complete retry task: %v", err)
	}

	view, err := svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if view.State != rt.RunStateCompleted {
		t.Fatalf("expected completed run, got %s", view.State)
	}

	events, err := svc.ListEvents(runID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	assertEventTypesPresentInOrder(t, events, []rt.EventType{
		rt.EventWorkflowStarted,
		rt.EventLeaseGranted,
		rt.EventNodeStarted,
		rt.EventNodeFailed,
		rt.EventTimerScheduled,
		rt.EventTimerFired,
		rt.EventLeaseGranted,
		rt.EventNodeStarted,
		rt.EventNodeCompleted,
		rt.EventWorkflowCompleted,
	})
}

func TestOverdueRetryTimerFiresAfterRestart(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	svc := api.New(dir)
	if err := svc.RegisterWorker("worker-1", []rt.ActivityType{"flaky"}); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "retry-after-restart",
		Nodes: []rt.NodeDefinition{
			{
				ID:           "A",
				ActivityType: "flaky",
				RetryPolicy: &rt.RetryPolicy{
					MaxAttempts: 2,
					Backoff:     80 * time.Millisecond,
				},
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	task, ok, err := svc.PollTask("worker-1", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("poll first task: %v", err)
	}
	if !ok {
		t.Fatal("expected first attempt")
	}
	if err := svc.FailTask(rt.FailTaskRequest{
		WorkerID:  "worker-1",
		RunID:     runID,
		NodeID:    "A",
		Attempt:   task.Attempt,
		Message:   "transient",
		Retryable: true,
	}); err != nil {
		t.Fatalf("fail retryable task: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	svc = api.New(dir)
	if err := svc.RegisterWorker("worker-1", []rt.ActivityType{"flaky"}); err != nil {
		t.Fatalf("register worker after restart: %v", err)
	}

	task, ok, err = svc.PollTask("worker-1", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("poll retried task after restart: %v", err)
	}
	if !ok {
		t.Fatal("expected overdue retry after restart")
	}
	if task.Attempt != 2 {
		t.Fatalf("expected attempt 2 after restart, got %d", task.Attempt)
	}
	if err := svc.CompleteTask(rt.CompleteTaskRequest{
		WorkerID: "worker-1",
		RunID:    runID,
		NodeID:   "A",
		Attempt:  task.Attempt,
		Output:   map[string]any{"status": "ok"},
	}); err != nil {
		t.Fatalf("complete retry task after restart: %v", err)
	}

	events, err := svc.ListEvents(runID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	assertEventTypesPresentInOrder(t, events, []rt.EventType{
		rt.EventWorkflowStarted,
		rt.EventLeaseGranted,
		rt.EventNodeStarted,
		rt.EventNodeFailed,
		rt.EventTimerScheduled,
		rt.EventTimerFired,
		rt.EventLeaseGranted,
		rt.EventNodeStarted,
		rt.EventNodeCompleted,
		rt.EventWorkflowCompleted,
	})
}

func TestExecutionDeadlineTurnsRunningNodeIntoTimeoutFailure(t *testing.T) {
	t.Parallel()

	svc := api.New(t.TempDir())
	if err := svc.RegisterWorker("worker-1", []rt.ActivityType{"hanging_step"}); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "deadline-timeout",
		Nodes: []rt.NodeDefinition{
			{
				ID:                "A",
				ActivityType:      "hanging_step",
				ExecutionDeadline: 80 * time.Millisecond,
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	task, ok, err := svc.PollTask("worker-1", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("poll task: %v", err)
	}
	if !ok {
		t.Fatal("expected deadline-bound task to be dispatched")
	}
	if task.Attempt != 1 {
		t.Fatalf("expected first attempt, got %d", task.Attempt)
	}

	waitForNodeState(t, svc, runID, "A", rt.NodeStateFailed)

	view, err := svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if view.State != rt.RunStateFailed {
		t.Fatalf("expected failed run after timeout, got %s", view.State)
	}
	if node := view.Nodes["A"]; node.LastError != "execution deadline exceeded" {
		t.Fatalf("expected timeout error, got %+v", node)
	}

	events, err := svc.ListEvents(runID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	assertEventTypesPresentInOrder(t, events, []rt.EventType{
		rt.EventWorkflowStarted,
		rt.EventLeaseGranted,
		rt.EventNodeStarted,
		rt.EventTimerScheduled,
		rt.EventTimerFired,
		rt.EventNodeFailed,
		rt.EventWorkflowFailed,
	})
}

func TestNonRetryableFailureFailsWorkflow(t *testing.T) {
	t.Parallel()

	svc := api.New(t.TempDir())
	if err := svc.RegisterWorker("worker-1", []rt.ActivityType{"step"}); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "terminal-failure",
		Nodes: []rt.NodeDefinition{
			{ID: "A", ActivityType: "step"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	task, ok, err := svc.PollTask("worker-1", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("poll task: %v", err)
	}
	if !ok {
		t.Fatal("expected task")
	}
	if err := svc.FailTask(rt.FailTaskRequest{
		WorkerID: "worker-1",
		RunID:    runID,
		NodeID:   task.NodeID,
		Attempt:  task.Attempt,
		Message:  "boom",
	}); err != nil {
		t.Fatalf("fail task: %v", err)
	}

	view, err := svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if view.State != rt.RunStateFailed {
		t.Fatalf("expected failed run, got %s", view.State)
	}

	events, err := svc.ListEvents(runID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	assertEventTypesPresentInOrder(t, events, []rt.EventType{
		rt.EventWorkflowStarted,
		rt.EventLeaseGranted,
		rt.EventNodeStarted,
		rt.EventNodeFailed,
		rt.EventWorkflowFailed,
	})
}

func TestActiveLeaseDoesNotExpireImmediatelyAfterRestart(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	svc := api.New(dir)
	if err := svc.RegisterWorker("worker-1", []rt.ActivityType{"slow_step"}); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "lease-restart",
		Nodes: []rt.NodeDefinition{
			{ID: "A", ActivityType: "slow_step"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	task, ok, err := svc.PollTask("worker-1", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("poll task: %v", err)
	}
	if !ok {
		t.Fatal("expected task")
	}

	time.Sleep(50 * time.Millisecond)
	svc = api.New(dir)

	view, err := svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run after restart: %v", err)
	}
	if got := view.Nodes["A"].State; got != rt.NodeStateRunning {
		t.Fatalf("expected node to remain running immediately after restart, got %s", got)
	}

	time.Sleep(175 * time.Millisecond)
	waitForNodeState(t, svc, runID, "A", rt.NodeStateReady)

	err = svc.CompleteTask(rt.CompleteTaskRequest{
		WorkerID: "worker-1",
		RunID:    runID,
		NodeID:   "A",
		Attempt:  task.Attempt,
		Output:   map[string]any{"status": "late"},
	})
	if !errors.Is(err, engine.ErrLeaseExpired) {
		t.Fatalf("expected lease expired after timeout window, got %v", err)
	}
}

func TestAbsoluteDeadlineFailsRunningNode(t *testing.T) {
	t.Parallel()

	svc := api.New(t.TempDir())
	if err := svc.RegisterWorker("worker-1", []rt.ActivityType{"hanging_step"}); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "absolute-deadline",
		Nodes: []rt.NodeDefinition{
			{
				ID:               "A",
				ActivityType:     "hanging_step",
				AbsoluteDeadline: 80 * time.Millisecond,
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	if _, ok, err := svc.PollTask("worker-1", 250*time.Millisecond); err != nil {
		t.Fatalf("poll task: %v", err)
	} else if !ok {
		t.Fatal("expected task")
	}

	waitForNodeState(t, svc, runID, "A", rt.NodeStateFailed)

	view, err := svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if node := view.Nodes["A"]; node.LastError != "absolute deadline exceeded" {
		t.Fatalf("expected absolute deadline error, got %+v", node)
	}

	events, err := svc.ListEvents(runID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	assertEventTypesPresentInOrder(t, events, []rt.EventType{
		rt.EventWorkflowStarted,
		rt.EventLeaseGranted,
		rt.EventNodeStarted,
		rt.EventTimerScheduled,
		rt.EventTimerFired,
		rt.EventNodeFailed,
		rt.EventWorkflowFailed,
	})
}

func assertEventSequence(t *testing.T, events []rt.Event, want []rt.EventType) {
	t.Helper()

	if len(events) != len(want) {
		t.Fatalf("expected %d events, got %d", len(want), len(events))
	}
	for i, expected := range want {
		if events[i].Type != expected {
			t.Fatalf("expected event %d to be %s, got %s", i, expected, events[i].Type)
		}
	}
}

func assertEventTypesPresentInOrder(t *testing.T, events []rt.Event, want []rt.EventType) {
	t.Helper()

	index := 0
	for _, event := range events {
		if index >= len(want) {
			return
		}
		if event.Type == want[index] {
			index++
		}
	}
	if index != len(want) {
		t.Fatalf("expected event subsequence %v, got %v", want, eventTypes(events))
	}
}

func eventTypes(events []rt.Event) []rt.EventType {
	types := make([]rt.EventType, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}

func waitForNodeState(t *testing.T, svc *api.Service, runID, nodeID string, want rt.NodeState) {
	t.Helper()

	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		view, err := svc.GetRun(runID)
		if err != nil {
			t.Fatalf("get run: %v", err)
		}
		if view.Nodes[nodeID].State == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	view, err := svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	t.Fatalf("expected node %s to reach state %s, got %s", nodeID, want, view.Nodes[nodeID].State)
}
