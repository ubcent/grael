package worker

import (
	"fmt"
	"slices"
	"sync"
	"time"

	rt "grael/internal/runtime"
)

type Registration struct {
	WorkerID   string
	Activities []rt.ActivityType
	LastSeenAt time.Time
}

type Registry struct {
	mu      sync.RWMutex
	workers map[string]Registration
}

func NewRegistry() *Registry {
	return &Registry{
		workers: map[string]Registration{},
	}
}

func (r *Registry) Register(workerID string, activities []rt.ActivityType) error {
	if workerID == "" {
		return fmt.Errorf("worker registry: worker_id is required")
	}
	if len(activities) == 0 {
		return fmt.Errorf("worker registry: at least one activity is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.workers[workerID] = Registration{
		WorkerID:   workerID,
		Activities: slices.Clone(activities),
		LastSeenAt: time.Now().UTC(),
	}
	return nil
}

func (r *Registry) Heartbeat(workerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	reg, ok := r.workers[workerID]
	if !ok {
		return fmt.Errorf("worker registry: unknown worker %q", workerID)
	}
	reg.LastSeenAt = time.Now().UTC()
	r.workers[workerID] = reg
	return nil
}

func (r *Registry) CanHandle(workerID string, activity rt.ActivityType) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	reg, ok := r.workers[workerID]
	if !ok {
		return false
	}
	return slices.Contains(reg.Activities, activity)
}

func (r *Registry) LastSeen(workerID string) (time.Time, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	reg, ok := r.workers[workerID]
	if !ok {
		return time.Time{}, false
	}
	return reg.LastSeenAt, true
}
