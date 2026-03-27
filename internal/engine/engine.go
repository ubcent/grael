package engine

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	rt "grael/internal/runtime"
	"grael/internal/snapshot"
	"grael/internal/state"
	"grael/internal/wal"
	"grael/internal/worker"
	"grael/internal/workflowdef"
)

var (
	ErrRunNotFound       = errors.New("engine: run not found")
	ErrWorkerUnavailable = errors.New("engine: worker is not registered for this activity type")
	ErrTaskNotFound      = errors.New("engine: task not found")
	ErrAttemptMismatch   = errors.New("engine: attempt does not match active lease")
	ErrLeaseExpired      = errors.New("engine: lease expired")
)

const (
	defaultHeartbeatTimeout = 150 * time.Millisecond
	leaseMonitorInterval    = 25 * time.Millisecond
	timerMonitorInterval    = 25 * time.Millisecond
)

type Engine struct {
	mu        sync.RWMutex
	wal       *wal.Store
	snapshots *snapshot.Store
	workers   *worker.Registry
	runs      map[string]*state.ExecutionState
}

func New(baseDir string) *Engine {
	e := &Engine{
		wal:       wal.NewStore(baseDir),
		snapshots: snapshot.NewStore(baseDir),
		workers:   worker.NewRegistry(),
		runs:      map[string]*state.ExecutionState{},
	}
	e.loadPersistedRuns()
	go e.runLeaseMonitor()
	go e.runTimerMonitor()
	return e
}

func (e *Engine) StartRun(def rt.WorkflowDefinition, input map[string]any) (string, error) {
	runID := fmt.Sprintf("%s-%d", def.Name, time.Now().UTC().UnixNano())
	event, err := e.wal.Append(rt.Event{
		RunID:     runID,
		Type:      rt.EventWorkflowStarted,
		Timestamp: time.Now().UTC(),
		Payload: rt.WorkflowStartedPayload{
			Workflow: def,
			Input:    input,
		},
	})
	if err != nil {
		return "", err
	}

	st := state.New()
	if err := st.Apply(event); err != nil {
		return "", err
	}

	e.mu.Lock()
	e.runs[runID] = st
	e.mu.Unlock()

	_ = e.snapshots.Save(st)
	return runID, nil
}

func (e *Engine) RegisterWorker(workerID string, activities []rt.ActivityType) error {
	return e.workers.Register(workerID, activities)
}

