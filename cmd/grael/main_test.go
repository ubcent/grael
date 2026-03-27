package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"grael/internal/api"
	rt "grael/internal/runtime"
	"grael/internal/workflowdef"
)

func TestBuiltInExampleLinearNoop(t *testing.T) {
	def, err := workflowdef.BuiltIn("linear-noop")
	if err != nil {
		t.Fatalf("BuiltIn returned error: %v", err)
	}

	if def.Name != "linear-noop" {
		t.Fatalf("expected workflow name linear-noop, got %q", def.Name)
	}
	if len(def.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(def.Nodes))
	}
	if def.Nodes[1].ID != "B" || len(def.Nodes[1].DependsOn) != 1 || def.Nodes[1].DependsOn[0] != "A" {
		t.Fatalf("unexpected second node shape: %+v", def.Nodes[1])
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	if err := run([]string{"bogus-command"}); err == nil {
		t.Fatal("expected unknown command to be rejected")
	}
}

func TestStartExampleProducesCompletedEventHistory(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	out, err := stdoutString(func() error {
		return run([]string{
			"start",
			"-data-dir", dataDir,
			"-workflow", filepath.Join("..", "..", "examples", "workflows", "linear-noop.json"),
			"-demo-worker",
			"-wait-timeout", "2s",
		})
	})
	if err != nil {
		t.Fatalf("run start example: %v", err)
	}

	runID := strings.TrimSpace(out)
	if runID == "" {
		t.Fatal("expected run id output")
	}

	content, err := stdoutString(func() error {
		return run([]string{"events", "-data-dir", dataDir, "-run-id", runID})
	})
	if err != nil {
		t.Fatalf("run events: %v", err)
	}

	var events []map[string]any
	if err := json.Unmarshal([]byte(content), &events); err != nil {
		t.Fatalf("decode events json: %v", err)
	}
	if len(events) != 11 {
		t.Fatalf("expected 11 events for worker-driven linear example, got %d", len(events))
	}
	if got := events[0]["type"]; got != "WorkflowStarted" {
		t.Fatalf("expected first event WorkflowStarted, got %v", got)
	}
	if got := events[len(events)-1]["type"]; got != "WorkflowCompleted" {
		t.Fatalf("expected last event WorkflowCompleted, got %v", got)
	}
}

func TestStartLivingDagExampleProducesSpawnedExecution(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	out, err := stdoutString(func() error {
		return run([]string{
			"start",
			"-data-dir", dataDir,
			"-workflow", filepath.Join("..", "..", "examples", "workflows", "living-dag.json"),
			"-demo-worker",
			"-wait-timeout", "2s",
		})
	})
	if err != nil {
		t.Fatalf("run living-dag example: %v", err)
	}

	runID := strings.TrimSpace(out)
	if runID == "" {
		t.Fatal("expected run id output")
	}

	statusJSON, err := stdoutString(func() error {
		return run([]string{"status", "-data-dir", dataDir, "-run-id", runID})
	})
	if err != nil {
		t.Fatalf("run status: %v", err)
	}
	var view struct {
		State string                 `json:"state"`
		Nodes map[string]interface{} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(statusJSON), &view); err != nil {
		t.Fatalf("decode status json: %v", err)
	}
	if view.State != "COMPLETED" {
		t.Fatalf("expected completed run, got %s", view.State)
	}
	for _, nodeID := range []string{"discover", "analyze-1", "analyze-2", "analyze-3"} {
		if _, ok := view.Nodes[nodeID]; !ok {
			t.Fatalf("expected node %s in final run view", nodeID)
		}
	}

	content, err := stdoutString(func() error {
		return run([]string{"events", "-data-dir", dataDir, "-run-id", runID})
	})
	if err != nil {
		t.Fatalf("run events: %v", err)
	}

	var events []map[string]any
	if err := json.Unmarshal([]byte(content), &events); err != nil {
		t.Fatalf("decode events json: %v", err)
	}
	if len(events) < 11 {
		t.Fatalf("expected living-dag example to produce expanded event history, got %d events", len(events))
	}
}

func TestStartLivingDagOpsExampleExercisesCheckpointAndCompensation(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	out, err := stdoutString(func() error {
		return run([]string{
			"start",
			"-data-dir", dataDir,
			"-workflow", filepath.Join("..", "..", "examples", "workflows", "living-dag-ops.json"),
			"-demo-worker",
			"-wait-timeout", "3s",
		})
	})
	if err != nil {
		t.Fatalf("run living-dag-ops example: %v", err)
	}

	runID := strings.TrimSpace(out)
	if runID == "" {
		t.Fatal("expected run id output")
	}

	statusJSON, err := stdoutString(func() error {
		return run([]string{"status", "-data-dir", dataDir, "-run-id", runID})
	})
	if err != nil {
		t.Fatalf("run status: %v", err)
	}
	var view struct {
		State string                 `json:"state"`
		Nodes map[string]interface{} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(statusJSON), &view); err != nil {
		t.Fatalf("decode status json: %v", err)
	}
	if view.State != "COMPENSATED" {
		t.Fatalf("expected compensated run, got %s", view.State)
	}
	for _, nodeID := range []string{"discover", "analyze-1", "review", "finalize"} {
		if _, ok := view.Nodes[nodeID]; !ok {
			t.Fatalf("expected node %s in final run view", nodeID)
		}
	}

	content, err := stdoutString(func() error {
		return run([]string{"events", "-data-dir", dataDir, "-run-id", runID})
	})
	if err != nil {
		t.Fatalf("run events: %v", err)
	}

	var events []map[string]any
	if err := json.Unmarshal([]byte(content), &events); err != nil {
		t.Fatalf("decode events json: %v", err)
	}
	want := []string{
		"CheckpointReached",
		"CheckpointApproved",
		"CompensationStarted",
		"CompensationTaskStarted",
		"CompensationTaskCompleted",
		"CompensationCompleted",
	}
	index := 0
	for _, event := range events {
		if index >= len(want) {
			break
		}
		if event["type"] == want[index] {
			index++
		}
	}
	if index != len(want) {
		t.Fatalf("expected event subsequence %v, got %v", want, events)
	}
}

