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
)

var (
	ErrRunNotFound       = errors.New("engine: run not found")
	ErrWorkerUnavailable = errors.New("engine: worker is not registered for this activity type")
	ErrTaskNotFound      = errors.New("engine: task not found")
	ErrAttemptMismatch   = errors.New("engine: attempt does not match active lease")
)

type Engine struct {
	mu        sync.RWMutex
	wal       *wal.Store
	snapshots *snapshot.Store
	workers   *worker.Registry
	runs      map[string]*state.ExecutionState
}

func New(baseDir string) *Engine {
	return &Engine{
		wal:       wal.NewStore(baseDir),
		snapshots: snapshot.NewStore(baseDir),
		workers:   worker.NewRegistry(),
		runs:      map[string]*state.ExecutionState{},
	}
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
	if node.State != rt.NodeStateRunning || node.WorkerID != req.WorkerID || node.Attempt != req.Attempt {
		return ErrAttemptMismatch
	}

	event, err := e.wal.Append(rt.Event{
		RunID:     req.RunID,
		Type:      rt.EventNodeCompleted,
		Timestamp: time.Now().UTC(),
		Payload: rt.NodeCompletedPayload{
			NodeID:   req.NodeID,
			WorkerID: req.WorkerID,
			Attempt:  req.Attempt,
			Output:   req.Output,
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
	if node.State != rt.NodeStateRunning || node.WorkerID != req.WorkerID || node.Attempt != req.Attempt {
		return ErrAttemptMismatch
	}

	event, err := e.wal.Append(rt.Event{
		RunID:     req.RunID,
		Type:      rt.EventNodeFailed,
		Timestamp: time.Now().UTC(),
		Payload: rt.NodeFailedPayload{
			NodeID:   req.NodeID,
			WorkerID: req.WorkerID,
			Attempt:  req.Attempt,
			Message:  req.Message,
		},
	})
	if err != nil {
		return err
	}
	if err := st.Apply(event); err != nil {
		return err
	}
	return e.snapshots.Save(st)
}

func (e *Engine) Heartbeat(workerID string) error {
	return e.workers.Heartbeat(workerID)
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
