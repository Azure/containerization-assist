// Package types - Transport layer type definitions
package types

import (
	"context"
	"time"
)

// ============================================================================
// MCP Protocol Types - Core transport layer
// ============================================================================

// MCPMessage represents a message in the MCP protocol
type MCPMessage struct {
	ID      string                 `json:"id"`
	Type    MCPMessageType         `json:"type"`
	Payload map[string]interface{} `json:"payload"`
	Headers map[string]string      `json:"headers,omitempty"`
}

// MCPMessageType represents the type of MCP message
type MCPMessageType string

const (
	MCPMessageTypeRequest      MCPMessageType = "request"
	MCPMessageTypeResponse     MCPMessageType = "response"
	MCPMessageTypeNotification MCPMessageType = "notification"
	MCPMessageTypeError        MCPMessageType = "error"
)

// ============================================================================
// Transport Configuration Types
// ============================================================================

// TransportConfig represents transport layer configuration
type TransportConfig struct {
	Type    TransportType          `json:"type"`
	Options map[string]interface{} `json:"options"`
	Timeout time.Duration          `json:"timeout"`
}

// TransportType represents the type of transport
type TransportType string

const (
	TransportTypeStdio TransportType = "stdio"
	TransportTypeHTTP  TransportType = "http"
	TransportTypeWS    TransportType = "websocket"
)

// HTTP Configuration Constants
const (
	DefaultHTTPPort           = 8090
	DefaultRateLimitPerMinute = 100
	DefaultMaxBodyLogSize     = 4096
	HTTPTimeoutSeconds        = 30
	HTTPIdleTimeoutSeconds    = 60
	CORSMaxAgeSeconds         = 3600
)

// HTTP timeout durations
var (
	HTTPTimeout     = time.Duration(HTTPTimeoutSeconds) * time.Second
	HTTPIdleTimeout = time.Duration(HTTPIdleTimeoutSeconds) * time.Second
	CORSMaxAge      = time.Duration(CORSMaxAgeSeconds) * time.Second
)

// LLMTransport interface for LLM transport implementations
type LLMTransport interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Send(ctx context.Context, message interface{}) error
	Receive(ctx context.Context) (interface{}, error)
	IsConnected() bool
}

// ToolInvocationResponse represents the response from a tool invocation
type ToolInvocationResponse struct {
	ToolName string                 `json:"tool_name"`
	Success  bool                   `json:"success"`
	Result   map[string]interface{} `json:"result,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Duration time.Duration          `json:"duration"`
}

// HTTPTransportConfig configures HTTP transport
type HTTPTransportConfig struct {
	Host string `json:"host" env:"HTTP_HOST" default:"localhost"`
	Port int    `json:"port" env:"HTTP_PORT" default:"8090"`
}

// ============================================================================
// API Request/Response Types - For external integrations
// ============================================================================

// APIRequest represents a generic API request
type APIRequest struct {
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      interface{}       `json:"body,omitempty"`
	Timestamp int64             `json:"timestamp"`
}

// APIResponse represents a generic API response
type APIResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       interface{}       `json:"body,omitempty"`
	Error      *APIError         `json:"error,omitempty"`
	Timestamp  int64             `json:"timestamp"`
}

// APIError represents an API error
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// ============================================================================
// Session Metadata Types - Transport layer session info
// ============================================================================

// SessionMetadata contains metadata about a session for transport
type SessionMetadata struct {
	SessionID  string            `json:"session_id"`
	UserID     string            `json:"user_id,omitempty"`
	ClientInfo ClientInfo        `json:"client_info"`
	StartTime  time.Time         `json:"start_time"`
	LastActive time.Time         `json:"last_active"`
	Properties map[string]string `json:"properties,omitempty"`
}

// ClientInfo contains information about the client
type ClientInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Type      string `json:"type"` // "cli", "web", "api", etc.
	UserAgent string `json:"user_agent,omitempty"`
	IP        string `json:"ip,omitempty"`
}

// ============================================================================
// Registry Types - Tool registration and discovery
// ============================================================================

// RegistryConfig is an alias for the canonical registry configuration
// NOTE: The canonical RegistryConfig is aliased in interfaces_compat.go

// ToolRegistration represents a registered tool
type ToolRegistration struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Version      string                 `json:"version"`
	Schema       map[string]interface{} `json:"schema"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	RegisteredAt time.Time              `json:"registered_at"`
}

// RegistryStats provides statistics about the tool registry
type RegistryStats struct {
	TotalTools     int                      `json:"total_tools"`
	ActiveTools    int                      `json:"active_tools"`
	ToolsByType    map[string]int           `json:"tools_by_type"`
	LastUpdated    time.Time                `json:"last_updated"`
	ExecutionStats map[string]ExecutionStat `json:"execution_stats,omitempty"`
}

// ExecutionStat tracks tool execution statistics
type ExecutionStat struct {
	TotalExecutions int           `json:"total_executions"`
	SuccessfulRuns  int           `json:"successful_runs"`
	FailedRuns      int           `json:"failed_runs"`
	AverageTime     time.Duration `json:"average_time"`
	LastExecution   time.Time     `json:"last_execution"`
}

// ============================================================================
// Workflow Types - Workflow execution in transport layer
// ============================================================================

// WorkflowExecution represents an executing workflow
type WorkflowExecution struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Status      WorkflowStatus          `json:"status"`
	Steps       []WorkflowStepExecution `json:"steps"`
	StartedAt   time.Time               `json:"started_at"`
	CompletedAt *time.Time              `json:"completed_at,omitempty"`
	Duration    time.Duration           `json:"duration"`
	Error       string                  `json:"error,omitempty"`
	Metadata    map[string]interface{}  `json:"metadata,omitempty"`
}

// WorkflowStatus represents the status of workflow execution
type WorkflowStatus string

const (
	WorkflowPending   WorkflowStatus = "pending"
	WorkflowRunning   WorkflowStatus = "running"
	WorkflowCompleted WorkflowStatus = "completed"
	WorkflowFailed    WorkflowStatus = "failed"
	WorkflowCancelled WorkflowStatus = "cancelled"
)

// WorkflowStepExecution represents the execution of a single workflow step
type WorkflowStepExecution struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	ToolName    string                 `json:"tool_name"`
	Status      WorkflowStatus         `json:"status"`
	Input       map[string]interface{} `json:"input,omitempty"`
	Output      map[string]interface{} `json:"output,omitempty"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Duration    time.Duration          `json:"duration"`
	Error       string                 `json:"error,omitempty"`
}

// ============================================================================
// Monitoring and Observability Types
// ============================================================================

// MetricsData represents metrics data for transport
type MetricsData struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Unit      string            `json:"unit,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// HealthStatus represents system health status
type HealthStatus struct {
	Status    string                 `json:"status"` // "healthy", "degraded", "unhealthy"
	Version   string                 `json:"version"`
	Uptime    time.Duration          `json:"uptime"`
	Checks    map[string]HealthCheck `json:"checks"`
	Timestamp time.Time              `json:"timestamp"`
}

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	Name      string        `json:"name"`
	Status    string        `json:"status"`
	Message   string        `json:"message,omitempty"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
}
