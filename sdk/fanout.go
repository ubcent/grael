package sdk

import (
	"fmt"
	"maps"
	"time"

	rt "grael/internal/runtime"
)

type FanOutSpec struct {
	IDPrefix     string
	ActivityType rt.ActivityType
	Items        []FanOutItem
}

type FanOutItem struct {
	ID                   string
	Input                map[string]any
	DependsOn            []string
	RetryPolicy          *rt.RetryPolicy
	CompensationActivity rt.ActivityType
	CheckpointTimeout    time.Duration
	ExecutionDeadline    time.Duration
	AbsoluteDeadline     time.Duration
}

func ExpandFanOut(spec FanOutSpec) ([]rt.NodeDefinition, error) {
	if spec.ActivityType == "" {
		return nil, fmt.Errorf("sdk: fan-out activity_type is required")
	}
	if len(spec.Items) == 0 {
		return nil, nil
	}

	nodes := make([]rt.NodeDefinition, 0, len(spec.Items))
	seen := make(map[string]struct{}, len(spec.Items))
	for index, item := range spec.Items {
		id := item.ID
		if id == "" {
			if spec.IDPrefix == "" {
				return nil, fmt.Errorf("sdk: fan-out item %d is missing id and id_prefix", index)
			}
			id = fmt.Sprintf("%s-%d", spec.IDPrefix, index+1)
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("sdk: duplicate fan-out item id %q", id)
		}
		seen[id] = struct{}{}

		nodes = append(nodes, rt.NodeDefinition{
			ID:                   id,
			ActivityType:         spec.ActivityType,
			Input:                maps.Clone(item.Input),
			DependsOn:            append([]string(nil), item.DependsOn...),
			RetryPolicy:          cloneRetryPolicy(item.RetryPolicy),
			CompensationActivity: item.CompensationActivity,
			CheckpointTimeout:    item.CheckpointTimeout,
			ExecutionDeadline:    item.ExecutionDeadline,
			AbsoluteDeadline:     item.AbsoluteDeadline,
		})
	}
	return nodes, nil
}

func cloneRetryPolicy(policy *rt.RetryPolicy) *rt.RetryPolicy {
	if policy == nil {
		return nil
	}
	copyPolicy := *policy
	return &copyPolicy
}
