package api_test

import (
	"testing"
	"time"

	"grael/internal/api"
	rt "grael/internal/runtime"
)

func TestStartRunGetRunAndListEvents(t *testing.T) {
	t.Parallel()

	svc := api.New(t.TempDir())
	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "linear-noop",
		Nodes: []rt.NodeDefinition{
			{ID: "A", ActivityType: rt.ActivityTypeNoop},
			{ID: "B", ActivityType: rt.ActivityTypeNoop, DependsOn: []string{"A"}},
			{ID: "C", ActivityType: rt.ActivityTypeNoop, DependsOn: []string{"B"}},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	view := eventuallyGetCompletedRun(t, svc, runID)

	events, err := svc.ListEvents(runID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected non-empty event history")
	}
	if events[0].Type != rt.EventWorkflowStarted {
		t.Fatalf("expected first event WorkflowStarted, got %s", events[0].Type)
	}
	if events[len(events)-1].Type != rt.EventWorkflowCompleted {
		t.Fatalf("expected last event WorkflowCompleted, got %s", events[len(events)-1].Type)
	}
	if view.LastSeq == 0 {
		t.Fatal("expected completed view to include applied events")
	}
}

func TestGetRunCanObserveActiveNonTerminalRun(t *testing.T) {
	t.Parallel()

	svc := api.New(t.TempDir())
	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "hold-run",
		Nodes: []rt.NodeDefinition{
			{ID: "A", ActivityType: rt.ActivityTypeHold},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		view, err := svc.GetRun(runID)
		if err != nil {
			t.Fatalf("get run: %v", err)
		}
		if view.State == rt.RunStateRunning && view.Nodes["A"].State == rt.NodeStateRunning {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	view, err := svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	t.Fatalf("expected active running node, got run=%s node=%s", view.State, view.Nodes["A"].State)
}

func TestGetRunRehydratesFromSnapshotAndWalDelta(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	svc := api.New(dir)
	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "linear-noop",
		Nodes: []rt.NodeDefinition{
			{ID: "A", ActivityType: rt.ActivityTypeNoop},
			{ID: "B", ActivityType: rt.ActivityTypeNoop, DependsOn: []string{"A"}},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	view := eventuallyGetCompletedRunByIDs(t, svc, runID, []string{"A", "B"})
	if view.State != rt.RunStateCompleted {
		t.Fatalf("expected completed run, got %s", view.State)
	}

	// Simulate a fresh process by recreating the service against the same data dir.
	svc = api.New(dir)
	view, err = svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run after recreate: %v", err)
	}
	if view.State != rt.RunStateCompleted {
		t.Fatalf("expected completed run after rehydrate, got %s", view.State)
	}
	for _, nodeID := range []string{"A", "B"} {
		if got := view.Nodes[nodeID].State; got != rt.NodeStateCompleted {
			t.Fatalf("expected node %s completed after rehydrate, got %s", nodeID, got)
		}
	}
}

func eventuallyGetCompletedRun(t *testing.T, svc *api.Service, runID string) rt.RunView {
	t.Helper()
	return eventuallyGetCompletedRunByIDs(t, svc, runID, []string{"A", "B", "C"})
}

func eventuallyGetCompletedRunByIDs(t *testing.T, svc *api.Service, runID string, nodeIDs []string) rt.RunView {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		view, err := svc.GetRun(runID)
		if err != nil {
			t.Fatalf("get run: %v", err)
		}
		if view.State == rt.RunStateCompleted {
			for _, nodeID := range nodeIDs {
				if got := view.Nodes[nodeID].State; got != rt.NodeStateCompleted {
					t.Fatalf("expected node %s completed, got %s", nodeID, got)
				}
			}
			return view
		}
		time.Sleep(10 * time.Millisecond)
	}

	view, err := svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	t.Fatalf("expected completed run, got %s", view.State)
	return rt.RunView{}
}
