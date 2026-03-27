package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"
	"time"

	"grael/internal/api"
	rt "grael/internal/runtime"
	"grael/internal/workflowdef"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "start":
		return startRun(args[1:])
	case "status":
		return getRun(args[1:])
	case "events":
		return listEvents(args[1:])
	case "snapshot":
		return snapshotInfo(args[1:])
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func startRun(args []string) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	dataDir := fs.String("data-dir", ".grael-data", "directory for Grael WAL data")
	workflowFile := fs.String("workflow", "", "path to workflow JSON file")
	example := fs.String("example", "", "name of built-in example workflow")
	demoWorker := fs.Bool("demo-worker", false, "start an in-process demo worker for the workflow activity types")
	waitTimeout := fs.Duration("wait-timeout", 2*time.Second, "maximum time to wait for the initial run pass before exiting")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *workflowFile == "" && *example == "" {
		return errors.New("missing required -workflow or -example")
	}
	if *workflowFile != "" && *example != "" {
		return errors.New("use only one of -workflow or -example")
	}

	var (
		def rt.WorkflowDefinition
		err error
	)
	if *example != "" {
		def, err = workflowdef.BuiltIn(*example)
	} else {
		def, err = workflowdef.LoadJSON(*workflowFile)
	}
	if err != nil {
		return err
	}

	svc := api.New(*dataDir)
	runID, err := svc.StartRun(def, nil)
	if err != nil {
		return err
	}
	if *demoWorker {
		if err := startDemoWorker(svc, runID, def); err != nil {
			return err
		}
	}
	if *demoWorker {
		if err := waitForTerminalRun(svc, runID, *waitTimeout); err != nil {
			return err
		}
	} else {
		if _, err := svc.WaitForQuiescence(runID, *waitTimeout); err != nil {
			return err
		}
	}

	fmt.Println(runID)
	return nil
}

func getRun(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	dataDir := fs.String("data-dir", ".grael-data", "directory for Grael WAL data")
	runID := fs.String("run-id", "", "run identifier")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *runID == "" {
		return errors.New("missing required -run-id")
	}

	svc := api.New(*dataDir)
	view, err := svc.GetRun(*runID)
	if err != nil {
		return err
	}
	return printJSON(view)
}

func listEvents(args []string) error {
	fs := flag.NewFlagSet("events", flag.ContinueOnError)
	dataDir := fs.String("data-dir", ".grael-data", "directory for Grael WAL data")
	runID := fs.String("run-id", "", "run identifier")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *runID == "" {
		return errors.New("missing required -run-id")
	}

	svc := api.New(*dataDir)
	events, err := svc.ListEvents(*runID)
	if err != nil {
		return err
	}
	return printJSON(events)
}

func snapshotInfo(args []string) error {
	fs := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	dataDir := fs.String("data-dir", ".grael-data", "directory for Grael WAL data")
	runID := fs.String("run-id", "", "run identifier")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *runID == "" {
		return errors.New("missing required -run-id")
	}

	svc := api.New(*dataDir)
	info, err := svc.SnapshotInfo(*runID)
	if err != nil {
		return err
	}
	return printJSON(info)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printUsage() {
	fmt.Println(`grael

Commands:
  start  (-workflow <file> | -example <name>) [-demo-worker] [-data-dir <dir>] [-wait-timeout <duration>]
  status -run-id <id> [-data-dir <dir>]
  events -run-id <id> [-data-dir <dir>]
  snapshot -run-id <id> [-data-dir <dir>]

Workflow definition example (JSON ingress format):
{
  "name": "linear-noop",
  "nodes": [
    {"id": "A", "activity_type": "noop"},
    {"id": "B", "activity_type": "noop", "depends_on": ["A"]}
  ]
}

Built-in examples:
  linear-noop`)
}

func startDemoWorker(svc *api.Service, runID string, def rt.WorkflowDefinition) error {
	return startDemoWorkerWithContext(context.Background(), svc, runID, def)
}

func startDemoWorkerWithContext(ctx context.Context, svc *api.Service, runID string, def rt.WorkflowDefinition) error {
	activities := demoWorkerActivities(def)
	const workerID = "demo-worker"
	if err := svc.RegisterWorker(workerID, activities); err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			task, ok, err := svc.PollTask(workerID, 50*time.Millisecond)
			if err != nil {
				return
			}
			if !ok {
				if err := maybeApproveDemoCheckpoint(svc, runID, def); err != nil {
					return
				}
				view, err := svc.GetRun(runID)
				if err == nil && isTerminalRunState(view.State) {
					return
				}
				continue
			}
			if err := handleDemoTask(svc, runID, def, workerID, task); err != nil {
				return
			}
		}
	}()

	return nil
}

func demoWorkerActivities(def rt.WorkflowDefinition) []rt.ActivityType {
	activities := make([]rt.ActivityType, 0, len(def.Nodes))
	seen := map[rt.ActivityType]struct{}{}
	for _, node := range def.Nodes {
		if _, ok := seen[node.ActivityType]; ok {
			continue
		}
		seen[node.ActivityType] = struct{}{}
		activities = append(activities, node.ActivityType)
	}
	if _, ok := seen[rt.ActivityType("discover")]; ok {
		for _, extra := range []rt.ActivityType{"analyze", "review", "finalize", "summarize", "undo"} {
			if _, exists := seen[extra]; exists {
				continue
			}
			seen[extra] = struct{}{}
			activities = append(activities, extra)
		}
	}
	slices.Sort(activities)
	return activities
}