func (e *Engine) PollTask(workerID string, timeout time.Duration) (rt.WorkerTask, bool, error) {
	if timeout < 0 {
		timeout = 0
	}
	if err := e.Heartbeat(workerID); err != nil {
		return rt.WorkerTask{}, false, err
	}

	deadline := time.Now().Add(timeout)
	for {
		task, ok, err := e.tryClaimTask(workerID)
		if err != nil || ok || timeout == 0 {
			return task, ok, err
		}
		if time.Now().After(deadline) {
			return rt.WorkerTask{}, false, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (e *Engine) CompleteTask(req rt.CompleteTaskRequest) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	st, err := e.loadStateLocked(req.RunID)
	if err != nil {
		return err
	}

	node := st.Nodes[req.NodeID]
	if node == nil {
		return ErrTaskNotFound
	}
	if node.Attempt == req.Attempt && node.State != rt.NodeStateRunning {
		return ErrLeaseExpired
	}
	if node.State != rt.NodeStateRunning || node.WorkerID != req.WorkerID || node.Attempt != req.Attempt {
		return ErrAttemptMismatch
	}
	normalizedSpawned, err := normalizeSpawnedNodes(st, req.NodeID, req.SpawnedNodes)
	if err != nil {
		return e.failNodeForInvalidSpawnLocked(st, req, err)
	}

	event, err := e.wal.Append(rt.Event{
		RunID:     req.RunID,
		Type:      rt.EventNodeCompleted,
		Timestamp: time.Now().UTC(),
		Payload: rt.NodeCompletedPayload{
			NodeID:       req.NodeID,
			WorkerID:     req.WorkerID,
			Attempt:      req.Attempt,
			Output:       req.Output,
			SpawnedNodes: normalizedSpawned,
		},
	})
	if err != nil {
		return err
	}
	if err := st.Apply(event); err != nil {
		return err
	}
	if err := e.completeWorkflowIfTerminalLocked(st); err != nil {
		return err
	}
	if err := e.failWorkflowIfTerminalLocked(st); err != nil {
		return err
	}
	return e.snapshots.Save(st)
}

func (e *Engine) FailTask(req rt.FailTaskRequest) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	st, err := e.loadStateLocked(req.RunID)
	if err != nil {
		return err
	}

	node := st.Nodes[req.NodeID]
	if node == nil {
		return ErrTaskNotFound
	}
	if node.Attempt == req.Attempt && node.State != rt.NodeStateRunning {
		return ErrLeaseExpired
	}
	if node.State != rt.NodeStateRunning || node.WorkerID != req.WorkerID || node.Attempt != req.Attempt {
		return ErrAttemptMismatch
	}

	event, err := e.wal.Append(rt.Event{
		RunID:     req.RunID,
		Type:      rt.EventNodeFailed,
		Timestamp: time.Now().UTC(),
		Payload: rt.NodeFailedPayload{
			NodeID:    req.NodeID,
			WorkerID:  req.WorkerID,
			Attempt:   req.Attempt,
			Message:   req.Message,
			Retryable: req.Retryable,
		},
	})
	if err != nil {
		return err
	}
	if err := st.Apply(event); err != nil {
		return err
	}
	retryScheduled := false
	if req.Retryable {
		if err := e.scheduleRetryTimerLocked(st, node, req); err != nil {
			return err
		}
		retryScheduled = hasPendingRetryTimer(st, node.ID, req.Attempt)
	}
	if !retryScheduled {
		if err := e.failWorkflowIfTerminalLocked(st); err != nil {
			return err
		}
	}
	return e.snapshots.Save(st)
}

func (e *Engine) Heartbeat(workerID string) error {
	if err := e.workers.Heartbeat(workerID); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now().UTC()
	for runID := range e.runs {
		st, err := e.loadStateLocked(runID)
		if err != nil || st.IsTerminal() {
			continue
		}
		changed := false
		for _, node := range st.Nodes {
			if node.State != rt.NodeStateRunning || node.WorkerID != workerID {
				continue
			}
			event, err := e.wal.Append(rt.Event{
				RunID:     st.RunID,
				Type:      rt.EventHeartbeatRecorded,
				Timestamp: now,
				Payload: rt.HeartbeatRecordedPayload{
					NodeID:   node.ID,
					WorkerID: workerID,
					Attempt:  node.Attempt,
				},
			})
			if err != nil {
				continue
			}
			if err := st.Apply(event); err != nil {
				continue
			}
			changed = true
		}
		if changed {
			_ = e.snapshots.Save(st)
		}
	}
	return nil
}

func (e *Engine) GetRun(runID string) (rt.RunView, error) {
	e.mu.RLock()
	st, ok := e.runs[runID]
	if ok {
		view := st.View()
		e.mu.RUnlock()
		return view, nil
	}
	e.mu.RUnlock()

	events, err := e.wal.List(runID)
	if err != nil {
		return rt.RunView{}, err
	}
	if len(events) == 0 {
		return rt.RunView{}, ErrRunNotFound
	}

	rehydrated, err := e.rehydrate(runID, events)
	if err != nil {
		return rt.RunView{}, err
	}

	e.mu.Lock()
	e.runs[runID] = rehydrated
	e.mu.Unlock()
	return rehydrated.View(), nil
}

func (e *Engine) ListEvents(runID string) ([]rt.Event, error) {
	events, err := e.wal.List(runID)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, ErrRunNotFound
	}
	return events, nil
}

