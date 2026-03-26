package processor

import (
	"fmt"
	"time"

	rt "grael/internal/runtime"
	"grael/internal/state"
	"grael/internal/wal"
)

type Processor struct {
	wal *wal.Store
}

func New(store *wal.Store) *Processor {
	return &Processor{wal: store}
}

// Execute turns scheduler commands into persisted events. A command that no
// longer matches the current state becomes a no-op rather than forcing state.
func (p *Processor) Execute(st *state.ExecutionState, cmd rt.Command) ([]rt.Event, error) {
	switch cmd.Type {
	case rt.CommandStartNode:
		node := st.Nodes[cmd.NodeID]
		if node == nil {
			return nil, fmt.Errorf("start unknown node %q", cmd.NodeID)
		}
		if node.State != rt.NodeStateReady {
			return nil, nil
		}
		event, err := p.wal.Append(rt.Event{
			RunID:     cmd.RunID,
			Type:      rt.EventNodeStarted,
			Timestamp: time.Now().UTC(),
			Payload: rt.NodeStartedPayload{
				NodeID: cmd.NodeID,
			},
		})
		if err != nil {
			return nil, err
		}
		return []rt.Event{event}, nil
	case rt.CommandCompleteNode:
		node := st.Nodes[cmd.NodeID]
		if node == nil {
			return nil, fmt.Errorf("complete unknown node %q", cmd.NodeID)
		}
		if node.State != rt.NodeStateRunning {
			return nil, nil
		}
		event, err := p.wal.Append(rt.Event{
			RunID:     cmd.RunID,
			Type:      rt.EventNodeCompleted,
			Timestamp: time.Now().UTC(),
			Payload: rt.NodeCompletedPayload{
				NodeID: cmd.NodeID,
				Output: map[string]any{"status": "ok"},
			},
		})
		if err != nil {
			return nil, err
		}
		return []rt.Event{event}, nil
	case rt.CommandCompleteWorkflow:
		event, err := p.wal.Append(rt.Event{
			RunID:     cmd.RunID,
			Type:      rt.EventWorkflowCompleted,
			Timestamp: time.Now().UTC(),
			Payload:   rt.WorkflowCompletedPayload{},
		})
		if err != nil {
			return nil, err
		}
		return []rt.Event{event}, nil
	default:
		return nil, fmt.Errorf("unsupported command type %q", cmd.Type)
	}
}
