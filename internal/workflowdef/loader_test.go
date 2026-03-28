package workflowdef

import (
	"os"
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

func TestLoadJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.json")
	if err := os.WriteFile(path, []byte(`{
  "name": "linear-noop",
  "nodes": [
    {"id": "A", "activity_type": "noop", "input": {"message": "hello"}},
    {"id": "B", "activity_type": "noop", "depends_on": ["A"]}
  ]
}`), 0o644); err != nil {
		t.Fatalf("write workflow json: %v", err)
	}

	def, err := LoadJSON(path)
	if err != nil {
		t.Fatalf("LoadJSON returned error: %v", err)
	}

	if def.Name != "linear-noop" {
		t.Fatalf("expected workflow name linear-noop, got %q", def.Name)
	}
	if got := def.Nodes[0].Input["message"]; got != "hello" {
		t.Fatalf("expected node input to survive load, got %v", def.Nodes[0].Input)
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