func (e *Engine) WaitForQuiescence(runID string, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for {
		view, err := e.GetRun(runID)
		if err != nil {
			return false, err
		}
		if view.State != rt.RunStateRunning || !hasReadyNode(view) {
			return true, nil
		}
		if timeout > 0 && time.Now().After(deadline) {
			return false, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (e *Engine) SnapshotInfo(runID string) (snapshot.Info, error) {
	return e.snapshots.Info(runID)
}

func (e *Engine) tryClaimTask(workerID string) (rt.WorkerTask, bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	runIDs := make([]string, 0, len(e.runs))
	for runID := range e.runs {
		runIDs = append(runIDs, runID)
	}
	slices.Sort(runIDs)

	for _, runID := range runIDs {
		st, err := e.loadStateLocked(runID)
		if err != nil {
			return rt.WorkerTask{}, false, err
		}
		task, ok, err := e.claimReadyTaskLocked(st, workerID)
		if err != nil {
			return rt.WorkerTask{}, false, err
		}
		if ok {
			return task, true, nil
		}
	}

	return rt.WorkerTask{}, false, nil
}

func (e *Engine) claimReadyTaskLocked(st *state.ExecutionState, workerID string) (rt.WorkerTask, bool, error) {
	if st.IsTerminal() {
		return rt.WorkerTask{}, false, nil
	}

	nodeIDs := make([]string, 0, len(st.Nodes))
	for nodeID := range st.Nodes {
		nodeIDs = append(nodeIDs, nodeID)
	}
	slices.Sort(nodeIDs)

	for _, nodeID := range nodeIDs {
		node := st.Nodes[nodeID]
		if node.State != rt.NodeStateReady {
			continue
		}
		if !e.workers.CanHandle(workerID, node.ActivityType) {
			continue
		}

		attempt := node.Attempt + 1
		events := []rt.Event{
			{
				RunID:     st.RunID,
				Type:      rt.EventLeaseGranted,
				Timestamp: time.Now().UTC(),
				Payload: rt.LeaseGrantedPayload{
					NodeID:   node.ID,
					WorkerID: workerID,
					Attempt:  attempt,
					Activity: string(node.ActivityType),
				},
			},
			{
				RunID:     st.RunID,
				Type:      rt.EventNodeStarted,
				Timestamp: time.Now().UTC(),
				Payload: rt.NodeStartedPayload{
					NodeID:   node.ID,
					WorkerID: workerID,
					Attempt:  attempt,
				},
			},
		}

		for i := range events {
			appended, err := e.wal.Append(events[i])
			if err != nil {
				return rt.WorkerTask{}, false, err
			}
			if err := st.Apply(appended); err != nil {
				return rt.WorkerTask{}, false, err
			}
		}
		if err := e.scheduleExecutionDeadlineTimerLocked(st, node, workerID, attempt); err != nil {
			return rt.WorkerTask{}, false, err
		}
		if err := e.scheduleAbsoluteDeadlineTimerLocked(st, node, workerID, attempt); err != nil {
			return rt.WorkerTask{}, false, err
		}
		if err := e.snapshots.Save(st); err != nil {
			return rt.WorkerTask{}, false, err
		}

		return rt.WorkerTask{
			RunID:         st.RunID,
			NodeID:        node.ID,
			ActivityType:  node.ActivityType,
			Attempt:       attempt,
			Workflow:      st.Workflow,
			WorkflowInput: st.Input,
		}, true, nil
	}

	return rt.WorkerTask{}, false, nil
}

func (e *Engine) completeWorkflowIfTerminalLocked(st *state.ExecutionState) error {
	if st.IsTerminal() || !allNodesCompleted(st) {
		return nil
	}

	event, err := e.wal.Append(rt.Event{
		RunID:     st.RunID,
		Type:      rt.EventWorkflowCompleted,
		Timestamp: time.Now().UTC(),
		Payload:   rt.WorkflowCompletedPayload{},
	})
	if err != nil {
		return err
	}
	return st.Apply(event)
}

func (e *Engine) failWorkflowIfTerminalLocked(st *state.ExecutionState) error {
	if st.IsTerminal() || !anyNodeFailed(st) {
		return nil
	}

	event, err := e.wal.Append(rt.Event{
		RunID:     st.RunID,
		Type:      rt.EventWorkflowFailed,
		Timestamp: time.Now().UTC(),
		Payload: rt.WorkflowFailedPayload{
			Reason: "node failed",
		},
	})
	if err != nil {
		return err
	}
	return st.Apply(event)
}

func (e *Engine) failNodeForInvalidSpawnLocked(st *state.ExecutionState, req rt.CompleteTaskRequest, validationErr error) error {
	event, err := e.wal.Append(rt.Event{
		RunID:     req.RunID,
		Type:      rt.EventNodeFailed,
		Timestamp: time.Now().UTC(),
		Payload: rt.NodeFailedPayload{
			NodeID:    req.NodeID,
			WorkerID:  req.WorkerID,
			Attempt:   req.Attempt,
			Message:   fmt.Sprintf("invalid spawned graph: %v", validationErr),
			Retryable: false,
		},
	})
	if err != nil {
		return err
	}
	if err := st.Apply(event); err != nil {
		return err
	}
	if err := e.failWorkflowIfTerminalLocked(st); err != nil {
		return err
	}
	if err := e.snapshots.Save(st); err != nil {
		return err
	}
	return validationErr
}

func allNodesCompleted(st *state.ExecutionState) bool {
	if len(st.Nodes) == 0 {
		return false
	}
	for _, node := range st.Nodes {
		if node.State != rt.NodeStateCompleted {
			return false
		}
	}
	return true
}

func hasReadyNode(view rt.RunView) bool {
	for _, node := range view.Nodes {
		if node.State == rt.NodeStateReady {
			return true
		}
	}
	return false
}

func anyNodeFailed(st *state.ExecutionState) bool {
	for _, node := range st.Nodes {
		if node.State == rt.NodeStateFailed {
			return true
		}
	}
	return false
}

func (e *Engine) loadStateLocked(runID string) (*state.ExecutionState, error) {
	if st, ok := e.runs[runID]; ok {
		return st, nil
	}

	events, err := e.wal.List(runID)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, ErrRunNotFound
	}

	st, err := e.rehydrate(runID, events)
	if err != nil {
		return nil, err
	}
	e.runs[runID] = st
	return st, nil
}

func (e *Engine) rehydrate(runID string, events []rt.Event) (*state.ExecutionState, error) {
	snapState, ok, err := e.snapshots.Load(runID)
	if err != nil && !errors.Is(err, snapshot.ErrCorruptSnapshot) {
		return nil, err
	}
	if ok && err == nil {
		for _, event := range events {
			if event.Seq <= snapState.LastSeq {
				continue
			}
			if err := snapState.Apply(event); err != nil {
				return nil, err
			}
		}
		return snapState, nil
	}
	return state.Rehydrate(events)
}

func (e *Engine) runLeaseMonitor() {
	ticker := time.NewTicker(leaseMonitorInterval)
	defer ticker.Stop()

	for range ticker.C {
		e.expireOverdueLeases()
	}
}

func (e *Engine) runTimerMonitor() {
	ticker := time.NewTicker(timerMonitorInterval)
	defer ticker.Stop()

	for range ticker.C {
		e.fireDueTimers()
	}
}

func (e *Engine) expireOverdueLeases() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now().UTC()
	for runID := range e.runs {
		st, err := e.loadStateLocked(runID)
		if err != nil {
			continue
		}
		changed := false
		for _, node := range st.Nodes {
			if node.State != rt.NodeStateRunning || node.WorkerID == "" {
				continue
			}
			if !node.LastHeartbeatAt.IsZero() && now.Sub(node.LastHeartbeatAt) <= defaultHeartbeatTimeout {
				continue
			}

			event, err := e.wal.Append(rt.Event{
				RunID:     st.RunID,
				Type:      rt.EventLeaseExpired,
				Timestamp: now,
				Payload: rt.LeaseExpiredPayload{
					NodeID:   node.ID,
					WorkerID: node.WorkerID,
					Attempt:  node.Attempt,
				},
			})
			if err != nil {
				continue
			}
			if err := st.Apply(event); err != nil {
				continue
			}
			changed = true
		}
		if changed {
			_ = e.snapshots.Save(st)
		}
	}
}

func (e *Engine) fireDueTimers() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now().UTC()
	for runID := range e.runs {
		st, err := e.loadStateLocked(runID)
		if err != nil {
			continue
		}
		changed := false
		for _, timer := range st.Timers {
			if st.IsTerminal() || timer.Fired || timer.FireAt.After(now) {
				continue
			}
			event, err := e.wal.Append(rt.Event{
				RunID:     st.RunID,
				Type:      rt.EventTimerFired,
				Timestamp: now,
				Payload: rt.TimerFiredPayload{
					TimerID: timer.ID,
					NodeID:  timer.NodeID,
					Attempt: timer.Attempt,
					Purpose: timer.Purpose,
				},
			})
			if err != nil {
				continue
			}
			if err := st.Apply(event); err != nil {
				continue
			}
			if timer.Purpose == rt.TimerPurposeNodeExecDeadline || timer.Purpose == rt.TimerPurposeNodeAbsDeadline {
				node := st.Nodes[timer.NodeID]
				if node != nil && !isTerminalNode(node.State) && node.Attempt == timer.Attempt {
					failEvent, err := e.wal.Append(rt.Event{
						RunID:     st.RunID,
						Type:      rt.EventNodeFailed,
						Timestamp: now,
						Payload: rt.NodeFailedPayload{
							NodeID:    node.ID,
							WorkerID:  node.WorkerID,
							Attempt:   node.Attempt,
							Message:   timeoutMessageFor(timer.Purpose),
							TimedOut:  true,
							Retryable: false,
						},
					})
					if err == nil {
						if err := st.Apply(failEvent); err == nil {
							_ = e.failWorkflowIfTerminalLocked(st)
							changed = true
						}
					}
				}
			}
			changed = true
		}
		if changed {
			_ = e.snapshots.Save(st)
		}
	}
}

