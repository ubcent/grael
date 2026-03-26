package state

import (
	"fmt"
	"slices"
	"time"

	rt "grael/internal/runtime"
)

type ExecutionState struct {
	RunID      string
	Workflow   string
	RunState   rt.RunState
	CreatedAt  time.Time
	FinishedAt *time.Time
	LastSeq    uint64
	Nodes      map[string]*Node
}

type Node struct {
	ID           string
	ActivityType rt.ActivityType
	DependsOn    []string
	State        rt.NodeState
	Output       map[string]any
}

func New() *ExecutionState {
	return &ExecutionState{
		RunState: rt.RunStateRunning,
		Nodes:    map[string]*Node{},
	}
}

func Rehydrate(events []rt.Event) (*ExecutionState, error) {
	st := New()
	for _, event := range events {
		if err := st.Apply(event); err != nil {
			return nil, err
		}
	}
	return st, nil
}

func (s *ExecutionState) Apply(event rt.Event) error {
	switch event.Type {
	case rt.EventWorkflowStarted:
		payload := event.Payload.(rt.WorkflowStartedPayload)
		s.RunID = event.RunID
		s.Workflow = payload.Workflow.Name
		s.RunState = rt.RunStateRunning
		s.CreatedAt = event.Timestamp
		for _, def := range payload.Workflow.Nodes {
			s.Nodes[def.ID] = &Node{
				ID:           def.ID,
				ActivityType: def.ActivityType,
				DependsOn:    slices.Clone(def.DependsOn),
				State:        rt.NodeStatePending,
			}
		}
		// Readiness is derived, not stored independently. Recomputing it here keeps
		// the state model reconstructable from the event log alone.
		s.markReadyNodes()
	case rt.EventNodeReady:
		payload := event.Payload.(rt.NodeReadyPayload)
		node, ok := s.Nodes[payload.NodeID]
		if !ok {
			return fmt.Errorf("node ready for unknown node %q", payload.NodeID)
		}
		if node.State == rt.NodeStatePending {
			node.State = rt.NodeStateReady
		}
	case rt.EventNodeStarted:
		payload := event.Payload.(rt.NodeStartedPayload)
		node, ok := s.Nodes[payload.NodeID]
		if !ok {
			return fmt.Errorf("node started for unknown node %q", payload.NodeID)
		}
		node.State = rt.NodeStateRunning
	case rt.EventNodeCompleted:
		payload := event.Payload.(rt.NodeCompletedPayload)
		node, ok := s.Nodes[payload.NodeID]
		if !ok {
			return fmt.Errorf("node completed for unknown node %q", payload.NodeID)
		}
		node.State = rt.NodeStateCompleted
		node.Output = payload.Output
		s.markReadyNodes()
	case rt.EventWorkflowCompleted:
		s.RunState = rt.RunStateCompleted
		finishedAt := event.Timestamp
		s.FinishedAt = &finishedAt
	default:
		return fmt.Errorf("unsupported event type %q", event.Type)
	}
	s.LastSeq = event.Seq
	return nil
}

func (s *ExecutionState) markReadyNodes() {
	for _, node := range s.Nodes {
		if node.State != rt.NodeStatePending {
			continue
		}
		if s.dependenciesCompleted(node.DependsOn) {
			node.State = rt.NodeStateReady
		}
	}
}

func (s *ExecutionState) dependenciesCompleted(depIDs []string) bool {
	for _, depID := range depIDs {
		node, ok := s.Nodes[depID]
		if !ok || node.State != rt.NodeStateCompleted {
			return false
		}
	}
	return true
}

func (s *ExecutionState) IsTerminal() bool {
	return s.RunState == rt.RunStateCompleted
}

func (s *ExecutionState) View() rt.RunView {
	nodes := make(map[string]rt.NodeView, len(s.Nodes))
	for id, node := range s.Nodes {
		nodes[id] = rt.NodeView{
			ID:           node.ID,
			ActivityType: node.ActivityType,
			State:        node.State,
			DependsOn:    slices.Clone(node.DependsOn),
		}
	}
	return rt.RunView{
		RunID:      s.RunID,
		Workflow:   s.Workflow,
		State:      s.RunState,
		LastSeq:    s.LastSeq,
		Nodes:      nodes,
		CreatedAt:  s.CreatedAt,
		FinishedAt: s.FinishedAt,
	}
}
