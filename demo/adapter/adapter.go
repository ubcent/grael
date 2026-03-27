package adapter

import (
	"fmt"
	"sort"
	"time"

	rt "grael/internal/runtime"
	st "grael/internal/state"
)

type Reader interface {
	GetRun(runID string) (rt.RunView, error)
	ListEvents(runID string) ([]rt.Event, error)
}

type Adapter struct {
	reader Reader
}

func New(reader Reader) *Adapter {
	return &Adapter{reader: reader}
}

type Snapshot struct {
	Run      RunSummary      `json:"run"`
	Graph    Graph           `json:"graph"`
	Timeline Timeline        `json:"timeline"`
	Phases   []PhaseMarker   `json:"phases"`
	Replay   Replay          `json:"replay"`
	Cursor   Cursor          `json:"cursor"`
	Delta    []TimelineEvent `json:"delta"`
}

type RunSummary struct {
	RunID          string      `json:"run_id"`
	Workflow       string      `json:"workflow"`
	DefinitionHash string      `json:"definition_hash,omitempty"`
	State          rt.RunState `json:"state"`
	LastSeq        uint64      `json:"last_seq"`
	CreatedAt      time.Time   `json:"created_at"`
	FinishedAt     *time.Time  `json:"finished_at,omitempty"`
}

type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

type GraphNode struct {
	ID                string          `json:"id"`
	ActivityType      rt.ActivityType `json:"activity_type"`
	State             rt.NodeState    `json:"state"`
	DependsOn         []string        `json:"depends_on,omitempty"`
	Attempt           uint32          `json:"attempt,omitempty"`
	WorkerID          string          `json:"worker_id,omitempty"`
	LastError         string          `json:"last_error,omitempty"`
	CreatedSeq        uint64          `json:"created_seq,omitempty"`
	LastTransitionSeq uint64          `json:"last_transition_seq,omitempty"`
	Signals           []string        `json:"signals,omitempty"`
}

type PhaseMarker struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Active   bool   `json:"active"`
	Complete bool   `json:"complete"`
	NodeID   string `json:"node_id,omitempty"`
	Seq      uint64 `json:"seq,omitempty"`
}

type GraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind,omitempty"`
}

type Timeline struct {
	Events []TimelineEvent `json:"events"`
}

type Replay struct {
	Frames []ReplayFrame `json:"frames"`
}

type ReplayFrame struct {
	Seq      uint64          `json:"seq"`
	Label    string          `json:"label"`
	Type     rt.EventType    `json:"type"`
	RunState rt.RunState     `json:"run_state"`
	Graph    Graph           `json:"graph"`
	Phases   []PhaseMarker   `json:"phases"`
	Timeline []TimelineEvent `json:"timeline"`
}

type TimelineEvent struct {
	Seq       uint64       `json:"seq"`
	Type      rt.EventType `json:"type"`
	Timestamp time.Time    `json:"timestamp"`
	NodeID    string       `json:"node_id,omitempty"`
	Attempt   uint32       `json:"attempt,omitempty"`
	Family    string       `json:"family"`
	Label     string       `json:"label"`
}

type Cursor struct {
	AfterSeq   uint64 `json:"after_seq,omitempty"`
	CurrentSeq uint64 `json:"current_seq,omitempty"`
	HasChanges bool   `json:"has_changes"`
}