func (e *Engine) scheduleExecutionDeadlineTimerLocked(st *state.ExecutionState, node *state.Node, workerID string, attempt uint32) error {
	if node.ExecutionDeadline <= 0 {
		return nil
	}

	now := time.Now().UTC()
	timerID := fmt.Sprintf("%s:%s:%d:%s", st.RunID, node.ID, attempt, rt.TimerPurposeNodeExecDeadline)
	event, err := e.wal.Append(rt.Event{
		RunID:     st.RunID,
		Type:      rt.EventTimerScheduled,
		Timestamp: now,
		Payload: rt.TimerScheduledPayload{
			TimerID:  timerID,
			NodeID:   node.ID,
			Attempt:  attempt,
			Purpose:  rt.TimerPurposeNodeExecDeadline,
			FireAt:   now.Add(node.ExecutionDeadline),
			WorkerID: workerID,
		},
	})
	if err != nil {
		return err
	}
	return st.Apply(event)
}

func (e *Engine) scheduleAbsoluteDeadlineTimerLocked(st *state.ExecutionState, node *state.Node, workerID string, attempt uint32) error {
	if node.AbsoluteDeadline <= 0 {
		return nil
	}

	now := time.Now().UTC()
	timerID := fmt.Sprintf("%s:%s:%d:%s", st.RunID, node.ID, attempt, rt.TimerPurposeNodeAbsDeadline)
	event, err := e.wal.Append(rt.Event{
		RunID:     st.RunID,
		Type:      rt.EventTimerScheduled,
		Timestamp: now,
		Payload: rt.TimerScheduledPayload{
			TimerID:  timerID,
			NodeID:   node.ID,
			Attempt:  attempt,
			Purpose:  rt.TimerPurposeNodeAbsDeadline,
			FireAt:   now.Add(node.AbsoluteDeadline),
			WorkerID: workerID,
		},
	})
	if err != nil {
		return err
	}
	return st.Apply(event)
}

