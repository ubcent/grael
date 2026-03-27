package wal_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	rt "grael/internal/runtime"
	"grael/internal/wal"
)

func TestListStopsAtCorruptTail(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := wal.NewStore(dir)

	runID := "run-corrupt-tail"
	for _, eventType := range []rt.EventType{rt.EventWorkflowStarted, rt.EventWorkflowCompleted} {
		_, err := store.Append(rt.Event{
			RunID:     runID,
			Type:      eventType,
			Timestamp: time.Now().UTC(),
			Payload:   payloadFor(eventType),
		})
		if err != nil {
			t.Fatalf("append event: %v", err)
		}
	}

	f, err := os.OpenFile(filepath.Join(dir, runID+".wal"), os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open wal: %v", err)
	}
	if _, err := f.WriteString("{broken-json\n"); err != nil {
		t.Fatalf("write corrupt tail: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close wal: %v", err)
	}

	events, err := store.List(runID)
	if err != wal.ErrCorruptTail {
		t.Fatalf("expected ErrCorruptTail, got %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 valid events, got %d", len(events))
	}
}

func TestSubscribeReplaysCommittedHistoryThenStreamsLiveAppends(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := wal.NewStore(dir)
	runID := "run-stream"

	first, err := store.Append(rt.Event{
		RunID:     runID,
		Type:      rt.EventWorkflowStarted,
		Timestamp: time.Now().UTC(),
		Payload:   payloadFor(rt.EventWorkflowStarted),
	})
	if err != nil {
		t.Fatalf("append first event: %v", err)
	}

	events, cancel, err := store.Subscribe(runID, 0)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer cancel()

	select {
	case event := <-events:
		if event.Seq != first.Seq || event.Type != first.Type {
			t.Fatalf("expected replayed first event %+v, got %+v", first, event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for replayed event")
	}

	second, err := store.Append(rt.Event{
		RunID:     runID,
		Type:      rt.EventWorkflowCompleted,
		Timestamp: time.Now().UTC(),
		Payload:   payloadFor(rt.EventWorkflowCompleted),
	})
	if err != nil {
		t.Fatalf("append second event: %v", err)
	}

	select {
	case event := <-events:
		if event.Seq != second.Seq || event.Type != second.Type {
			t.Fatalf("expected live second event %+v, got %+v", second, event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for live event")
	}
}

func payloadFor(eventType rt.EventType) interface{} {
	switch eventType {
	case rt.EventWorkflowStarted:
		return rt.WorkflowStartedPayload{
			Workflow: rt.WorkflowDefinition{
				Name: "test",
				Nodes: []rt.NodeDefinition{
					{ID: "A", ActivityType: rt.ActivityTypeNoop},
				},
			},
		}
	case rt.EventWorkflowCompleted:
		return rt.WorkflowCompletedPayload{}
	default:
		return nil
	}
}
