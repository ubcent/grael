package harness

import (
	"context"
	"fmt"
	"strings"
	"time"

	"grael/internal/api"
	rt "grael/internal/runtime"
	"grael/sdk"
)

type Profile string

const (
	ProfileShowcase Profile = "showcase"
	ProfileFast     Profile = "fast"
)

type workerSpec struct {
	idSuffix   string
	activities []rt.ActivityType
}

func ParseProfile(value string) (Profile, error) {
	switch Profile(strings.TrimSpace(value)) {
	case "", ProfileShowcase:
		return ProfileShowcase, nil
	case ProfileFast:
		return ProfileFast, nil
	default:
		return "", fmt.Errorf("unknown demo profile %q", value)
	}
}

func DefaultWaitTimeout(profile Profile) time.Duration {
	switch profile {
	case ProfileFast:
		return 6 * time.Second
	case ProfileShowcase:
		return 60 * time.Second
	default:
		return 2 * time.Second
	}
}

func Start(ctx context.Context, svc *api.Service, runID string, def rt.WorkflowDefinition, profile Profile) error {
	return StartWithPrefix(ctx, svc, runID, def, profile, "demo-worker")
}

func StartWithPrefix(ctx context.Context, svc *api.Service, runID string, def rt.WorkflowDefinition, profile Profile, workerPrefix string) error {
	for _, spec := range workerSpecs(def) {
		workerID := workerPrefix
		if spec.idSuffix != "" {
			workerID = workerPrefix + "-" + spec.idSuffix
		}
		worker := sdk.NewWorker(sdk.NewServiceClient(svc), workerID)
		worker.SetPollTimeout(50 * time.Millisecond)
		for _, activity := range spec.activities {
			worker.Handle(activity, func(ctx context.Context, task sdk.Task) (sdk.Result, error) {
				return executeTask(ctx, svc, runID, def, workerID, task, profile)
			})
		}
		go func() {
			_ = worker.Run(ctx)
		}()
	}

	go func() {
		_ = autoApproveLoop(ctx, svc, runID, def.Name)
	}()

	return nil
}

func workerSpecs(def rt.WorkflowDefinition) []workerSpec {
	switch def.Name {
	case "core-demo":
		return []workerSpec{
			{idSuffix: "signals", activities: []rt.ActivityType{"collect_signals"}},
			{idSuffix: "metrics", activities: []rt.ActivityType{"collect_metrics"}},
			{idSuffix: "briefing", activities: []rt.ActivityType{"prepare_brief", "plan_follow_up", "draft_brief"}},
			{idSuffix: "investigation-a", activities: []rt.ActivityType{"investigate"}},
			{idSuffix: "investigation-b", activities: []rt.ActivityType{"investigate"}},
			{idSuffix: "investigation-c", activities: []rt.ActivityType{"investigate"}},
			{idSuffix: "editor", activities: []rt.ActivityType{"review", "publish"}},
		}
	default:
		return []workerSpec{
			{idSuffix: "", activities: definitionActivities(def)},
		}
	}
}

func definitionActivities(def rt.WorkflowDefinition) []rt.ActivityType {
	activities := make([]rt.ActivityType, 0, len(def.Nodes)+5)
	for _, node := range def.Nodes {
		activities = append(activities, node.ActivityType)
	}
	if def.Name == "living-dag" || def.Name == "living-dag-ops" {
		activities = append(activities, "analyze")
	}
	if def.Name == "living-dag-ops" {
		activities = append(activities, "review", "finalize", "undo")
	}
	return demoWorkerActivities(activities)
}

func demoWorkerActivities(activities []rt.ActivityType) []rt.ActivityType {
	seen := make(map[rt.ActivityType]struct{}, len(activities))
	unique := make([]rt.ActivityType, 0, len(activities))
	for _, activity := range activities {
		if _, ok := seen[activity]; ok {
			continue
		}
		seen[activity] = struct{}{}
		unique = append(unique, activity)
	}
	return unique
}

