package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	rt "grael/internal/runtime"
)

func TestSnapshotEndpointReturnsProjectedDemoModel(t *testing.T) {
	t.Parallel()

	srv := New(t.TempDir())
	defer srv.Close()

	if err := srv.svc.RegisterWorker("worker-1", []rt.ActivityType{"step"}); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	runID, err := srv.svc.StartRun(rt.WorkflowDefinition{
		Name: "demo-server",
		Nodes: []rt.NodeDefinition{
			{ID: "A", ActivityType: "step"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	task, ok, err := srv.svc.PollTask("worker-1", 250*time.Millisecond)
	if err != nil {
		t.Fatalf("poll task: %v", err)
	}
	if !ok {
		t.Fatal("expected task A")
	}
	if err := srv.svc.CompleteTask(rt.CompleteTaskRequest{
		WorkerID: "worker-1",
		RunID:    runID,
		NodeID:   task.NodeID,
		Attempt:  task.Attempt,
	}); err != nil {
		t.Fatalf("complete task: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/runs/"+runID+"/snapshot?after_seq=0", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Run struct {
			RunID string `json:"run_id"`
			State string `json:"state"`
		} `json:"run"`
		Graph struct {
			Nodes []struct {
				ID string `json:"id"`
			} `json:"nodes"`
			Edges []any `json:"edges"`
		} `json:"graph"`
		Timeline struct {
			Events []struct {
				Type string `json:"type"`
			} `json:"events"`
		} `json:"timeline"`
		Cursor struct {
			CurrentSeq uint64 `json:"current_seq"`
			HasChanges bool   `json:"has_changes"`
		} `json:"cursor"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Run.RunID != runID {
		t.Fatalf("expected run id %s, got %s", runID, body.Run.RunID)
	}
	if body.Run.State != string(rt.RunStateCompleted) {
		t.Fatalf("expected completed state, got %s", body.Run.State)
	}
	if len(body.Graph.Nodes) != 1 {
		t.Fatalf("expected 1 graph node, got %d", len(body.Graph.Nodes))
	}
	if len(body.Timeline.Events) == 0 {
		t.Fatal("expected non-empty timeline")
	}
	if body.Cursor.CurrentSeq == 0 || !body.Cursor.HasChanges {
		t.Fatalf("expected non-zero current seq and changes, got %+v", body.Cursor)
	}
}
