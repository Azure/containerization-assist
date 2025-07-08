package conversation

import (
	"time"
)

// ConversationMessage represents an incoming conversation message
type ConversationMessage struct {
	SessionID string                 `json:"session_id"`
	Type      string                 `json:"type"`
	ToolName  string                 `json:"tool_name,omitempty"`
	Arguments interface{}            `json:"arguments,omitempty"`
	AutoFix   bool                   `json:"auto_fix,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// ConversationResponse represents the response to a conversation message
type ConversationResponse struct {
	Success   bool                   `json:"success"`
	Result    interface{}            `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// Session represents a conversation session
type Session struct {
	ID        string                 `json:"id"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	State     map[string]interface{} `json:"state"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// BuildArgs represents build tool arguments for auto-fix
type BuildArgs struct {
	DockerfilePath string            `json:"dockerfile_path"`
	ContextPath    string            `json:"context_path"`
	Tags           []string          `json:"tags"`
	BuildArgs      map[string]string `json:"build_args"`
}

// WorkflowRequest represents a workflow execution request
type WorkflowRequest struct {
	WorkflowID string                 `json:"workflow_id"`
	Parameters map[string]interface{} `json:"parameters"`
	Options    WorkflowOptions        `json:"options"`
}

// WorkflowOptions configures workflow execution
type WorkflowOptions struct {
	Parallel     bool          `json:"parallel"`
	MaxRetries   int           `json:"max_retries"`
	Timeout      time.Duration `json:"timeout"`
	SkipOnError  bool          `json:"skip_on_error"`
	SaveProgress bool          `json:"save_progress"`
}

// StatusRequest represents a status check request
type StatusRequest struct {
	Type   string   `json:"type"` // "session", "workflow", "tool"
	IDs    []string `json:"ids,omitempty"`
	Filter string   `json:"filter,omitempty"`
}

// StatusResponse contains status information
type StatusResponse struct {
	Type    string        `json:"type"`
	Entries []StatusEntry `json:"entries"`
	Summary StatusSummary `json:"summary"`
}

// StatusEntry represents a single status entry
type StatusEntry struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Status    string                 `json:"status"`
	Progress  float64                `json:"progress"`
	StartTime *time.Time             `json:"start_time,omitempty"`
	EndTime   *time.Time             `json:"end_time,omitempty"`
	Duration  *time.Duration         `json:"duration,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// StatusSummary provides aggregate status information
type StatusSummary struct {
	Total     int           `json:"total"`
	Active    int           `json:"active"`
	Completed int           `json:"completed"`
	Failed    int           `json:"failed"`
	Duration  time.Duration `json:"duration"`
	StartTime *time.Time    `json:"start_time,omitempty"`
	EndTime   *time.Time    `json:"end_time,omitempty"`
}