func (a *Adapter) Snapshot(runID string, afterSeq uint64) (Snapshot, error) {
	view, err := a.reader.GetRun(runID)
	if err != nil {
		return Snapshot{}, err
	}
	events, err := a.reader.ListEvents(runID)
	if err != nil {
		return Snapshot{}, err
	}

	nodeCreatedSeq, nodeLastTransitionSeq, nodeSignals, phaseMarkers, spawnEdges := analyzeEvents(events)
	timeline := make([]TimelineEvent, 0, len(events))
	delta := make([]TimelineEvent, 0)
	for _, event := range events {
		item := timelineEvent(event)
		timeline = append(timeline, item)
		if event.Seq > afterSeq {
			delta = append(delta, item)
		}
	}

	nodes := make([]GraphNode, 0, len(view.Nodes))
	for _, node := range sortedNodeViews(view.Nodes) {
		nodes = append(nodes, GraphNode{
			ID:                node.ID,
			ActivityType:      node.ActivityType,
			State:             node.State,
			DependsOn:         append([]string(nil), node.DependsOn...),
			Attempt:           node.Attempt,
			WorkerID:          node.WorkerID,
			LastError:         node.LastError,
			CreatedSeq:        nodeCreatedSeq[node.ID],
			LastTransitionSeq: nodeLastTransitionSeq[node.ID],
			Signals:           nodeSignals[node.ID],
		})
	}

	edges := make([]GraphEdge, 0, len(spawnEdges))
	for _, node := range nodes {
		for _, dep := range node.DependsOn {
			edges = append(edges, GraphEdge{From: dep, To: node.ID, Kind: "dependency"})
		}
	}
	edges = append(edges, spawnEdges...)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From == edges[j].From {
			if edges[i].To == edges[j].To {
				return edges[i].Kind < edges[j].Kind
			}
			return edges[i].To < edges[j].To
		}
		return edges[i].From < edges[j].From
	})

	return Snapshot{
		Run: RunSummary{
			RunID:          view.RunID,
			Workflow:       view.Workflow,
			DefinitionHash: view.DefinitionHash,
			State:          view.State,
			LastSeq:        view.LastSeq,
			CreatedAt:      view.CreatedAt,
			FinishedAt:     view.FinishedAt,
		},
		Graph: Graph{
			Nodes: nodes,
			Edges: edges,
		},
		Timeline: Timeline{Events: timeline},
		Phases:   phaseMarkers,
		Replay: Replay{
			Frames: buildReplayFrames(events),
		},
		Cursor: Cursor{
			AfterSeq:   afterSeq,
			CurrentSeq: view.LastSeq,
			HasChanges: view.LastSeq > afterSeq,
		},
		Delta: delta,
	}, nil
}

func buildReplayFrames(events []rt.Event) []ReplayFrame {
	if len(events) == 0 {
		return nil
	}
	execState := st.New()
	frames := make([]ReplayFrame, 0, len(events))
	for i, event := range events {
		if err := execState.Apply(event); err != nil {
			continue
		}
		prefix := events[:i+1]
		view := execState.View()
		graph, phases := graphAndPhasesFromViewAndEvents(view, prefix)
		timeline := make([]TimelineEvent, 0, len(prefix))
		for _, item := range prefix {
			timeline = append(timeline, timelineEvent(item))
		}
		frames = append(frames, ReplayFrame{
			Seq:      event.Seq,
			Label:    timelineEvent(event).Label,
			Type:     event.Type,
			RunState: view.State,
			Graph:    graph,
			Phases:   phases,
			Timeline: timeline,
		})
	}
	return frames
}

func graphAndPhasesFromViewAndEvents(view rt.RunView, events []rt.Event) (Graph, []PhaseMarker) {
	nodeCreatedSeq, nodeLastTransitionSeq, nodeSignals, phaseMarkers, spawnEdges := analyzeEvents(events)
	nodes := make([]GraphNode, 0, len(view.Nodes))
	for _, node := range sortedNodeViews(view.Nodes) {
		nodes = append(nodes, GraphNode{
			ID:                node.ID,
			ActivityType:      node.ActivityType,
			State:             node.State,
			DependsOn:         append([]string(nil), node.DependsOn...),
			Attempt:           node.Attempt,
			WorkerID:          node.WorkerID,
			LastError:         node.LastError,
			CreatedSeq:        nodeCreatedSeq[node.ID],
			LastTransitionSeq: nodeLastTransitionSeq[node.ID],
			Signals:           nodeSignals[node.ID],
		})
	}
	edges := make([]GraphEdge, 0, len(spawnEdges))
	for _, node := range nodes {
		for _, dep := range node.DependsOn {
			edges = append(edges, GraphEdge{From: dep, To: node.ID, Kind: "dependency"})
		}
	}
	edges = append(edges, spawnEdges...)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From == edges[j].From {
			if edges[i].To == edges[j].To {
				return edges[i].Kind < edges[j].Kind
			}
			return edges[i].To < edges[j].To
		}
		return edges[i].From < edges[j].From
	})
	return Graph{Nodes: nodes, Edges: edges}, phaseMarkers
}

func sortedNodeViews(nodes map[string]rt.NodeView) []rt.NodeView {
	list := make([]rt.NodeView, 0, len(nodes))
	for _, node := range nodes {
		list = append(list, node)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})
	return list
}

