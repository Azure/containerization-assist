// Package queries provides CQRS query definitions for Containerization Assist MCP.
package queries

import (
	"fmt"
	"time"

	"github.com/Azure/containerization-assist/pkg/mcp/domain/workflow"
)

// Query represents a query that reads system state
type Query interface {
	// QueryID returns the unique identifier for this query
	QueryID() string

	// QueryType returns the type name of this query
	QueryType() string

	// Validate checks if the query is valid
	Validate() error
}

// WorkflowStatusQuery retrieves the status of a specific workflow
type WorkflowStatusQuery struct {
	ID         string `json:"id"`
	SessionID  string `json:"session_id"`
	WorkflowID string `json:"workflow_id,omitempty"`
}

func (q WorkflowStatusQuery) QueryID() string   { return q.ID }
func (q WorkflowStatusQuery) QueryType() string { return "workflow_status" }

func (q WorkflowStatusQuery) Validate() error {
	if q.ID == "" {
		return fmt.Errorf("query ID is required")
	}
	if q.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}
	return nil
}

// SessionListQuery retrieves all sessions for a user
type SessionListQuery struct {
	ID     string `json:"id"`
	UserID string `json:"user_id,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

func (q SessionListQuery) QueryID() string   { return q.ID }
func (q SessionListQuery) QueryType() string { return "session_list" }

func (q SessionListQuery) Validate() error {
	if q.ID == "" {
		return fmt.Errorf("query ID is required")
	}
	if q.Limit < 0 {
		return fmt.Errorf("limit must be non-negative")
	}
	if q.Offset < 0 {
		return fmt.Errorf("offset must be non-negative")
	}
	return nil
}

// WorkflowHistoryQuery retrieves workflow execution history
type WorkflowHistoryQuery struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	StartTime time.Time `json:"start_time,omitempty"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Limit     int       `json:"limit,omitempty"`
	Offset    int       `json:"offset,omitempty"`
}

func (q WorkflowHistoryQuery) QueryID() string   { return q.ID }
func (q WorkflowHistoryQuery) QueryType() string { return "workflow_history" }

func (q WorkflowHistoryQuery) Validate() error {
	if q.ID == "" {
		return fmt.Errorf("query ID is required")
	}
	if !q.StartTime.IsZero() && !q.EndTime.IsZero() && q.StartTime.After(q.EndTime) {
		return fmt.Errorf("start time must be before end time")
	}
	if q.Limit < 0 {
		return fmt.Errorf("limit must be non-negative")
	}
	if q.Offset < 0 {
		return fmt.Errorf("offset must be non-negative")
	}
	return nil
}

// --- Query Result Types ---

// WorkflowStatusView represents the current status of a workflow
type WorkflowStatusView struct {
	WorkflowID  string                  `json:"workflow_id"`
	SessionID   string                  `json:"session_id"`
	Status      string                  `json:"status"`
	Progress    float64                 `json:"progress"`
	CurrentStep int                     `json:"current_step"`
	TotalSteps  int                     `json:"total_steps"`
	Steps       []workflow.WorkflowStep `json:"steps"`
	StartTime   time.Time               `json:"start_time"`
	EndTime     *time.Time              `json:"end_time,omitempty"`
	Duration    *time.Duration          `json:"duration,omitempty"`

	// Result information
	ImageRef   string                 `json:"image_ref,omitempty"`
	Namespace  string                 `json:"k8s_namespace,omitempty"`
	Endpoint   string                 `json:"endpoint,omitempty"`
	ScanReport map[string]interface{} `json:"scan_report,omitempty"`
	Error      string                 `json:"error,omitempty"`

	// Repository information
	RepoURL string `json:"repo_url"`
	Branch  string `json:"branch,omitempty"`
}

// SessionSummaryView represents a summary of a session
type SessionSummaryView struct {
	SessionID       string                 `json:"session_id"`
	Status          string                 `json:"status"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	ExpiresAt       time.Time              `json:"expires_at"`
	Labels          map[string]string      `json:"labels"`
	Metadata        map[string]interface{} `json:"metadata"`
	ActiveWorkflows int                    `json:"active_workflows"`
	TotalWorkflows  int                    `json:"total_workflows"`
}

// WorkflowHistoryView represents a historical workflow execution
type WorkflowHistoryView struct {
	WorkflowID     string         `json:"workflow_id"`
	SessionID      string         `json:"session_id"`
	Status         string         `json:"status"`
	StartTime      time.Time      `json:"start_time"`
	EndTime        *time.Time     `json:"end_time,omitempty"`
	Duration       *time.Duration `json:"duration,omitempty"`
	Success        bool           `json:"success"`
	StepsTotal     int            `json:"steps_total"`
	StepsCompleted int            `json:"steps_completed"`
	ImageRef       string         `json:"image_ref,omitempty"`
	RepoURL        string         `json:"repo_url"`
	Branch         string         `json:"branch,omitempty"`
	Error          string         `json:"error,omitempty"`
}

// SessionListView represents a paginated list of sessions
type SessionListView struct {
	Sessions []SessionSummaryView `json:"sessions"`
	Total    int                  `json:"total"`
	Limit    int                  `json:"limit"`
	Offset   int                  `json:"offset"`
	HasMore  bool                 `json:"has_more"`
}

// WorkflowHistoryListView represents a paginated list of workflow history
type WorkflowHistoryListView struct {
	Workflows []WorkflowHistoryView `json:"workflows"`
	Total     int                   `json:"total"`
	Limit     int                   `json:"limit"`
	Offset    int                   `json:"offset"`
	HasMore   bool                  `json:"has_more"`
}
