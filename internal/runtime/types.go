package runtime

import "time"

type EventType string

const (
	EventWorkflowStarted   EventType = "WorkflowStarted"
	EventNodeReady         EventType = "NodeReady"
	EventNodeStarted       EventType = "NodeStarted"
	EventNodeCompleted     EventType = "NodeCompleted"
	EventWorkflowCompleted EventType = "WorkflowCompleted"
)

type NodeState string

const (
	NodeStatePending   NodeState = "PENDING"
	NodeStateReady     NodeState = "READY"
	NodeStateRunning   NodeState = "RUNNING"
	NodeStateCompleted NodeState = "COMPLETED"
)

type RunState string

const (
	RunStateRunning   RunState = "RUNNING"
	RunStateCompleted RunState = "COMPLETED"
)

type ActivityType string

const (
	ActivityTypeNoop ActivityType = "noop"
	ActivityTypeHold ActivityType = "hold"
)

type WorkflowDefinition struct {
	Name  string           `json:"name"`
	Nodes []NodeDefinition `json:"nodes"`
}

type NodeDefinition struct {
	ID           string       `json:"id"`
	ActivityType ActivityType `json:"activity_type"`
	DependsOn    []string     `json:"depends_on,omitempty"`
}

type Event struct {
	Seq       uint64      `json:"seq"`
	RunID     string      `json:"run_id"`
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}

type WorkflowStartedPayload struct {
	Workflow WorkflowDefinition `json:"workflow"`
	Input    map[string]any     `json:"input,omitempty"`
}

type NodeReadyPayload struct {
	NodeID string `json:"node_id"`
}

type NodeStartedPayload struct {
	NodeID string `json:"node_id"`
}

type NodeCompletedPayload struct {
	NodeID string         `json:"node_id"`
	Output map[string]any `json:"output,omitempty"`
}

type WorkflowCompletedPayload struct{}

type CommandType string

const (
	CommandStartNode        CommandType = "StartNode"
	CommandCompleteNode     CommandType = "CompleteNode"
	CommandCompleteWorkflow CommandType = "CompleteWorkflow"
)

type Command struct {
	Type   CommandType
	RunID  string
	NodeID string
}

type RunView struct {
	RunID      string              `json:"run_id"`
	Workflow   string              `json:"workflow"`
	State      RunState            `json:"state"`
	LastSeq    uint64              `json:"last_seq"`
	Nodes      map[string]NodeView `json:"nodes"`
	CreatedAt  time.Time           `json:"created_at"`
	FinishedAt *time.Time          `json:"finished_at,omitempty"`
}

type NodeView struct {
	ID           string       `json:"id"`
	ActivityType ActivityType `json:"activity_type"`
	State        NodeState    `json:"state"`
	DependsOn    []string     `json:"depends_on,omitempty"`
}
