package api

import (
	"time"

	"grael/internal/engine"
	rt "grael/internal/runtime"
	"grael/internal/snapshot"
)

type Service struct {
	engine *engine.Engine
}

func New(baseDir string) *Service {
	return &Service{engine: engine.New(baseDir)}
}

func NewWithConfig(baseDir string, cfg engine.Config) *Service {
	return &Service{engine: engine.NewWithConfig(baseDir, cfg)}
}

func (s *Service) Close() {
	if s == nil || s.engine == nil {
		return
	}
	s.engine.Close()
}

func (s *Service) StartRun(def rt.WorkflowDefinition, input map[string]any) (string, error) {
	return s.engine.StartRun(def, input)
}

func (s *Service) RegisterWorker(workerID string, activities []rt.ActivityType) error {
	return s.engine.RegisterWorker(workerID, activities)
}

func (s *Service) PollTask(workerID string, timeout time.Duration) (rt.WorkerTask, bool, error) {
	return s.engine.PollTask(workerID, timeout)
}

func (s *Service) CompleteTask(req rt.CompleteTaskRequest) error {
	return s.engine.CompleteTask(req)
}

func (s *Service) FailTask(req rt.FailTaskRequest) error {
	return s.engine.FailTask(req)
}

func (s *Service) Heartbeat(workerID string) error {
	return s.engine.Heartbeat(workerID)
}

func (s *Service) ApproveCheckpoint(runID, nodeID string) error {
	return s.engine.ApproveCheckpoint(runID, nodeID)
}

func (s *Service) CancelRun(runID string) error {
	return s.engine.CancelRun(runID)
}

func (s *Service) GetRun(runID string) (rt.RunView, error) {
	return s.engine.GetRun(runID)
}

func (s *Service) ListEvents(runID string) ([]rt.Event, error) {
	return s.engine.ListEvents(runID)
}

func (s *Service) WaitForQuiescence(runID string, timeout time.Duration) (bool, error) {
	return s.engine.WaitForQuiescence(runID, timeout)
}

func (s *Service) SnapshotInfo(runID string) (snapshot.Info, error) {
	return s.engine.SnapshotInfo(runID)
}