func handleDemoTask(svc *api.Service, runID string, def rt.WorkflowDefinition, workerID string, task rt.WorkerTask) error {
	req := rt.CompleteTaskRequest{
		WorkerID:     workerID,
		RunID:        task.RunID,
		NodeID:       task.NodeID,
		Attempt:      task.Attempt,
		Compensation: task.Compensation,
		Output: map[string]any{
			"status": "ok",
		},
	}
	switch {
	case task.Compensation:
		req.Output["compensated"] = true
	case task.ActivityType == "discover" && def.Name == "living-dag":
		req.SpawnedNodes = []rt.NodeDefinition{
			{ID: "analyze-1", ActivityType: "analyze"},
			{ID: "analyze-2", ActivityType: "analyze"},
			{ID: "analyze-3", ActivityType: "analyze"},
		}
		req.Output["discovered"] = 3
	case task.ActivityType == "discover" && def.Name == "living-dag-ops":
		req.SpawnedNodes = []rt.NodeDefinition{
			{ID: "analyze-1", ActivityType: "analyze", CompensationActivity: "undo"},
			{ID: "review", ActivityType: "review", CompensationActivity: "undo"},
			{ID: "finalize", ActivityType: "finalize", DependsOn: []string{"analyze-1", "review"}},
		}
		req.Output["discovered"] = 3
	case task.ActivityType == "discover" && def.Name == "core-demo":
		req.SpawnedNodes = []rt.NodeDefinition{
			{
				ID:           "analyze-1",
				ActivityType: "analyze",
				RetryPolicy: &rt.RetryPolicy{
					MaxAttempts: 2,
					Backoff:     50 * time.Millisecond,
				},
			},
			{ID: "analyze-2", ActivityType: "analyze"},
			{ID: "review", ActivityType: "review", CheckpointTimeout: time.Second},
			{ID: "summary", ActivityType: "summarize", DependsOn: []string{"analyze-1", "analyze-2", "review"}},
		}
		req.Output["discovered"] = 4
	case def.Name == "core-demo" && task.NodeID == "analyze-1" && task.Attempt == 1:
		return svc.FailTask(rt.FailTaskRequest{
			WorkerID:  workerID,
			RunID:     task.RunID,
			NodeID:    task.NodeID,
			Attempt:   task.Attempt,
			Message:   "demo retryable analysis failure",
			Retryable: true,
		})
	case task.ActivityType == "review":
		reached, err := hasCheckpointReached(svc, runID, task.NodeID)
		if err != nil {
			return err
		}
		if !reached {
			req.Checkpoint = &rt.CheckpointRequest{Reason: "human review"}
		}
	case task.ActivityType == "finalize":
		return svc.FailTask(rt.FailTaskRequest{
			WorkerID: workerID,
			RunID:    task.RunID,
			NodeID:   task.NodeID,
			Attempt:  task.Attempt,
			Message:  "demo finalize failure",
		})
	}
	return svc.CompleteTask(req)
}

func maybeApproveDemoCheckpoint(svc *api.Service, runID string, def rt.WorkflowDefinition) error {
	if def.Name != "living-dag-ops" && def.Name != "core-demo" {
		return nil
	}
	view, err := svc.GetRun(runID)
	if err != nil {
		return err
	}
	reviewNode, ok := view.Nodes["review"]
	if !ok || reviewNode.State != rt.NodeStateAwaitingApproval {
		return nil
	}
	analyzeNode, analyzeOK := view.Nodes["analyze-1"]
	if !analyzeOK || analyzeNode.State != rt.NodeStateCompleted {
		return nil
	}
	return svc.ApproveCheckpoint(runID, "review")
}

func hasCheckpointReached(svc *api.Service, runID, nodeID string) (bool, error) {
	events, err := svc.ListEvents(runID)
	if err != nil {
		return false, err
	}
	for _, event := range events {
		if event.Type != rt.EventCheckpointReached {
			continue
		}
		payload, ok := event.Payload.(rt.CheckpointReachedPayload)
		if ok && payload.NodeID == nodeID {
			return true, nil
		}
	}
	return false, nil
}

func waitForTerminalRun(svc *api.Service, runID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		view, err := svc.GetRun(runID)
		if err != nil {
			return err
		}
		if isTerminalRunState(view.State) {
			return nil
		}
		if timeout > 0 && time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for run %s to reach terminal state", runID)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func isTerminalRunState(state rt.RunState) bool {
	return state == rt.RunStateCompleted || state == rt.RunStateFailed || state == rt.RunStateCancelled || state == rt.RunStateCompensated
}

func stdoutString(fn func() error) (string, error) {
	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	defer func() {
		os.Stdout = original
	}()

	callErr := fn()
	if err := w.Close(); err != nil && callErr == nil {
		callErr = err
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil && callErr == nil {
		callErr = err
	}
	if err := r.Close(); err != nil && callErr == nil {
		callErr = err
	}

	return buf.String(), callErr
}