func executeTask(ctx context.Context, svc *api.Service, runID string, def rt.WorkflowDefinition, workerID string, task sdk.Task, profile Profile) (sdk.Result, error) {
	if err := paceTask(ctx, svc, workerID, def.Name, task, profile); err != nil {
		return sdk.Result{}, err
	}

	switch def.Name {
	case "living-dag":
		return executeLivingDAGTask(task), nil
	case "living-dag-ops":
		return executeLivingDAGOpsTask(task)
	case "core-demo":
		return executeCoreDemoTask(task)
	default:
		return sdk.Result{
			Output: map[string]any{
				"status": "ok",
			},
		}, nil
	}
}

func executeLivingDAGTask(task sdk.Task) sdk.Result {
	result := sdk.Result{
		Output: map[string]any{
			"status": "ok",
		},
	}
	if task.ActivityType == "discover" {
		result.SpawnedNodes = []rt.NodeDefinition{
			{ID: "analyze-1", ActivityType: "analyze"},
			{ID: "analyze-2", ActivityType: "analyze"},
			{ID: "analyze-3", ActivityType: "analyze"},
		}
		result.Output["discovered"] = 3
	}
	return result
}

func executeLivingDAGOpsTask(task sdk.Task) (sdk.Result, error) {
	result := sdk.Result{
		Output: map[string]any{
			"status": "ok",
		},
	}
	switch {
	case task.Compensation:
		result.Output["compensated"] = true
		return result, nil
	case task.ActivityType == "discover":
		result.SpawnedNodes = []rt.NodeDefinition{
			{ID: "analyze-1", ActivityType: "analyze", CompensationActivity: "undo"},
			{ID: "review", ActivityType: "review", CompensationActivity: "undo"},
			{ID: "finalize", ActivityType: "finalize", DependsOn: []string{"analyze-1", "review"}},
		}
		result.Output["discovered"] = 3
		return result, nil
	case task.ActivityType == "review" && task.Attempt == 1:
		result.Checkpoint = &rt.CheckpointRequest{Reason: "human review"}
		return result, nil
	case task.ActivityType == "finalize":
		return sdk.Result{}, &sdk.TaskError{Message: "demo finalize failure"}
	default:
		return result, nil
	}
}

func executeCoreDemoTask(task sdk.Task) (sdk.Result, error) {
	result := sdk.Result{
		Output: map[string]any{
			"status": "ok",
		},
	}

	switch task.NodeID {
	case "collect-customer-escalations":
		result.Output["summary"] = "support escalations clustered around checkout reliability"
	case "pull-checkout-metrics":
		result.Output["summary"] = "checkout success rate dipped below the morning threshold"
	case "prepare-brief-outline":
		result.Output["summary"] = "brief shell prepared for the morning operator update"
	case "decide-follow-up-checks":
		result.Output["summary"] = "open targeted follow-up checks before publishing the brief"
		result.SpawnedNodes = []rt.NodeDefinition{
			{ID: "verify-checkout-latency", ActivityType: "investigate"},
			{
				ID:           "confirm-payment-auth-drop",
				ActivityType: "investigate",
				RetryPolicy: &rt.RetryPolicy{
					MaxAttempts: 2,
					Backoff:     50 * time.Millisecond,
				},
			},
			{ID: "review-support-spike", ActivityType: "investigate"},
			{
				ID:           "assemble-incident-brief",
				ActivityType: "draft_brief",
				DependsOn: []string{
					"verify-checkout-latency",
					"confirm-payment-auth-drop",
					"review-support-spike",
				},
			},
			{
				ID:           "publish-morning-brief",
				ActivityType: "publish",
				DependsOn: []string{
					"assemble-incident-brief",
					"editor-approval",
				},
			},
		}
	case "verify-checkout-latency":
		result.Output["finding"] = "europe-west checkout latency spiked after the overnight deploy"
	case "confirm-payment-auth-drop":
		if task.Attempt == 1 {
			return sdk.Result{}, &sdk.TaskError{
				Message:   "payment gateway sample timed out; retrying with a fresh fetch",
				Retryable: true,
			}
		}
		result.Output["finding"] = "payment authorization dip confirmed between 08:02 and 08:09 UTC"
	case "review-support-spike":
		result.Output["finding"] = "support queue spike tracks the same payment and latency window"
	case "assemble-incident-brief":
		result.Output["summary"] = "investigation findings merged into the operator-facing morning brief"
	case "editor-approval":
		if task.Attempt == 1 {
			result.Checkpoint = &rt.CheckpointRequest{Reason: "editor sign-off before publishing the morning brief"}
			result.Output["status"] = "awaiting editor sign-off"
			return result, nil
		}
		result.Output["status"] = "editor approved the morning brief"
	case "publish-morning-brief":
		result.Output["status"] = "morning incident brief published to leadership and support"
	}

	return result, nil
}

