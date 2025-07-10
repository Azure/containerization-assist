package core

import (
	"encoding/json"
	"time"
)

// TypedArgs represents strongly typed tool arguments
type TypedArgs struct {
	Data json.RawMessage `json:"data"`
}

// TypedResult represents strongly typed tool results
type TypedResult struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// ChatArgs represents typed arguments for chat operations
type ChatArgs struct {
	Message   string            `json:"message"`
	SessionID string            `json:"session_id,omitempty"`
	Context   map[string]string `json:"context,omitempty"`
}

// WorkflowArgs represents typed arguments for workflow operations
type WorkflowArgs struct {
	WorkflowName string            `json:"workflow_name,omitempty"`
	WorkflowSpec json.RawMessage   `json:"workflow_spec,omitempty"`
	Variables    map[string]string `json:"variables,omitempty"`
	Options      WorkflowOptions   `json:"options,omitempty"`
}

// WorkflowOptions represents workflow execution options
type WorkflowOptions struct {
	Timeout time.Duration `json:"timeout,omitempty"`
	Async   bool          `json:"async,omitempty"`
	Retries int           `json:"retries,omitempty"`
}

// WorkflowStatusArgs represents arguments for workflow status queries
type WorkflowStatusArgs struct {
	WorkflowID string `json:"workflow_id"`
	Detailed   bool   `json:"detailed,omitempty"`
}

// ConversationHistoryArgs represents arguments for conversation history queries
type ConversationHistoryArgs struct {
	SessionID string `json:"session_id"`
	Limit     int    `json:"limit,omitempty"`
}

// WorkflowListArgs represents arguments for workflow listing
type WorkflowListArgs struct {
	Status string `json:"status,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// ============================================================================
// Type-Safe Server Enhancements
// ============================================================================

// TypedToolArgs represents strongly typed tool arguments with validation
type TypedToolArgs struct {
	ToolName string                 `json:"tool_name"`
	Args     map[string]interface{} `json:"args"`
	Context  map[string]string      `json:"context,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// TypedToolResult represents strongly typed tool results with metadata
type TypedToolResult struct {
	Success   bool                   `json:"success"`
	Data      interface{}            `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// TypedServerInfo represents server information with capabilities
type TypedServerInfo struct {
	Name         string             `json:"name"`
	Version      string             `json:"version"`
	Capabilities ServerCapabilities `json:"capabilities"`
	Mode         ServerMode         `json:"mode"`
	Uptime       time.Duration      `json:"uptime"`
}

// TypedToolInfo represents typed tool information
type TypedToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	Schema      map[string]interface{} `json:"schema"`
	Category    string                 `json:"category"`
	Tags        []string               `json:"tags,omitempty"`
}

// TypedWorkflowResponse represents workflow execution response
type TypedWorkflowResponse struct {
	WorkflowID string                 `json:"workflow_id"`
	Status     string                 `json:"status"`
	Steps      []TypedWorkflowStep    `json:"steps"`
	Result     interface{}            `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    *time.Time             `json:"end_time,omitempty"`
}

// TypedWorkflowStep represents a single step in workflow execution
type TypedWorkflowStep struct {
	StepID    string                 `json:"step_id"`
	Name      string                 `json:"name"`
	Status    string                 `json:"status"`
	Result    interface{}            `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	StartTime time.Time              `json:"start_time"`
	EndTime   *time.Time             `json:"end_time,omitempty"`
}

// TypedSessionInfo represents session information
type TypedSessionInfo struct {
	SessionID   string                 `json:"session_id"`
	UserID      string                 `json:"user_id,omitempty"`
	WorkspaceID string                 `json:"workspace_id,omitempty"`
	Status      string                 `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Labels      map[string]string      `json:"labels,omitempty"`
}

// ServerMode defines the operational mode of the server
type ServerMode string

const (
	ModeDual     ServerMode = "dual"     // Both interfaces available
	ModeChat     ServerMode = "chat"     // Chat-only mode
	ModeWorkflow ServerMode = "workflow" // Workflow-only mode
)

// ServerCapabilities defines what the server can do
type ServerCapabilities struct {
	ChatSupport     bool     `json:"chat_support"`
	WorkflowSupport bool     `json:"workflow_support"`
	AvailableModes  []string `json:"available_modes"`
	SharedTools     []string `json:"shared_tools"`
}

// ToolDefinition represents a tool definition
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Category    string                 `json:"category,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
}
