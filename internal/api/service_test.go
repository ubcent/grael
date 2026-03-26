package api_test

import (
	"testing"
	"time"

	"grael/internal/api"
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
