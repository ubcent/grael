package state

import (
	"fmt"
	"maps"
	"slices"
	"time"

	rt "grael/internal/runtime"
)

type ExecutionState struct {
	RunID              string
	Workflow           string
	DefinitionHash     string
	Input              map[string]any
	RunState           rt.RunState
	CancelRequested    bool
	CompensationStack  []CompensationEntry
	ActiveCompensation *ActiveCompensation
	CreatedAt          time.Time
	FinishedAt         *time.Time
	LastSeq            uint64
	Nodes              map[string]*Node
	Timers             map[string]*Timer
}

type Node struct {
	ID                   string
	ActivityType         rt.ActivityType
	Input                map[string]any
	DependsOn            []string
	RetryPolicy          *rt.RetryPolicy
	CompensationActivity rt.ActivityType
	CheckpointTimeout    time.Duration
	ExecutionDeadline    time.Duration
	AbsoluteDeadline     time.Duration
	State                rt.NodeState
	Attempt              uint32
	WorkerID             string
	LastHeartbeatAt      time.Time
	LastError            string
	Output               map[string]any
}

type Timer struct {
	ID      string
	NodeID  string
	Attempt uint32
	Purpose rt.TimerPurpose
	FireAt  time.Time
	Fired   bool
}

type CompensationEntry struct {
	NodeID       string
	ActivityType rt.ActivityType
	Completed    bool
	Attempt      uint32
}

type ActiveCompensation struct {
	NodeID          string
	WorkerID        string
	Attempt         uint32
	LastHeartbeatAt time.Time
}

// New returns the empty derived state used both for new runs and full replay.
func New() *ExecutionState {
	return &ExecutionState{
		RunState: rt.RunStateRunning,
		Nodes:    map[string]*Node{},
		Timers:   map[string]*Timer{},
	}
}

// Rehydrate rebuilds state exclusively from the persisted event history.
func Rehydrate(events []rt.Event) (*ExecutionState, error) {
	st := New()
	for _, event := range events {
		if err := st.Apply(event); err != nil {
			return nil, err
		}
	}
	return st, nil
}

