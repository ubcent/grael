package workflowdef

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	rt "grael/internal/runtime"
)

// LoadJSON reads a workflow definition from disk and normalizes it into the
// canonical runtime model. JSON is only an ingress format.
func LoadJSON(path string) (rt.WorkflowDefinition, error) {
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return rt.WorkflowDefinition{}, fmt.Errorf("read workflow file: %w", err)
	}

	var def rt.WorkflowDefinition
	if err := json.Unmarshal(content, &def); err != nil {
		return rt.WorkflowDefinition{}, fmt.Errorf("decode workflow file: %w", err)
	}

	return Normalize(def)
}

// BuiltIn returns a normalized built-in workflow example through the same
// validation path used for file-based definitions.
func BuiltIn(name string) (rt.WorkflowDefinition, error) {
	switch name {
	case "linear-noop":
		return Normalize(rt.WorkflowDefinition{
			Name: "linear-noop",
			Nodes: []rt.NodeDefinition{
				{ID: "A", ActivityType: rt.ActivityTypeNoop},
				{ID: "B", ActivityType: rt.ActivityTypeNoop, DependsOn: []string{"A"}},
				{ID: "C", ActivityType: rt.ActivityTypeNoop, DependsOn: []string{"B"}},
			},
		})
	default:
		return rt.WorkflowDefinition{}, fmt.Errorf("unknown built-in example %q", name)
	}
}

// Normalize validates the external workflow contract and returns the canonical
// in-memory definition that the runtime consumes independent of authoring
// format.
func Normalize(def rt.WorkflowDefinition) (rt.WorkflowDefinition, error) {
	if def.Name == "" {
		return rt.WorkflowDefinition{}, errors.New("workflow name is required")
	}
	if len(def.Nodes) == 0 {
		return rt.WorkflowDefinition{}, errors.New("workflow must contain at least one node")
	}

	seen := make(map[string]struct{}, len(def.Nodes))
	normalized := rt.WorkflowDefinition{
		Name:  def.Name,
		Nodes: make([]rt.NodeDefinition, 0, len(def.Nodes)),
	}

	for _, node := range def.Nodes {
		if node.ID == "" {
			return rt.WorkflowDefinition{}, errors.New("every node must have an id")
		}
		if _, exists := seen[node.ID]; exists {
			return rt.WorkflowDefinition{}, fmt.Errorf("duplicate node id %q", node.ID)
		}
		if node.ActivityType == "" {
			return rt.WorkflowDefinition{}, fmt.Errorf("node %q must define activity_type", node.ID)
		}

		normalized.Nodes = append(normalized.Nodes, rt.NodeDefinition{
			ID:                   node.ID,
			ActivityType:         node.ActivityType,
			DependsOn:            append([]string(nil), node.DependsOn...),
			RetryPolicy:          normalizeRetryPolicy(node.RetryPolicy),
			CompensationActivity: node.CompensationActivity,
			CheckpointTimeout:    normalizeExecutionDeadline(node.CheckpointTimeout),
			ExecutionDeadline:    normalizeExecutionDeadline(node.ExecutionDeadline),
			AbsoluteDeadline:     normalizeExecutionDeadline(node.AbsoluteDeadline),
		})
		seen[node.ID] = struct{}{}
	}

	for _, node := range normalized.Nodes {
		for _, dep := range node.DependsOn {
			if dep == node.ID {
				return rt.WorkflowDefinition{}, fmt.Errorf("node %q cannot depend on itself", node.ID)
			}
			if _, exists := seen[dep]; !exists {
				return rt.WorkflowDefinition{}, fmt.Errorf("node %q depends on unknown node %q", node.ID, dep)
			}
		}
	}

	return normalized, nil
}

func normalizeRetryPolicy(policy *rt.RetryPolicy) *rt.RetryPolicy {
	if policy == nil {
		return nil
	}
	copyPolicy := *policy
	if copyPolicy.MaxAttempts < 0 {
		copyPolicy.MaxAttempts = 0
	}
	if copyPolicy.Backoff < 0 {
		copyPolicy.Backoff = 0
	}
	return &copyPolicy
}

func normalizeExecutionDeadline(deadline time.Duration) time.Duration {
	if deadline < 0 {
		return 0
	}
	return deadline
}
