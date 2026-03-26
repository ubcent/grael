package workflowdef

import (
	"path/filepath"
	"testing"

	rt "grael/internal/runtime"
)

func TestBuiltInLinearNoop(t *testing.T) {
	t.Parallel()

	def, err := BuiltIn("linear-noop")
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

func TestLoadJSONExample(t *testing.T) {
	t.Parallel()

	def, err := LoadJSON(filepath.Join("..", "..", "examples", "workflows", "linear-noop.json"))
	if err != nil {
		t.Fatalf("LoadJSON returned error: %v", err)
	}

	if def.Name != "linear-noop" {
		t.Fatalf("expected workflow name linear-noop, got %q", def.Name)
	}
}

func TestNormalizeRejectsUnknownDependency(t *testing.T) {
	t.Parallel()

	_, err := Normalize(rt.WorkflowDefinition{
		Name: "bad",
		Nodes: []rt.NodeDefinition{
			{ID: "A", ActivityType: rt.ActivityTypeNoop, DependsOn: []string{"missing"}},
		},
	})
	if err == nil {
		t.Fatal("expected unknown dependency to be rejected")
	}
}

func TestNormalizeRejectsDuplicateNodeID(t *testing.T) {
	t.Parallel()

	_, err := Normalize(rt.WorkflowDefinition{
		Name: "bad",
		Nodes: []rt.NodeDefinition{
			{ID: "A", ActivityType: rt.ActivityTypeNoop},
			{ID: "A", ActivityType: rt.ActivityTypeNoop},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate node IDs to be rejected")
	}
}