func analyzeEvents(events []rt.Event) (map[string]uint64, map[string]uint64, map[string][]string, []PhaseMarker, []GraphEdge) {
	created := map[string]uint64{}
	latest := map[string]uint64{}
	signals := map[string][]string{}
	seenSignals := map[string]map[string]struct{}{}
	spawnEdges := make([]GraphEdge, 0)
	seenSpawnEdges := map[string]struct{}{}
	phaseState := map[string]*PhaseMarker{
		"spawn":      {Key: "spawn", Label: "Dynamic spawn"},
		"retry":      {Key: "retry", Label: "Retry recovery"},
		"checkpoint": {Key: "checkpoint", Label: "Approval gate"},
		"complete":   {Key: "complete", Label: "Workflow completed"},
	}
	for _, event := range events {
		nodeIDs := eventNodeIDs(event)
		for _, nodeID := range nodeIDs {
			if _, ok := created[nodeID]; !ok {
				created[nodeID] = event.Seq
			}
			latest[nodeID] = event.Seq
		}
		switch payload := event.Payload.(type) {
		case rt.NodeCompletedPayload:
			if len(payload.SpawnedNodes) > 0 {
				phaseState["spawn"].Complete = true
				phaseState["spawn"].Seq = event.Seq
				phaseState["spawn"].NodeID = payload.NodeID
				for _, spawned := range payload.SpawnedNodes {
					addSignal(signals, seenSignals, spawned.ID, "spawned")
					if len(spawned.DependsOn) == 0 {
						edgeKey := payload.NodeID + "->" + spawned.ID
						if _, ok := seenSpawnEdges[edgeKey]; !ok {
							seenSpawnEdges[edgeKey] = struct{}{}
							spawnEdges = append(spawnEdges, GraphEdge{
								From: payload.NodeID,
								To:   spawned.ID,
								Kind: "spawn",
							})
						}
					}
				}
			}
		case rt.NodeFailedPayload:
			if payload.Retryable {
				addSignal(signals, seenSignals, payload.NodeID, "retrying")
				phaseState["retry"].Active = true
				phaseState["retry"].NodeID = payload.NodeID
				if phaseState["retry"].Seq == 0 {
					phaseState["retry"].Seq = event.Seq
				}
			}
			if payload.TimedOut {
				addSignal(signals, seenSignals, payload.NodeID, "timed_out")
			}
		case rt.TimerFiredPayload:
			if payload.Purpose == rt.TimerPurposeRetryBackoff {
				addSignal(signals, seenSignals, payload.NodeID, "retried")
				phaseState["retry"].Complete = true
				phaseState["retry"].Seq = event.Seq
				phaseState["retry"].NodeID = payload.NodeID
			}
		case rt.CheckpointReachedPayload:
			addSignal(signals, seenSignals, payload.NodeID, "checkpoint")
			phaseState["checkpoint"].Active = true
			phaseState["checkpoint"].NodeID = payload.NodeID
			if phaseState["checkpoint"].Seq == 0 {
				phaseState["checkpoint"].Seq = event.Seq
			}
		case rt.CheckpointApprovedPayload:
			addSignal(signals, seenSignals, payload.NodeID, "approved")
			phaseState["checkpoint"].Complete = true
			phaseState["checkpoint"].Seq = event.Seq
			phaseState["checkpoint"].NodeID = payload.NodeID
		}
		if event.Type == rt.EventWorkflowCompleted {
			phaseState["complete"].Complete = true
			phaseState["complete"].Seq = event.Seq
		}
	}
	phases := make([]PhaseMarker, 0, len(phaseState))
	for _, key := range []string{"spawn", "retry", "checkpoint", "complete"} {
		phase := phaseState[key]
		if phase.Active || phase.Complete {
			phases = append(phases, *phase)
		}
	}
	return created, latest, signals, phases, spawnEdges
}

func addSignal(signals map[string][]string, seen map[string]map[string]struct{}, nodeID, signal string) {
	if nodeID == "" || signal == "" {
		return
	}
	if _, ok := seen[nodeID]; !ok {
		seen[nodeID] = map[string]struct{}{}
	}
	if _, ok := seen[nodeID][signal]; ok {
		return
	}
	seen[nodeID][signal] = struct{}{}
	signals[nodeID] = append(signals[nodeID], signal)
}

