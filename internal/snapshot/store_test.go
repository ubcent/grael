package snapshot_test

import (
	"testing"
	"time"

	rt "grael/internal/runtime"
	"grael/internal/snapshot"
	"grael/internal/state"
)

func TestSaveAndLoadSnapshot(t *testing.T) {
	t.Parallel()

	st := state.New()
	if err := st.Apply(rt.Event{
		Seq:       1,
		RunID:     "run-snapshot",
		Type:      rt.EventWorkflowStarted,
		Timestamp: time.Now().UTC(),
		Payload: rt.WorkflowStartedPayload{
			Workflow: rt.WorkflowDefinition{
				Name: "linear",
				Nodes: []rt.NodeDefinition{
					{ID: "A", ActivityType: rt.ActivityTypeNoop},
				},
			},
		},
	}); err != nil {
		t.Fatalf("apply workflow started: %v", err)
	}

	store := snapshot.NewStore(t.TempDir())
	if err := store.Save(st); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	loaded, ok, err := store.Load("run-snapshot")
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if !ok {
		t.Fatal("expected snapshot to exist")
	}
	if loaded.RunID != st.RunID || loaded.Workflow != st.Workflow || loaded.LastSeq != st.LastSeq {
		t.Fatalf("loaded snapshot mismatch: got %+v want %+v", loaded, st)
	}
}
