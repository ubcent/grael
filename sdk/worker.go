package sdk

import (
	"context"
	"errors"
	"fmt"
	"time"

	"grael/internal/api"
	rt "grael/internal/runtime"
)

type Client interface {
	RegisterWorker(workerID string, activities []rt.ActivityType) error
	PollTask(workerID string, timeout time.Duration) (rt.WorkerTask, bool, error)
	CompleteTask(req rt.CompleteTaskRequest) error
	FailTask(req rt.FailTaskRequest) error
}

type Worker struct {
	client      Client
	workerID    string
	pollTimeout time.Duration
	handlers    map[rt.ActivityType]Handler
}

type Handler func(context.Context, Task) (Result, error)

type Task struct {
	rt.WorkerTask
}

type Result struct {
	Output       map[string]any
	SpawnedNodes []rt.NodeDefinition
	Checkpoint   *rt.CheckpointRequest
}

type TaskError struct {
	Message   string
	Retryable bool
	Cancelled bool
}

func (e *TaskError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func NewWorker(client Client, workerID string) *Worker {
	return &Worker{
		client:      client,
		workerID:    workerID,
		pollTimeout: 50 * time.Millisecond,
		handlers:    map[rt.ActivityType]Handler{},
	}
}

func NewServiceClient(svc *api.Service) Client {
	return svc
}

func (w *Worker) Handle(activity rt.ActivityType, handler Handler) {
	w.handlers[activity] = handler
}

func (w *Worker) SetPollTimeout(timeout time.Duration) {
	if timeout > 0 {
		w.pollTimeout = timeout
	}
}

func (w *Worker) Run(ctx context.Context) error {
	if len(w.handlers) == 0 {
		return errors.New("sdk: no handlers registered")
	}

	activities := make([]rt.ActivityType, 0, len(w.handlers))
	for activity := range w.handlers {
		activities = append(activities, activity)
	}
	if err := w.client.RegisterWorker(w.workerID, activities); err != nil {
		return err
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}

		task, ok, err := w.client.PollTask(w.workerID, w.pollTimeout)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		handler, ok := w.handlers[task.ActivityType]
		if !ok {
			return fmt.Errorf("sdk: no handler registered for activity %q", task.ActivityType)
		}
		result, err := handler(ctx, Task{WorkerTask: task})
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, ctxErr) {
				// Worker shutdown must not invent a task failure; the lease expiry
				// path already provides the honest recovery mechanism.
				return nil
			}
			taskErr := &TaskError{Message: err.Error()}
			if errors.As(err, &taskErr) {
				err = w.client.FailTask(rt.FailTaskRequest{
					WorkerID:     w.workerID,
					RunID:        task.RunID,
					NodeID:       task.NodeID,
					Attempt:      task.Attempt,
					Compensation: task.Compensation,
					Message:      taskErr.Message,
					Cancelled:    taskErr.Cancelled,
					Retryable:    taskErr.Retryable,
				})
			} else {
				err = w.client.FailTask(rt.FailTaskRequest{
					WorkerID:     w.workerID,
					RunID:        task.RunID,
					NodeID:       task.NodeID,
					Attempt:      task.Attempt,
					Compensation: task.Compensation,
					Message:      err.Error(),
				})
			}
			if err != nil {
				return err
			}
			continue
		}

		if err := w.client.CompleteTask(rt.CompleteTaskRequest{
			WorkerID:     w.workerID,
			RunID:        task.RunID,
			NodeID:       task.NodeID,
			Attempt:      task.Attempt,
			Compensation: task.Compensation,
			Output:       result.Output,
			Checkpoint:   result.Checkpoint,
			SpawnedNodes: result.SpawnedNodes,
		}); err != nil {
			return err
		}
	}
}