func TestStartCoreDemoExampleExercisesSpawnRetryAndApproval(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	out, err := stdoutString(func() error {
		return run([]string{
			"start",
			"-data-dir", dataDir,
			"-workflow", filepath.Join("..", "..", "examples", "workflows", "core-demo.json"),
			"-demo-worker",
			"-wait-timeout", "3s",
		})
	})
	if err != nil {
		t.Fatalf("run core-demo example: %v", err)
	}

	runID := strings.TrimSpace(out)
	if runID == "" {
		t.Fatal("expected run id output")
	}

	svc := api.New(dataDir)
	defer svc.Close()

	view, err := svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if view.State != rt.RunStateCompleted {
		t.Fatalf("expected completed run, got %s", view.State)
	}
	for _, nodeID := range []string{"discover", "analyze-1", "analyze-2", "review", "summary"} {
		if _, ok := view.Nodes[nodeID]; !ok {
			t.Fatalf("expected node %s in final run view", nodeID)
		}
	}

	events, err := svc.ListEvents(runID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	assertEventSubsequence(t, events, []rt.EventType{
		rt.EventWorkflowStarted,
		rt.EventNodeFailed,
		rt.EventTimerScheduled,
		rt.EventCheckpointReached,
		rt.EventTimerFired,
		rt.EventCheckpointApproved,
		rt.EventWorkflowCompleted,
	})
}

func TestCoreDemoHarnessSurvivesRestartMidExecution(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	def, err := workflowdef.LoadJSON(filepath.Join("..", "..", "examples", "workflows", "core-demo.json"))
	if err != nil {
		t.Fatalf("load core-demo workflow: %v", err)
	}

	svc := api.New(dataDir)
	runID, err := svc.StartRun(def, nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	if err := startDemoWorkerWithContext(ctx, svc, runID, def); err != nil {
		t.Fatalf("start demo worker: %v", err)
	}

	if err := waitForEventSubsequence(svc, runID, []rt.EventType{
		rt.EventWorkflowStarted,
		rt.EventNodeFailed,
		rt.EventTimerScheduled,
		rt.EventCheckpointReached,
	}, 2*time.Second); err != nil {
		t.Fatalf("wait for pre-restart progress: %v", err)
	}

	cancel()
	svc.Close()

	svc = api.New(dataDir)
	defer svc.Close()

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	if err := startDemoWorkerWithContext(ctx, svc, runID, def); err != nil {
		t.Fatalf("restart demo worker: %v", err)
	}
	if err := waitForTerminalRun(svc, runID, 3*time.Second); err != nil {
		t.Fatalf("wait for terminal run after restart: %v", err)
	}

	view, err := svc.GetRun(runID)
	if err != nil {
		t.Fatalf("get run after restart: %v", err)
	}
	if view.State != rt.RunStateCompleted {
		t.Fatalf("expected completed run after restart, got %s", view.State)
	}
	if got := view.Nodes["analyze-2"].State; got != rt.NodeStateCompleted {
		t.Fatalf("expected analyze-2 progress to survive restart, got %s", got)
	}

	events, err := svc.ListEvents(runID)
	if err != nil {
		t.Fatalf("list events after restart: %v", err)
	}
	assertEventSubsequence(t, events, []rt.EventType{
		rt.EventWorkflowStarted,
		rt.EventNodeFailed,
		rt.EventTimerScheduled,
		rt.EventCheckpointReached,
		rt.EventTimerFired,
		rt.EventCheckpointApproved,
		rt.EventWorkflowCompleted,
	})
}

func waitForEventSubsequence(svc *api.Service, runID string, want []rt.EventType, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		events, err := svc.ListEvents(runID)
		if err != nil {
			return err
		}
		if hasEventSubsequence(events, want) {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return context.DeadlineExceeded
}

func assertEventSubsequence(t *testing.T, events []rt.Event, want []rt.EventType) {
	t.Helper()
	if !hasEventSubsequence(events, want) {
		t.Fatalf("expected event subsequence %v, got %v", want, events)
	}
}

func hasEventSubsequence(events []rt.Event, want []rt.EventType) bool {
	index := 0
	for _, event := range events {
		if index >= len(want) {
			break
		}
		if event.Type == want[index] {
			index++
		}
	}
	return index == len(want)
}
