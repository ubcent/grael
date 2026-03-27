package runtime

import "time"

type EventType string

const (
	EventWorkflowStarted   EventType = "WorkflowStarted"
	EventLeaseGranted      EventType = "LeaseGranted"
	EventHeartbeatRecorded EventType = "HeartbeatRecorded"
	EventLeaseExpired      EventType = "LeaseExpired"
	EventTimerScheduled    EventType = "TimerScheduled"
	EventTimerFired        EventType = "TimerFired"
	EventNodeReady         EventType = "NodeReady"
	EventNodeStarted       EventType = "NodeStarted"
	EventNodeCompleted     EventType = "NodeCompleted"
	EventNodeFailed        EventType = "NodeFailed"
	EventWorkflowFailed    EventType = "WorkflowFailed"
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
	RunStateFailed    RunState = "FAILED"
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

type RetryPolicy struct {
	MaxAttempts int           `json:"max_attempts"`
	Backoff     time.Duration `json:"backoff"`
}

type NodeDefinition struct {
	ID                string        `json:"id"`
	ActivityType      ActivityType  `json:"activity_type"`
	DependsOn         []string      `json:"depends_on,omitempty"`
	RetryPolicy       *RetryPolicy  `json:"retry_policy,omitempty"`
	ExecutionDeadline time.Duration `json:"execution_deadline,omitempty"`
	AbsoluteDeadline  time.Duration `json:"absolute_deadline,omitempty"`
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

type HeartbeatRecordedPayload struct {
	NodeID   string `json:"node_id"`
	WorkerID string `json:"worker_id"`
	Attempt  uint32 `json:"attempt"`
}

type LeaseExpiredPayload struct {
	NodeID   string `json:"node_id"`
	WorkerID string `json:"worker_id"`
	Attempt  uint32 `json:"attempt"`
}

type TimerPurpose string

const (
	TimerPurposeRetryBackoff     TimerPurpose = "retry_backoff"
	TimerPurposeNodeExecDeadline TimerPurpose = "node_exec_deadline"
	TimerPurposeNodeAbsDeadline  TimerPurpose = "node_abs_deadline"
)

type TimerScheduledPayload struct {
	TimerID  string       `json:"timer_id"`
	NodeID   string       `json:"node_id"`
	Attempt  uint32       `json:"attempt"`
	Purpose  TimerPurpose `json:"purpose"`
	FireAt   time.Time    `json:"fire_at"`
	WorkerID string       `json:"worker_id,omitempty"`
}

type TimerFiredPayload struct {
	TimerID string       `json:"timer_id"`
	NodeID  string       `json:"node_id"`
	Attempt uint32       `json:"attempt"`
	Purpose TimerPurpose `json:"purpose"`
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
	NodeID       string           `json:"node_id"`
	WorkerID     string           `json:"worker_id"`
	Attempt      uint32           `json:"attempt"`
	Output       map[string]any   `json:"output,omitempty"`
	SpawnedNodes []NodeDefinition `json:"spawned_nodes,omitempty"`
}

type NodeFailedPayload struct {
	NodeID    string `json:"node_id"`
	WorkerID  string `json:"worker_id"`
	Attempt   uint32 `json:"attempt"`
	Message   string `json:"message,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
	TimedOut  bool   `json:"timed_out,omitempty"`
}

type WorkflowCompletedPayload struct{}

type WorkflowFailedPayload struct {
	Reason string `json:"reason,omitempty"`
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
	WorkerID     string           `json:"worker_id"`
	RunID        string           `json:"run_id"`
	NodeID       string           `json:"node_id"`
	Attempt      uint32           `json:"attempt"`
	Output       map[string]any   `json:"output,omitempty"`
	SpawnedNodes []NodeDefinition `json:"spawned_nodes,omitempty"`
}

type FailTaskRequest struct {
	WorkerID  string `json:"worker_id"`
	RunID     string `json:"run_id"`
	NodeID    string `json:"node_id"`
	Attempt   uint32 `json:"attempt"`
	Message   string `json:"message,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
}
