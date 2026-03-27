package sdk_test

import (
	"context"
	"testing"
	"time"

	"grael/internal/api"
	rt "grael/internal/runtime"
	"grael/sdk"
)

func TestGoWorkerSDKSeamHandlesTaskSuccessfully(t *testing.T) {
	t.Parallel()

	svc := api.New(t.TempDir())
	worker := sdk.NewWorker(sdk.NewServiceClient(svc), "sdk-worker")
	worker.SetPollTimeout(25 * time.Millisecond)
	worker.Handle("hello", func(ctx context.Context, task sdk.Task) (sdk.Result, error) {
		return sdk.Result{
			Output: map[string]any{
				"handled_by": "sdk",
				"node_id":    task.NodeID,
			},
		}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- worker.Run(ctx)
	}()

	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "sdk-run",
		Nodes: []rt.NodeDefinition{
			{ID: "A", ActivityType: "hello"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	waitForRunState(t, svc, runID, rt.RunStateCompleted)

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("worker run returned error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for sdk worker to stop")
	}

	view, err := svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got := view.Nodes["A"].State; got != rt.NodeStateCompleted {
		t.Fatalf("expected node A completed, got %s", got)
	}

	events, err := svc.ListEvents(runID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) < 5 {
		t.Fatalf("expected standard worker event history, got %d events", len(events))
	}
}

func TestGoWorkerSDKShutdownDoesNotReportContextCancelAsTaskFailure(t *testing.T) {
	t.Parallel()

	svc := api.New(t.TempDir())
	worker := sdk.NewWorker(sdk.NewServiceClient(svc), "sdk-worker-shutdown")
	worker.SetPollTimeout(25 * time.Millisecond)

	taskStarted := make(chan struct{})
	releaseTask := make(chan struct{})
	worker.Handle("hello", func(ctx context.Context, task sdk.Task) (sdk.Result, error) {
		close(taskStarted)
		<-ctx.Done()
		<-releaseTask
		return sdk.Result{}, ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- worker.Run(ctx)
	}()

	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "sdk-run-shutdown",
		Nodes: []rt.NodeDefinition{
			{ID: "A", ActivityType: "hello"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	select {
	case <-taskStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for task to start")
	}

	cancel()
	close(releaseTask)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("worker run returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for worker shutdown")
	}

	events, err := svc.ListEvents(runID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	for _, event := range events {
		if event.Type == rt.EventNodeFailed {
			t.Fatalf("expected shutdown path to avoid synthetic node failure, got %v", events)
		}
	}
}

func waitForRunState(t *testing.T, svc *api.Service, runID string, want rt.RunState) {
	t.Helper()

	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		view, err := svc.GetRun(runID)
		if err != nil {
			t.Fatalf("get run: %v", err)
		}
		if view.State == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	view, err := svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	t.Fatalf("expected run %s to reach state %s, got %s", runID, want, view.State)
}