// Apply is the only legal way to mutate derived execution state.
func (s *ExecutionState) Apply(event rt.Event) error {
	switch event.Type {
	case rt.EventWorkflowStarted:
		payload := event.Payload.(rt.WorkflowStartedPayload)
		s.RunID = event.RunID
		s.Workflow = payload.Workflow.Name
		s.DefinitionHash = payload.DefinitionHash
		s.Input = maps.Clone(payload.Input)
		s.RunState = rt.RunStateRunning
		s.CreatedAt = event.Timestamp
		for _, def := range payload.Workflow.Nodes {
			s.Nodes[def.ID] = &Node{
				ID:                   def.ID,
				ActivityType:         def.ActivityType,
				Input:                maps.Clone(def.Input),
				DependsOn:            slices.Clone(def.DependsOn),
				RetryPolicy:          def.RetryPolicy,
				CompensationActivity: def.CompensationActivity,
				CheckpointTimeout:    def.CheckpointTimeout,
				ExecutionDeadline:    def.ExecutionDeadline,
				AbsoluteDeadline:     def.AbsoluteDeadline,
				State:                rt.NodeStatePending,
			}
		}
		// Readiness is derived, not stored independently. Recomputing it here keeps
		// the state model reconstructable from the event log alone.
		s.markReadyNodes()
	case rt.EventLeaseGranted:
		payload := event.Payload.(rt.LeaseGrantedPayload)
		node, ok := s.Nodes[payload.NodeID]
		if !ok {
			return fmt.Errorf("lease granted for unknown node %q", payload.NodeID)
		}
		node.Attempt = payload.Attempt
		node.WorkerID = payload.WorkerID
		node.LastHeartbeatAt = event.Timestamp
		node.LastError = ""
	case rt.EventHeartbeatRecorded:
		payload := event.Payload.(rt.HeartbeatRecordedPayload)
		node, ok := s.Nodes[payload.NodeID]
		if !ok {
			if s.ActiveCompensation != nil && s.ActiveCompensation.NodeID == payload.NodeID && s.ActiveCompensation.Attempt == payload.Attempt && s.ActiveCompensation.WorkerID == payload.WorkerID {
				s.ActiveCompensation.LastHeartbeatAt = event.Timestamp
				break
			}
			return fmt.Errorf("heartbeat recorded for unknown node %q", payload.NodeID)
		}
		if node.Attempt == payload.Attempt && node.WorkerID == payload.WorkerID {
			node.LastHeartbeatAt = event.Timestamp
		}
		if s.ActiveCompensation != nil && s.ActiveCompensation.NodeID == payload.NodeID && s.ActiveCompensation.Attempt == payload.Attempt && s.ActiveCompensation.WorkerID == payload.WorkerID {
			s.ActiveCompensation.LastHeartbeatAt = event.Timestamp
		}
	case rt.EventLeaseExpired:
		payload := event.Payload.(rt.LeaseExpiredPayload)
		node, ok := s.Nodes[payload.NodeID]
		if !ok {
			return fmt.Errorf("lease expired for unknown node %q", payload.NodeID)
		}
		if node.Attempt != payload.Attempt {
			return fmt.Errorf("lease expired for stale attempt %d on node %q", payload.Attempt, payload.NodeID)
		}
		node.State = rt.NodeStateReady
		node.WorkerID = ""
		node.LastHeartbeatAt = time.Time{}
		node.LastError = "lease expired"
	case rt.EventCancellationRequested:
		s.CancelRequested = true
	case rt.EventCancellationCompleted:
		s.RunState = rt.RunStateCancelled
		finishedAt := event.Timestamp
		s.FinishedAt = &finishedAt
	case rt.EventCompensationStarted:
		s.RunState = rt.RunStateCompensating
	case rt.EventCompensationTaskStarted:
		payload := event.Payload.(rt.CompensationTaskStartedPayload)
		for i := range s.CompensationStack {
			if s.CompensationStack[i].NodeID != payload.NodeID {
				continue
			}
			s.CompensationStack[i].Attempt = payload.Attempt
			s.ActiveCompensation = &ActiveCompensation{
				NodeID:          payload.NodeID,
				WorkerID:        payload.WorkerID,
				Attempt:         payload.Attempt,
				LastHeartbeatAt: event.Timestamp,
			}
			break
		}
	case rt.EventCompensationTaskCompleted:
		payload := event.Payload.(rt.CompensationTaskCompletedPayload)
		for i := range s.CompensationStack {
			if s.CompensationStack[i].NodeID != payload.NodeID {
				continue
			}
			s.CompensationStack[i].Completed = true
			break
		}
		s.ActiveCompensation = nil
	case rt.EventCompensationTaskExpired:
		s.ActiveCompensation = nil
	case rt.EventCompensationTaskFailed:
		s.ActiveCompensation = nil
	case rt.EventCompensationCompleted:
		s.RunState = rt.RunStateCompensated
		s.ActiveCompensation = nil
		finishedAt := event.Timestamp
		s.FinishedAt = &finishedAt
	case rt.EventTimerScheduled:
		payload := event.Payload.(rt.TimerScheduledPayload)
		s.Timers[payload.TimerID] = &Timer{
			ID:      payload.TimerID,
			NodeID:  payload.NodeID,
			Attempt: payload.Attempt,
			Purpose: payload.Purpose,
			FireAt:  payload.FireAt,
		}
	case rt.EventTimerFired:
		payload := event.Payload.(rt.TimerFiredPayload)
		timer, ok := s.Timers[payload.TimerID]
		if !ok {
			return fmt.Errorf("timer fired for unknown timer %q", payload.TimerID)
		}
		timer.Fired = true
		if payload.Purpose == rt.TimerPurposeRetryBackoff {
			node, ok := s.Nodes[payload.NodeID]
			if !ok {
				return fmt.Errorf("retry timer fired for unknown node %q", payload.NodeID)
			}
			if node.Attempt == payload.Attempt && node.State == rt.NodeStateFailed {
				node.State = rt.NodeStateReady
				node.WorkerID = ""
				node.LastHeartbeatAt = time.Time{}
			}
		}
	case rt.EventCheckpointReached:
		payload := event.Payload.(rt.CheckpointReachedPayload)
		node, ok := s.Nodes[payload.NodeID]
		if !ok {
			return fmt.Errorf("checkpoint reached for unknown node %q", payload.NodeID)
		}
		node.State = rt.NodeStateAwaitingApproval
		node.WorkerID = ""
		node.LastHeartbeatAt = time.Time{}
		node.LastError = ""
	case rt.EventCheckpointApproved:
		payload := event.Payload.(rt.CheckpointApprovedPayload)
		node, ok := s.Nodes[payload.NodeID]
		if !ok {
			return fmt.Errorf("checkpoint approved for unknown node %q", payload.NodeID)
		}
		if node.State == rt.NodeStateAwaitingApproval {
			node.State = rt.NodeStateReady
			node.WorkerID = ""
			node.LastHeartbeatAt = time.Time{}
		}
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
		node.Attempt = payload.Attempt
		node.WorkerID = payload.WorkerID
		node.LastHeartbeatAt = event.Timestamp
		node.State = rt.NodeStateRunning
	case rt.EventNodeCancelled:
		payload := event.Payload.(rt.NodeCancelledPayload)
		node, ok := s.Nodes[payload.NodeID]
		if !ok {
			return fmt.Errorf("node cancelled for unknown node %q", payload.NodeID)
		}
		node.State = rt.NodeStateCancelled
		node.WorkerID = ""
		node.LastHeartbeatAt = time.Time{}
		node.LastError = "cancelled"
	case rt.EventNodeCompleted:
		payload := event.Payload.(rt.NodeCompletedPayload)
		node, ok := s.Nodes[payload.NodeID]
		if !ok {
			return fmt.Errorf("node completed for unknown node %q", payload.NodeID)
		}
		node.State = rt.NodeStateCompleted
		node.WorkerID = payload.WorkerID
		node.Attempt = payload.Attempt
		node.LastHeartbeatAt = time.Time{}
		node.LastError = ""
		node.Output = payload.Output
		if node.CompensationActivity != "" {
			s.CompensationStack = append(s.CompensationStack, CompensationEntry{
				NodeID:       node.ID,
				ActivityType: node.CompensationActivity,
			})
		}
		for _, def := range payload.SpawnedNodes {
			s.Nodes[def.ID] = &Node{
				ID:                   def.ID,
				ActivityType:         def.ActivityType,
				Input:                maps.Clone(def.Input),
				DependsOn:            slices.Clone(def.DependsOn),
				RetryPolicy:          def.RetryPolicy,
				CompensationActivity: def.CompensationActivity,
				CheckpointTimeout:    def.CheckpointTimeout,
				ExecutionDeadline:    def.ExecutionDeadline,
				AbsoluteDeadline:     def.AbsoluteDeadline,
				State:                rt.NodeStatePending,
			}
		}
		s.markReadyNodes()
	case rt.EventNodeFailed:
		payload := event.Payload.(rt.NodeFailedPayload)
		node, ok := s.Nodes[payload.NodeID]
		if !ok {
			return fmt.Errorf("node failed for unknown node %q", payload.NodeID)
		}
		node.State = rt.NodeStateFailed
		node.WorkerID = payload.WorkerID
		node.Attempt = payload.Attempt
		node.LastHeartbeatAt = time.Time{}
		node.LastError = payload.Message
	case rt.EventWorkflowFailed:
		s.RunState = rt.RunStateFailed
		finishedAt := event.Timestamp
		s.FinishedAt = &finishedAt
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
	return s.RunState == rt.RunStateCompleted || s.RunState == rt.RunStateFailed || s.RunState == rt.RunStateCancelled || s.RunState == rt.RunStateCompensated
}

// View materializes the external read model from the current derived state.
func (s *ExecutionState) View() rt.RunView {
	nodes := make(map[string]rt.NodeView, len(s.Nodes))
	for id, node := range s.Nodes {
		nodes[id] = rt.NodeView{
			ID:           node.ID,
			ActivityType: node.ActivityType,
			State:        node.State,
			DependsOn:    slices.Clone(node.DependsOn),
			Attempt:      node.Attempt,
			WorkerID:     node.WorkerID,
			LastError:    node.LastError,
		}
	}
	return rt.RunView{
		RunID:          s.RunID,
		Workflow:       s.Workflow,
		DefinitionHash: s.DefinitionHash,
		State:          s.RunState,
		LastSeq:        s.LastSeq,
		Nodes:          nodes,
		CreatedAt:      s.CreatedAt,
		FinishedAt:     s.FinishedAt,
	}
}
