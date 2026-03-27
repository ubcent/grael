package adapter

import (
	"testing"
	"time"

	"grael/internal/api"
	rt "grael/internal/runtime"
)

func TestSnapshotProjectsGraphAndTimelineFromReadSurfaces(t *testing.T) {
	t.Parallel()

	svc := api.New(t.TempDir())
	defer svc.Close()

	if err := svc.RegisterWorker("worker-1", []rt.ActivityType{"discover", "analyze", "review", "summarize"}); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "demo-adapter",
		Nodes: []rt.NodeDefinition{
			{ID: "discover", ActivityType: "discover"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	task, ok, err := svc.PollTask("worker-1", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("poll discover: %v", err)
	}
	if !ok || task.NodeID != "discover" {
		t.Fatalf("expected discover task, got %+v ok=%v", task, ok)
	}
	if err := svc.CompleteTask(rt.CompleteTaskRequest{
		WorkerID: "worker-1",
		RunID:    runID,
		NodeID:   task.NodeID,
		Attempt:  task.Attempt,
		SpawnedNodes: []rt.NodeDefinition{
			{ID: "analyze-1", ActivityType: "analyze"},
			{ID: "review", ActivityType: "review"},
			{ID: "summary", ActivityType: "summarize", DependsOn: []string{"analyze-1", "review"}},
		},
	}); err != nil {
		t.Fatalf("complete discover with spawn: %v", err)
	}

	task, ok, err = svc.PollTask("worker-1", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("poll analyze-1: %v", err)
	}
	if !ok || task.NodeID != "analyze-1" {
		t.Fatalf("expected analyze-1 task, got %+v ok=%v", task, ok)
	}
	if err := svc.CompleteTask(rt.CompleteTaskRequest{
		WorkerID: "worker-1",
		RunID:    runID,
		NodeID:   task.NodeID,
		Attempt:  task.Attempt,
	}); err != nil {
		t.Fatalf("complete analyze-1: %v", err)
	}

	task, ok, err = svc.PollTask("worker-1", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("poll review: %v", err)
	}
	if !ok || task.NodeID != "review" {
		t.Fatalf("expected review task, got %+v ok=%v", task, ok)
	}
	if err := svc.CompleteTask(rt.CompleteTaskRequest{
		WorkerID: "worker-1",
		RunID:    runID,
		NodeID:   task.NodeID,
		Attempt:  task.Attempt,
		Checkpoint: &rt.CheckpointRequest{
			Reason: "manual review",
		},
	}); err != nil {
		t.Fatalf("checkpoint review: %v", err)
	}

	model, err := New(svc).Snapshot(runID, 0)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if model.Run.RunID != runID {
		t.Fatalf("expected run id %s, got %s", runID, model.Run.RunID)
	}
	if model.Cursor.CurrentSeq != model.Run.LastSeq {
		t.Fatalf("expected cursor to match last seq, got %d vs %d", model.Cursor.CurrentSeq, model.Run.LastSeq)
	}
	if !model.Cursor.HasChanges {
		t.Fatal("expected initial snapshot to report changes")
	}
	if len(model.Graph.Nodes) != 4 {
		t.Fatalf("expected 4 graph nodes, got %d", len(model.Graph.Nodes))
	}
	if len(model.Graph.Edges) != 4 {
		t.Fatalf("expected 4 graph edges, got %d", len(model.Graph.Edges))
	}
	assertGraphNode(t, model.Graph.Nodes, "discover", rt.NodeStateCompleted)
	assertGraphNode(t, model.Graph.Nodes, "review", rt.NodeStateAwaitingApproval)
	assertGraphNodeCreated(t, model.Graph.Nodes, "analyze-1")
	assertGraphNodeSignal(t, model.Graph.Nodes, "review", "checkpoint")
	assertGraphNodeSignal(t, model.Graph.Nodes, "analyze-1", "spawned")
	assertGraphEdge(t, model.Graph.Edges, "discover", "analyze-1", "spawn")
	assertGraphEdge(t, model.Graph.Edges, "analyze-1", "summary", "dependency")
	assertNoGraphEdge(t, model.Graph.Edges, "discover", "summary", "spawn")
	assertTimelineFamily(t, model.Timeline.Events, "checkpoint")
	assertPhaseMarker(t, model.Phases, "spawn", true)
	assertPhaseMarker(t, model.Phases, "checkpoint", false)
}

func TestSnapshotCursorReturnsOnlyDeltaAfterKnownSeq(t *testing.T) {
	t.Parallel()

	svc := api.New(t.TempDir())
	defer svc.Close()

	if err := svc.RegisterWorker("worker-1", []rt.ActivityType{"step"}); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "cursor-demo",
		Nodes: []rt.NodeDefinition{
			{ID: "A", ActivityType: "step"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	first, err := New(svc).Snapshot(runID, 0)
	if err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	if len(first.Delta) != len(first.Timeline.Events) {
		t.Fatalf("expected first delta to contain full timeline, got %d vs %d", len(first.Delta), len(first.Timeline.Events))
	}

	task, ok, err := svc.PollTask("worker-1", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("poll A: %v", err)
	}
	if !ok {
		t.Fatal("expected task A")
	}
	if err := svc.CompleteTask(rt.CompleteTaskRequest{
		WorkerID: "worker-1",
		RunID:    runID,
		NodeID:   task.NodeID,
		Attempt:  task.Attempt,
	}); err != nil {
		t.Fatalf("complete A: %v", err)
	}

	second, err := New(svc).Snapshot(runID, first.Cursor.CurrentSeq)
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	if !second.Cursor.HasChanges {
		t.Fatal("expected second snapshot to report changes")
	}
	if len(second.Delta) == 0 {
		t.Fatal("expected non-empty delta after completion")
	}
	for _, event := range second.Delta {
		if event.Seq <= first.Cursor.CurrentSeq {
			t.Fatalf("expected delta events after seq %d, got event seq %d", first.Cursor.CurrentSeq, event.Seq)
		}
	}

	third, err := New(svc).Snapshot(runID, second.Cursor.CurrentSeq)
	if err != nil {
		t.Fatalf("third snapshot: %v", err)
	}
	if third.Cursor.HasChanges {
		t.Fatal("expected no changes when using current cursor")
	}
	if len(third.Delta) != 0 {
		t.Fatalf("expected empty delta at current cursor, got %d events", len(third.Delta))
	}
}

func TestSnapshotIncludesReplayFramesDerivedFromPersistedHistory(t *testing.T) {
	t.Parallel()

	svc := api.New(t.TempDir())
	defer svc.Close()

	if err := svc.RegisterWorker("worker-1", []rt.ActivityType{"step"}); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	runID, err := svc.StartRun(rt.WorkflowDefinition{
		Name: "replay-demo",
		Nodes: []rt.NodeDefinition{
			{ID: "A", ActivityType: "step"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	task, ok, err := svc.PollTask("worker-1", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("poll A: %v", err)
	}
	if !ok {
		t.Fatal("expected task A")
	}
	if err := svc.CompleteTask(rt.CompleteTaskRequest{
		WorkerID: "worker-1",
		RunID:    runID,
		NodeID:   task.NodeID,
		Attempt:  task.Attempt,
	}); err != nil {
		t.Fatalf("complete A: %v", err)
	}

	model, err := New(svc).Snapshot(runID, 0)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if len(model.Replay.Frames) != len(model.Timeline.Events) {
		t.Fatalf("expected replay frames to match timeline length, got %d vs %d", len(model.Replay.Frames), len(model.Timeline.Events))
	}

	first := model.Replay.Frames[0]
	if first.Seq != model.Timeline.Events[0].Seq {
		t.Fatalf("expected first replay frame seq %d, got %d", model.Timeline.Events[0].Seq, first.Seq)
	}
	if len(first.Graph.Nodes) != 1 {
		t.Fatalf("expected first replay frame to expose initial node, got %d nodes", len(first.Graph.Nodes))
	}

	last := model.Replay.Frames[len(model.Replay.Frames)-1]
	if last.RunState != rt.RunStateCompleted {
		t.Fatalf("expected final replay frame to be completed, got %s", last.RunState)
	}
	if got := last.Timeline[len(last.Timeline)-1].Type; got != rt.EventWorkflowCompleted {
		t.Fatalf("expected replay to end at workflow completed, got %s", got)
	}
}

func assertGraphNode(t *testing.T, nodes []GraphNode, nodeID string, want rt.NodeState) {
	t.Helper()
	for _, node := range nodes {
		if node.ID != nodeID {
			continue
		}
		if node.State != want {
			t.Fatalf("expected node %s to be %s, got %s", nodeID, want, node.State)
		}
		return
	}
	t.Fatalf("expected graph node %s", nodeID)
}

func assertGraphNodeCreated(t *testing.T, nodes []GraphNode, nodeID string) {
	t.Helper()
	for _, node := range nodes {
		if node.ID != nodeID {
			continue
		}
		if node.CreatedSeq == 0 {
			t.Fatalf("expected node %s to have non-zero created seq", nodeID)
		}
		if node.LastTransitionSeq == 0 {
			t.Fatalf("expected node %s to have non-zero last transition seq", nodeID)
		}
		return
	}
	t.Fatalf("expected graph node %s", nodeID)
}

func assertTimelineFamily(t *testing.T, events []TimelineEvent, family string) {
	t.Helper()
	for _, event := range events {
		if event.Family == family {
			return
		}
	}
	t.Fatalf("expected timeline family %q", family)
}

func assertGraphNodeSignal(t *testing.T, nodes []GraphNode, nodeID, signal string) {
	t.Helper()
	for _, node := range nodes {
		if node.ID != nodeID {
			continue
		}
		for _, got := range node.Signals {
			if got == signal {
				return
			}
		}
		t.Fatalf("expected node %s to have signal %s, got %+v", nodeID, signal, node.Signals)
	}
	t.Fatalf("expected graph node %s", nodeID)
}

func assertPhaseMarker(t *testing.T, phases []PhaseMarker, key string, complete bool) {
	t.Helper()
	for _, phase := range phases {
		if phase.Key != key {
			continue
		}
		if phase.Complete != complete {
			t.Fatalf("expected phase %s complete=%v, got %+v", key, complete, phase)
		}
		return
	}
	t.Fatalf("expected phase marker %s", key)
}

func assertGraphEdge(t *testing.T, edges []GraphEdge, from, to, kind string) {
	t.Helper()
	for _, edge := range edges {
		if edge.From == from && edge.To == to && edge.Kind == kind {
			return
		}
	}
	t.Fatalf("expected graph edge %s -> %s (%s), got %+v", from, to, kind, edges)
}

func assertNoGraphEdge(t *testing.T, edges []GraphEdge, from, to, kind string) {
	t.Helper()
	for _, edge := range edges {
		if edge.From == from && edge.To == to && edge.Kind == kind {
			t.Fatalf("did not expect graph edge %s -> %s (%s), got %+v", from, to, kind, edges)
		}
	}
}
