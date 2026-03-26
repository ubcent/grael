package scheduler

import (
	"slices"

	rt "grael/internal/runtime"
	"grael/internal/state"
)

type Scheduler struct{}

func New() *Scheduler {
	return &Scheduler{}
}

// Decide is a pure function over derived state. It must not read time, perform
// I/O, or depend on map iteration order.
func (s *Scheduler) Decide(st *state.ExecutionState) []rt.Command {
	if st.IsTerminal() {
		return nil
	}

	var commands []rt.Command
	nodeIDs := make([]string, 0, len(st.Nodes))
	for id := range st.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	// Stable ordering is part of the contract: identical state should yield
	// identical command order without depending on map iteration.
	slices.Sort(nodeIDs)

	allCompleted := len(nodeIDs) > 0
	for _, id := range nodeIDs {
		node := st.Nodes[id]
		switch node.State {
		case rt.NodeStateReady:
			commands = append(commands, rt.Command{
				Type:   rt.CommandStartNode,
				RunID:  st.RunID,
				NodeID: node.ID,
			})
			if node.ActivityType == rt.ActivityTypeNoop {
				commands = append(commands, rt.Command{
					Type:   rt.CommandCompleteNode,
					RunID:  st.RunID,
					NodeID: node.ID,
				})
			}
		}
		if node.State != rt.NodeStateCompleted {
			allCompleted = false
		}
	}

	if allCompleted && len(nodeIDs) > 0 {
		commands = append(commands, rt.Command{
			Type:  rt.CommandCompleteWorkflow,
			RunID: st.RunID,
		})
	}

	return commands
}
