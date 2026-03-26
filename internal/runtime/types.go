package runtime

import "time"

type EventType string

const (
	EventWorkflowStarted   EventType = "WorkflowStarted"
	EventLeaseGranted      EventType = "LeaseGranted"
	EventNodeReady         EventType = "NodeReady"
	EventNodeStarted       EventType = "NodeStarted"
	EventNodeCompleted     EventType = "NodeCompleted"
	EventNodeFailed        EventType = "NodeFailed"
	EventWorkflowCompleted EventType = "WorkflowCompleted"
)

type NodeState string

const (
	NodeStatePending   NodeState = "PENDING"
	NodeStateReady     NodeState = "READY"
	NodeStateRunning   NodeState = "RUNNING"
	NodeStateCompleted NodeState = "COMPLETED"
	NodeStateFailed    NodeState = "FAILED"
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

type LeaseGrantedPayload struct {
	NodeID   string `json:"node_id"`
	WorkerID string `json:"worker_id"`
	Attempt  uint32 `json:"attempt"`
	Activity string `json:"activity_type"`
}

type NodeReadyPayload struct {
	NodeID string `json:"node_id"`
}

type NodeStartedPayload struct {
	NodeID   string `json:"node_id"`
	WorkerID string `json:"worker_id"`
	Attempt  uint32 `json:"attempt"`
}

type NodeCompletedPayload struct {
	NodeID   string         `json:"node_id"`
	WorkerID string         `json:"worker_id"`
	Attempt  uint32         `json:"attempt"`
	Output   map[string]any `json:"output,omitempty"`
}

type NodeFailedPayload struct {
	NodeID   string `json:"node_id"`
	WorkerID string `json:"worker_id"`
	Attempt  uint32 `json:"attempt"`
	Message  string `json:"message,omitempty"`
}

type WorkflowCompletedPayload struct{}

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
	Attempt      uint32       `json:"attempt,omitempty"`
	WorkerID     string       `json:"worker_id,omitempty"`
	LastError    string       `json:"last_error,omitempty"`
}

type WorkerTask struct {
	RunID         string         `json:"run_id"`
	NodeID        string         `json:"node_id"`
	ActivityType  ActivityType   `json:"activity_type"`
	Attempt       uint32         `json:"attempt"`
	Workflow      string         `json:"workflow"`
	WorkflowInput map[string]any `json:"workflow_input,omitempty"`
}

type CompleteTaskRequest struct {
	WorkerID string         `json:"worker_id"`
	RunID    string         `json:"run_id"`
	NodeID   string         `json:"node_id"`
	Attempt  uint32         `json:"attempt"`
	Output   map[string]any `json:"output,omitempty"`
}

type FailTaskRequest struct {
	WorkerID string `json:"worker_id"`
	RunID    string `json:"run_id"`
	NodeID   string `json:"node_id"`
	Attempt  uint32 `json:"attempt"`
	Message  string `json:"message,omitempty"`
}