func paceTask(ctx context.Context, svc *api.Service, workerID, workflowName string, task sdk.Task, profile Profile) error {
	delay := taskDelay(workflowName, task, profile)
	if delay <= 0 {
		return nil
	}

	if err := svc.Heartbeat(workerID); err != nil {
		return err
	}

	deadline := time.Now().Add(delay)
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil
		}
		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-ticker.C:
			timer.Stop()
			if err := svc.Heartbeat(workerID); err != nil {
				return err
			}
		case <-timer.C:
			return nil
		}
	}
}

func taskDelay(workflowName string, task sdk.Task, profile Profile) time.Duration {
	if workflowName != "core-demo" {
		return 0
	}
	if profile == ProfileShowcase {
		switch task.NodeID {
		case "collect-customer-escalations":
			return 2800 * time.Millisecond
		case "pull-checkout-metrics":
			return 3200 * time.Millisecond
		case "prepare-brief-outline":
			return 2400 * time.Millisecond
		case "decide-follow-up-checks":
			return 2600 * time.Millisecond
		case "verify-checkout-latency":
			return 4200 * time.Millisecond
		case "confirm-payment-auth-drop":
			return 4500 * time.Millisecond
		case "review-support-spike":
			return 3900 * time.Millisecond
		case "assemble-incident-brief":
			return 3100 * time.Millisecond
		case "editor-approval":
			if task.Attempt == 1 {
				return 2200 * time.Millisecond
			}
			return 1800 * time.Millisecond
		case "publish-morning-brief":
			return 2600 * time.Millisecond
		}
	}

	switch task.NodeID {
	case "collect-customer-escalations":
		return 95 * time.Millisecond
	case "pull-checkout-metrics":
		return 110 * time.Millisecond
	case "prepare-brief-outline":
		return 85 * time.Millisecond
	case "decide-follow-up-checks":
		return 95 * time.Millisecond
	case "verify-checkout-latency":
		return 110 * time.Millisecond
	case "confirm-payment-auth-drop":
		if task.Attempt == 1 {
			return 120 * time.Millisecond
		}
		return 115 * time.Millisecond
	case "review-support-spike":
		return 100 * time.Millisecond
	case "assemble-incident-brief":
		return 95 * time.Millisecond
	case "editor-approval":
		if task.Attempt == 1 {
			return 80 * time.Millisecond
		}
		return 75 * time.Millisecond
	case "publish-morning-brief":
		return 90 * time.Millisecond
	default:
		return 0
	}
}

func autoApproveLoop(ctx context.Context, svc *api.Service, runID, workflowName string) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := maybeApproveCheckpoint(svc, runID, workflowName); err != nil {
				return err
			}
			view, err := svc.GetRun(runID)
			if err == nil && isTerminal(view.State) {
				return nil
			}
		}
	}
}

func maybeApproveCheckpoint(svc *api.Service, runID, workflowName string) error {
	switch workflowName {
	case "living-dag-ops":
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
	case "core-demo":
		view, err := svc.GetRun(runID)
		if err != nil {
			return err
		}
		reviewNode, ok := view.Nodes["editor-approval"]
		if !ok || reviewNode.State != rt.NodeStateAwaitingApproval {
			return nil
		}
		draftNode, draftOK := view.Nodes["assemble-incident-brief"]
		if !draftOK || draftNode.State != rt.NodeStateCompleted {
			return nil
		}
		return svc.ApproveCheckpoint(runID, "editor-approval")
	default:
		return nil
	}
}

func isTerminal(state rt.RunState) bool {
	return state == rt.RunStateCompleted || state == rt.RunStateFailed || state == rt.RunStateCancelled || state == rt.RunStateCompensated
}
