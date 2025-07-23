// Package events provides domain event definitions and publishing for Container Kit MCP.
package events

import (
	"time"
)

// DomainEvent represents a domain event that occurred within the system.
// Events are used to decouple components and enable reactive behaviors.
type DomainEvent interface {
	// EventID returns the unique identifier for this event
	EventID() string

	// OccurredAt returns when the event occurred
	OccurredAt() time.Time

	// WorkflowID returns the workflow this event is associated with
	// This aligns with existing tracing infrastructure
	WorkflowID() string

	// EventType returns the type name of this event
	EventType() string
}

// WorkflowStepCompletedEvent is published when a workflow step completes
type WorkflowStepCompletedEvent struct {
	ID         string        `json:"id"`
	Timestamp  time.Time     `json:"timestamp"`
	Workflow   string        `json:"workflow_id"`
	StepName   string        `json:"step_name"`
	Duration   time.Duration `json:"duration"`
	Success    bool          `json:"success"`
	ErrorMsg   string        `json:"error_message,omitempty"`
	Progress   float64       `json:"progress"`
	StepNumber int           `json:"step_number"`
	TotalSteps int           `json:"total_steps"`
}

func (e WorkflowStepCompletedEvent) EventID() string       { return e.ID }
func (e WorkflowStepCompletedEvent) OccurredAt() time.Time { return e.Timestamp }
func (e WorkflowStepCompletedEvent) WorkflowID() string    { return e.Workflow }
func (e WorkflowStepCompletedEvent) EventType() string     { return "workflow.step.completed" }

// WorkflowStartedEvent is published when a workflow begins
type WorkflowStartedEvent struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Workflow  string    `json:"workflow_id"`
	RepoURL   string    `json:"repo_url"`
	Branch    string    `json:"branch,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
}

func (e WorkflowStartedEvent) EventID() string       { return e.ID }
func (e WorkflowStartedEvent) OccurredAt() time.Time { return e.Timestamp }
func (e WorkflowStartedEvent) WorkflowID() string    { return e.Workflow }
func (e WorkflowStartedEvent) EventType() string     { return "workflow.started" }

// WorkflowCompletedEvent is published when an entire workflow finishes
type WorkflowCompletedEvent struct {
	ID            string        `json:"id"`
	Timestamp     time.Time     `json:"timestamp"`
	Workflow      string        `json:"workflow_id"`
	Success       bool          `json:"success"`
	TotalDuration time.Duration `json:"total_duration"`
	ImageRef      string        `json:"image_ref,omitempty"`
	Namespace     string        `json:"k8s_namespace,omitempty"`
	Endpoint      string        `json:"endpoint,omitempty"`
	ErrorMsg      string        `json:"error_message,omitempty"`
}

func (e WorkflowCompletedEvent) EventID() string       { return e.ID }
func (e WorkflowCompletedEvent) OccurredAt() time.Time { return e.Timestamp }
func (e WorkflowCompletedEvent) WorkflowID() string    { return e.Workflow }
func (e WorkflowCompletedEvent) EventType() string     { return "workflow.completed" }

// SecurityScanEvent is published when security scanning occurs
type SecurityScanEvent struct {
	ID            string        `json:"id"`
	Timestamp     time.Time     `json:"timestamp"`
	Workflow      string        `json:"workflow_id"`
	ImageRef      string        `json:"image_ref"`
	Scanner       string        `json:"scanner"`
	VulnCount     int           `json:"vulnerability_count"`
	CriticalCount int           `json:"critical_count"`
	HighCount     int           `json:"high_count"`
	ScanDuration  time.Duration `json:"scan_duration"`
}

func (e SecurityScanEvent) EventID() string       { return e.ID }
func (e SecurityScanEvent) OccurredAt() time.Time { return e.Timestamp }
func (e SecurityScanEvent) WorkflowID() string    { return e.Workflow }
func (e SecurityScanEvent) EventType() string     { return "security.scan.completed" }

// ErrorAnalysisEvent is published when AI analyzes a workflow error
type ErrorAnalysisEvent struct {
	ID             string      `json:"id"`
	Timestamp      time.Time   `json:"timestamp"`
	Workflow       string      `json:"workflow_id"`
	StepName       string      `json:"step_name"`
	ErrorMessage   string      `json:"error_message"`
	Classification interface{} `json:"classification"` // Will be ml.ErrorClassification
	Context        interface{} `json:"context"`        // Will be ml.WorkflowContext
}

func (e ErrorAnalysisEvent) EventID() string       { return e.ID }
func (e ErrorAnalysisEvent) OccurredAt() time.Time { return e.Timestamp }
func (e ErrorAnalysisEvent) WorkflowID() string    { return e.Workflow }
func (e ErrorAnalysisEvent) EventType() string     { return "error.analysis" }
