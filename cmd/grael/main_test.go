package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"grael/internal/workflowdef"
)

func TestBuiltInExampleLinearNoop(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	if err := run([]string{"bogus-command"}); err == nil {
		t.Fatal("expected unknown command to be rejected")
	}
}

func TestStartExampleProducesCompletedEventHistory(t *testing.T) {
	t.Parallel()

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