func (e *Engine) scheduleRetryTimerLocked(st *state.ExecutionState, node *state.Node, req rt.FailTaskRequest) error {
	if node.RetryPolicy == nil || node.RetryPolicy.MaxAttempts <= 1 {
		return nil
	}
	if int(req.Attempt) >= node.RetryPolicy.MaxAttempts {
		return nil
	}

	fireAt := time.Now().UTC().Add(node.RetryPolicy.Backoff)
	timerID := fmt.Sprintf("%s:%s:%d:%s", st.RunID, node.ID, req.Attempt, rt.TimerPurposeRetryBackoff)
	event, err := e.wal.Append(rt.Event{
		RunID:     st.RunID,
		Type:      rt.EventTimerScheduled,
		Timestamp: time.Now().UTC(),
		Payload: rt.TimerScheduledPayload{
			TimerID:  timerID,
			NodeID:   node.ID,
			Attempt:  req.Attempt,
			Purpose:  rt.TimerPurposeRetryBackoff,
			FireAt:   fireAt,
			WorkerID: req.WorkerID,
		},
	})
	if err != nil {
		return err
	}
	return st.Apply(event)
}

func (e *Engine) loadPersistedRuns() {
	runIDs, err := e.wal.RunIDs()
	if err != nil {
		return
	}
	for _, runID := range runIDs {
		events, err := e.wal.List(runID)
		if err != nil || len(events) == 0 {
			continue
		}
		st, err := e.rehydrate(runID, events)
		if err != nil {
			continue
		}
		e.runs[runID] = st
	}
}