func eventNodeIDs(event rt.Event) []string {
	switch payload := event.Payload.(type) {
	case rt.LeaseGrantedPayload:
		return []string{payload.NodeID}
	case rt.HeartbeatRecordedPayload:
		return []string{payload.NodeID}
	case rt.LeaseExpiredPayload:
		return []string{payload.NodeID}
	case rt.TimerScheduledPayload:
		return []string{payload.NodeID}
	case rt.TimerFiredPayload:
		return []string{payload.NodeID}
	case rt.NodeReadyPayload:
		return []string{payload.NodeID}
	case rt.NodeStartedPayload:
		return []string{payload.NodeID}
	case rt.NodeCancelledPayload:
		return []string{payload.NodeID}
	case rt.CheckpointReachedPayload:
		return []string{payload.NodeID}
	case rt.CheckpointApprovedPayload:
		return []string{payload.NodeID}
	case rt.NodeFailedPayload:
		return []string{payload.NodeID}
	case rt.CompensationTaskStartedPayload:
		return []string{payload.NodeID}
	case rt.CompensationTaskCompletedPayload:
		return []string{payload.NodeID}
	case rt.CompensationTaskExpiredPayload:
		return []string{payload.NodeID}
	case rt.CompensationTaskFailedPayload:
		return []string{payload.NodeID}
	case rt.NodeCompletedPayload:
		nodeIDs := []string{payload.NodeID}
		for _, spawned := range payload.SpawnedNodes {
			nodeIDs = append(nodeIDs, spawned.ID)
		}
		return nodeIDs
	default:
		return nil
	}
}

func timelineEvent(event rt.Event) TimelineEvent {
	nodeID, attempt := timelineFields(event)
	return TimelineEvent{
		Seq:       event.Seq,
		Type:      event.Type,
		Timestamp: event.Timestamp,
		NodeID:    nodeID,
		Attempt:   attempt,
		Family:    eventFamily(event.Type),
		Label:     timelineLabel(event),
	}
}

func timelineFields(event rt.Event) (string, uint32) {
	switch payload := event.Payload.(type) {
	case rt.LeaseGrantedPayload:
		return payload.NodeID, payload.Attempt
	case rt.HeartbeatRecordedPayload:
		return payload.NodeID, payload.Attempt
	case rt.LeaseExpiredPayload:
		return payload.NodeID, payload.Attempt
	case rt.TimerScheduledPayload:
		return payload.NodeID, payload.Attempt
	case rt.TimerFiredPayload:
		return payload.NodeID, payload.Attempt
	case rt.NodeStartedPayload:
		return payload.NodeID, payload.Attempt
	case rt.NodeCancelledPayload:
		return payload.NodeID, payload.Attempt
	case rt.CheckpointReachedPayload:
		return payload.NodeID, payload.Attempt
	case rt.NodeCompletedPayload:
		return payload.NodeID, payload.Attempt
	case rt.NodeFailedPayload:
		return payload.NodeID, payload.Attempt
	case rt.CompensationTaskStartedPayload:
		return payload.NodeID, payload.Attempt
	case rt.CompensationTaskCompletedPayload:
		return payload.NodeID, payload.Attempt
	case rt.CompensationTaskExpiredPayload:
		return payload.NodeID, payload.Attempt
	case rt.CompensationTaskFailedPayload:
		return payload.NodeID, payload.Attempt
	case rt.NodeReadyPayload:
		return payload.NodeID, 0
	case rt.CheckpointApprovedPayload:
		return payload.NodeID, 0
	default:
		return "", 0
	}
}

func eventFamily(eventType rt.EventType) string {
	switch eventType {
	case rt.EventWorkflowStarted, rt.EventWorkflowCompleted, rt.EventWorkflowFailed:
		return "run"
	case rt.EventLeaseGranted, rt.EventHeartbeatRecorded, rt.EventLeaseExpired:
		return "lease"
	case rt.EventTimerScheduled, rt.EventTimerFired:
		return "timer"
	case rt.EventCheckpointReached, rt.EventCheckpointApproved:
		return "checkpoint"
	case rt.EventCancellationRequested, rt.EventNodeCancelled, rt.EventCancellationCompleted:
		return "cancellation"
	case rt.EventCompensationStarted, rt.EventCompensationTaskStarted, rt.EventCompensationTaskCompleted, rt.EventCompensationTaskExpired, rt.EventCompensationTaskFailed, rt.EventCompensationCompleted:
		return "compensation"
	case rt.EventNodeReady, rt.EventNodeStarted, rt.EventNodeCompleted, rt.EventNodeFailed:
		return "node"
	default:
		return "other"
	}
}

func timelineLabel(event rt.Event) string {
	nodeID, attempt := timelineFields(event)
	switch event.Type {
	case rt.EventWorkflowStarted:
		return "workflow started"
	case rt.EventWorkflowCompleted:
		return "workflow completed"
	case rt.EventWorkflowFailed:
		return "workflow failed"
	case rt.EventCompensationCompleted:
		return "compensation completed"
	case rt.EventCompensationStarted:
		return "compensation started"
	}
	if nodeID == "" {
		return string(event.Type)
	}
	if attempt > 0 {
		return fmt.Sprintf("%s %s (attempt %d)", nodeID, event.Type, attempt)
	}
	return fmt.Sprintf("%s %s", nodeID, event.Type)
}
