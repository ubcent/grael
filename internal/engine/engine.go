package engine

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"grael/internal/processor"
	rt "grael/internal/runtime"
	"grael/internal/scheduler"
	"grael/internal/snapshot"
	"grael/internal/state"
	"grael/internal/wal"
)

var ErrRunNotFound = errors.New("engine: run not found")

type Engine struct {
	mu        sync.RWMutex
	wal       *wal.Store
	snapshots *snapshot.Store
	scheduler *scheduler.Scheduler
	processor *processor.Processor
	runs      map[string]*runEntry
}

type runEntry struct {
	state *state.ExecutionState
	done  chan struct{}
	once  sync.Once
}

func New(baseDir string) *Engine {
	store := wal.NewStore(baseDir)
	return &Engine{
		wal:       store,
		snapshots: snapshot.NewStore(baseDir),
		scheduler: scheduler.New(),
		processor: processor.New(store),
		runs:      map[string]*runEntry{},
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
	entry := &runEntry{
		state: st,
		done:  make(chan struct{}),
	}
	e.runs[runID] = entry
	e.mu.Unlock()

	// The engine returns after persisting the start event, then continues the run
	// in the background so current state can be observed while execution is active.
	go e.drive(runID, entry)
	return runID, nil
}

func (e *Engine) GetRun(runID string) (rt.RunView, error) {
	e.mu.RLock()
	entry, ok := e.runs[runID]
	if ok {
		view := entry.state.View()
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
	// If the run is not cached in memory, rebuild it from durable state and then
	// replay only the WAL tail beyond the snapshot boundary.
	rehydrated, err := e.rehydrate(runID, events)
	if err != nil {
		return rt.RunView{}, err
	}
	e.mu.Lock()
	e.runs[runID] = &runEntry{
		state: rehydrated,
		done:  closedChan(),
	}
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
	e.mu.RLock()
	entry, ok := e.runs[runID]
	e.mu.RUnlock()
	if !ok {
		return false, ErrRunNotFound
	}

	if timeout <= 0 {
		<-entry.done
		return true, nil
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-entry.done:
		return true, nil
	case <-timer.C:
		return false, nil
	}
}

func (e *Engine) SnapshotInfo(runID string) (snapshot.Info, error) {
	return e.snapshots.Info(runID)
}

func (e *Engine) drive(runID string, entry *runEntry) {
	defer entry.once.Do(func() { close(entry.done) })

	for {
		e.mu.Lock()
		current, ok := e.runs[runID]
		if !ok || current != entry {
			e.mu.Unlock()
			return
		}
		st := entry.state
		commands := e.scheduler.Decide(st)
		if len(commands) == 0 {
			_ = e.snapshots.Save(st)
			e.mu.Unlock()
			return
		}
		for _, cmd := range commands {
			events, err := e.processor.Execute(st, cmd)
			if err != nil {
				e.mu.Unlock()
				return
			}
			for _, event := range events {
				if err := st.Apply(event); err != nil {
					e.mu.Unlock()
					return
				}
			}
		}
		_ = e.snapshots.Save(st)
		e.mu.Unlock()
	}
}

func closedChan() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
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
