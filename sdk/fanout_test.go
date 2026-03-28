package sdk_test

import (
	"context"
	"testing"
	"time"

	"grael/internal/api"
	rt "grael/internal/runtime"
	"grael/sdk"
)

func TestExpandFanOutBuildsOrdinarySpawnedNodes(t *testing.T) {
	t.Parallel()

	nodes, err := sdk.ExpandFanOut(sdk.FanOutSpec{
		IDPrefix:     "spec-writer",
		ActivityType: "draft",
		Items: []sdk.FanOutItem{
			{
				Input:             map[string]any{"question": "What broke?"},
				RetryPolicy:       &rt.RetryPolicy{MaxAttempts: 2, Backoff: 10 * time.Millisecond},
				ExecutionDeadline: 50 * time.Millisecond,
			},
			{
				ID:    "custom-node",
				Input: map[string]any{"question": "Who is impacted?"},
			},
		},
	})
	if err != nil {
		t.Fatalf("ExpandFanOut returned error: %v", err)
	}

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].ID != "spec-writer-1" {
		t.Fatalf("expected generated id spec-writer-1, got %s", nodes[0].ID)
	}
	if got := nodes[0].Input["question"]; got != "What broke?" {
		t.Fatalf("expected input to be preserved, got %v", nodes[0].Input)
	}
	if nodes[0].RetryPolicy == nil || nodes[0].RetryPolicy.MaxAttempts != 2 {
		t.Fatalf("expected retry policy to be copied, got %+v", nodes[0].RetryPolicy)
	}
	if nodes[1].ID != "custom-node" {
		t.Fatalf("expected explicit id to be preserved, got %s", nodes[1].ID)
	}
}

func TestExpandFanOutHelperMatchesManualSpawnSemantics(t *testing.T) {
	t.Parallel()

	helperView, helperEvents := runFanOutWorkflow(t, "helper")
	manualView, manualEvents := runFanOutWorkflow(t, "manual")

	if helperView.State != rt.RunStateCompleted || manualView.State != rt.RunStateCompleted {
		t.Fatalf("expected both runs completed, got helper=%s manual=%s", helperView.State, manualView.State)
	}
	for _, nodeID := range []string{"plan", "draft-1", "draft-2"} {
		if _, ok := helperView.Nodes[nodeID]; !ok {
			t.Fatalf("expected helper run to contain node %s", nodeID)
		}
		if _, ok := manualView.Nodes[nodeID]; !ok {
			t.Fatalf("expected manual run to contain node %s", nodeID)
		}
	}

	helperSpawn := spawnedNodesFromPlanCompletion(t, helperEvents)
	manualSpawn := spawnedNodesFromPlanCompletion(t, manualEvents)
	if len(helperSpawn) != len(manualSpawn) {
		t.Fatalf("expected same spawn width, got helper=%d manual=%d", len(helperSpawn), len(manualSpawn))
	}
	for i := range helperSpawn {
		if helperSpawn[i].ID != manualSpawn[i].ID {
			t.Fatalf("expected matching spawned node ids, got helper=%s manual=%s", helperSpawn[i].ID, manualSpawn[i].ID)
		}
		if helperSpawn[i].ActivityType != manualSpawn[i].ActivityType {
			t.Fatalf("expected matching activity types, got helper=%s manual=%s", helperSpawn[i].ActivityType, manualSpawn[i].ActivityType)
		}
		if helperSpawn[i].Input["question"] != manualSpawn[i].Input["question"] {
			t.Fatalf("expected matching node input, got helper=%v manual=%v", helperSpawn[i].Input, manualSpawn[i].Input)
		}
	}
}

func runFanOutWorkflow(t *testing.T, mode string) (rt.RunView, []rt.Event) {
	t.Helper()

	svc := api.New(t.TempDir())
	worker := sdk.NewWorker(sdk.NewServiceClient(svc), "sdk-fanout-"+mode)
	worker.SetPollTimeout(25 * time.Millisecond)

	worker.Handle(rt.ActivityType("plan-"+mode), func(ctx context.Context, task sdk.Task) (sdk.Result, error) {
		var spawned []rt.NodeDefinition
		var err error
		switch mode {
		case "helper":
			spawned, err = sdk.ExpandFanOut(sdk.FanOutSpec{
				IDPrefix:     "draft",
				ActivityType: "draft",
				Items: []sdk.FanOutItem{
					{Input: map[string]any{"question": "What changed?"}},
					{Input: map[string]any{"question": "Who is impacted?"}},
				},
			})
			if err != nil {
				return sdk.Result{}, err
			}
		case "manual":
			spawned = []rt.NodeDefinition{
				{ID: "draft-1", ActivityType: "draft", Input: map[string]any{"question": "What changed?"}},
				{ID: "draft-2", ActivityType: "draft", Input: map[string]any{"question": "Who is impacted?"}},
			}
		default:
			t.Fatalf("unknown mode %q", mode)
		}
		return sdk.Result{
			Output:       map[string]any{"status": "spawned"},
			SpawnedNodes: spawned,
		}, nil
	})
	worker.Handle("draft", func(ctx context.Context, task sdk.Task) (sdk.Result, error) {
		return sdk.Result{
			Output: map[string]any{"question": task.NodeInput["question"]},
		}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- worker.Run(ctx)
	}()

	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "sdk-fanout-" + mode,
		Nodes: []rt.NodeDefinition{
			{ID: "plan", ActivityType: rt.ActivityType("plan-" + mode)},
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
			t.Fatalf("worker returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for worker shutdown")
	}

	view, err := svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	events, err := svc.ListEvents(runID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	return view, events
}

func spawnedNodesFromPlanCompletion(t *testing.T, events []rt.Event) []rt.NodeDefinition {
	t.Helper()
	for _, event := range events {
		if event.Type != rt.EventNodeCompleted {
			continue
		}
		payload, ok := event.Payload.(rt.NodeCompletedPayload)
		if !ok || payload.NodeID != "plan" {
			continue
		}
		return payload.SpawnedNodes
	}
	t.Fatal("expected plan completion with spawned nodes")
	return nil
}
