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

func (s *Service) StartRun(def rt.WorkflowDefinition, input map[string]any) (string, error) {
	return s.engine.StartRun(def, input)
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