func hasPendingRetryTimer(st *state.ExecutionState, nodeID string, attempt uint32) bool {
	for _, timer := range st.Timers {
		if timer.NodeID == nodeID && timer.Attempt == attempt && timer.Purpose == rt.TimerPurposeRetryBackoff && !timer.Fired {
			return true
		}
	}
	return false
}

func isTerminalNode(nodeState rt.NodeState) bool {
	return nodeState == rt.NodeStateCompleted || nodeState == rt.NodeStateFailed
}

func timeoutMessageFor(purpose rt.TimerPurpose) string {
	switch purpose {
	case rt.TimerPurposeNodeAbsDeadline:
		return "absolute deadline exceeded"
	default:
		return "execution deadline exceeded"
	}
}

func validateSpawnedNodes(st *state.ExecutionState, parentNodeID string, spawned []rt.NodeDefinition) error {
	_, err := normalizeSpawnedNodes(st, parentNodeID, spawned)
	return err
}

func normalizeSpawnedNodes(st *state.ExecutionState, parentNodeID string, spawned []rt.NodeDefinition) ([]rt.NodeDefinition, error) {
	if len(spawned) == 0 {
		return nil, nil
	}

	normalizedDef, err := workflowdef.Normalize(rt.WorkflowDefinition{
		Name:  "spawn-validation",
		Nodes: spawned,
	})
	if err != nil {
		return nil, err
	}

	normalizedSpawned := make([]rt.NodeDefinition, 0, len(normalizedDef.Nodes))
	spawnedIDs := make(map[string]struct{}, len(spawned))
	for _, def := range normalizedDef.Nodes {
		if def.ID == "" {
			return nil, fmt.Errorf("spawned node id is required")
		}
		if def.ActivityType == "" {
			return nil, fmt.Errorf("spawned node %q must define activity_type", def.ID)
		}
		if _, exists := st.Nodes[def.ID]; exists {
			return nil, fmt.Errorf("spawned node %q already exists", def.ID)
		}
		if _, exists := spawnedIDs[def.ID]; exists {
			return nil, fmt.Errorf("duplicate spawned node id %q", def.ID)
		}
		spawnedIDs[def.ID] = struct{}{}
		normalizedSpawned = append(normalizedSpawned, def)
	}

	graph := make(map[string][]string, len(st.Nodes)+len(normalizedSpawned))
	for id, node := range st.Nodes {
		graph[id] = slices.Clone(node.DependsOn)
	}
	for _, def := range normalizedSpawned {
		for _, dep := range def.DependsOn {
			if dep == def.ID {
				return nil, fmt.Errorf("spawned node %q cannot depend on itself", def.ID)
			}
			if _, ok := st.Nodes[dep]; !ok {
				if _, ok := spawnedIDs[dep]; !ok {
					return nil, fmt.Errorf("spawned node %q depends on unknown node %q", def.ID, dep)
				}
			}
		}
		graph[def.ID] = slices.Clone(def.DependsOn)
	}

	if hasCycle(graph) {
		return nil, fmt.Errorf("spawn from %q would create a cycle", parentNodeID)
	}
	return normalizedSpawned, nil
}

func hasCycle(graph map[string][]string) bool {
	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)

	stateByNode := make(map[string]int, len(graph))
	var visit func(string) bool
	visit = func(nodeID string) bool {
		switch stateByNode[nodeID] {
		case visiting:
			return true
		case visited:
			return false
		}
		stateByNode[nodeID] = visiting
		for _, dep := range graph[nodeID] {
			if _, ok := graph[dep]; ok && visit(dep) {
				return true
			}
		}
		stateByNode[nodeID] = visited
		return false
	}

	for nodeID := range graph {
		if visit(nodeID) {
			return true
		}
	}
	return false
}
