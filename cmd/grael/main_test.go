package main

import (
	"testing"
)

func TestBuiltInExampleLinearNoop(t *testing.T) {
	t.Parallel()

	def, err := builtInExample("linear-noop")
	if err != nil {
		t.Fatalf("builtInExample returned error: %v", err)
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
