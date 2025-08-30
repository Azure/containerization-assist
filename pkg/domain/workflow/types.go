// Package workflow provides domain types and interfaces for containerization workflow operations.
// This package contains the core business logic for orchestrating containerization workflows,
// including step execution, progress tracking, and error handling.
//
// The workflow domain follows a clean architecture pattern where:
//   - Orchestrator manages the overall workflow execution
//   - Steps implement individual workflow operations
//   - State maintains workflow context across steps
//   - Progress tracks workflow execution status
//
// Key workflow capabilities:
//   - Multi-step containerization process (analyze, build, deploy, etc.)
//   - Progress tracking with real-time updates
//   - Error recovery with intelligent retry logic
//   - Session persistence for long-running workflows
package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/containerization-assist/pkg/api"
)

// Workflow type constants
const (
	// WorkflowTypeContainerization represents the main containerization workflow
	WorkflowTypeContainerization = "containerization"
)

// Step name constants
const (
	StepAnalyzeRepository  = "analyze_repository"
	StepGenerateDockerfile = "generate_dockerfile"
	StepResolveBaseImages  = "resolve_base_images"
	StepBuildImage         = "build_image"
	StepSecurityScan       = "security_scan"
	StepTagImage           = "tag_image"
	StepPushImage          = "push_image"
	StepGenerateManifests  = "generate_k8s_manifests"
	StepSetupCluster       = "setup_cluster"
	StepDeployApplication  = "deploy_application"
	StepVerifyDeployment   = "verify_deployment"
)

// TypedArgs represents strongly typed tool arguments for MCP tool invocations.
// This provides a type-safe wrapper around raw JSON data while maintaining
// flexibility for different argument structures.
type TypedArgs struct {
	// Data contains the raw JSON payload for the tool arguments
	Data json.RawMessage `json:"data"`
}

// TypedResult represents strongly typed tool results returned from MCP tool executions.
// This standardizes the response format across all workflow tools while allowing
// flexible data payloads.
type TypedResult struct {
	// Success indicates whether the tool execution completed successfully
	Success bool `json:"success"`
	// Data contains the tool's result payload as raw JSON
	Data json.RawMessage `json:"data,omitempty"`
	// Error contains the error message if execution failed
	Error string `json:"error,omitempty"`
}

// ChatArgs represents typed arguments for chat-based workflow interactions.
// This enables conversational interfaces for workflow operations and debugging.
type ChatArgs struct {
	// Message is the user's input message or query
	Message string `json:"message"`
	// SessionID identifies the chat session for context continuity
	SessionID string `json:"session_id,omitempty"`
	// Context provides additional metadata for the conversation
	Context map[string]string `json:"context,omitempty"`
}

// WorkflowArgs represents typed arguments for workflow operations.
// This provides a generic structure for workflow-level configuration and execution.
type WorkflowArgs struct {
	// WorkflowName identifies the type of workflow to execute
	WorkflowName string `json:"workflow_name,omitempty"`
	// WorkflowSpec contains the workflow configuration as raw JSON
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

// ServerConfig represents simplified server configuration with only essential fields
type ServerConfig struct {
	// Core server settings (essential)
	WorkspaceDir string        `json:"workspace_dir"`
	StorePath    string        `json:"store_path"`
	SessionTTL   time.Duration `json:"session_ttl"`

	// Session management (essential for functionality)
	MaxSessions int `json:"max_sessions"`

	// Logging settings (essential)
	LogLevel string `json:"log_level"`
	LogFile  string `json:"log_file"`

	// Service identification (essential)
	ServiceName    string `json:"service_name"`
	ServiceVersion string `json:"service_version"`

	// Container registry (essential)
	RegistryURL      string `json:"registry_url"`
	RegistryUsername string `json:"registry_username"`
	RegistryPassword string `json:"registry_password"`

	// Workflow mode (essential)
	WorkflowMode string `json:"workflow_mode"`
}

// DefaultServerConfig returns a simplified default server configuration with only essential fields
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		WorkspaceDir:     filepath.Join(os.TempDir(), "containerization-assist-workspace"),
		StorePath:        filepath.Join(os.TempDir(), "containerization-assist-sessions.db"),
		SessionTTL:       24 * time.Hour,
		MaxSessions:      100,
		LogLevel:         "info",
		LogFile:          "",
		ServiceName:      "containerization-assist-mcp",
		ServiceVersion:   "dev",
		RegistryURL:      "",
		RegistryUsername: "",
		RegistryPassword: "",
		WorkflowMode:     "interactive", // Default to interactive mode
	}
}

// ============================================================================
// Core Workflow Types (moved from legacy_orchestrator.go)
// ============================================================================

// StepResult represents the result of a step execution with minimal data and metadata
type StepResult struct {
	Success  bool                   `json:"success"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Validate ensures the StepResult is properly formed
func (sr *StepResult) Validate() error {
	// StepResult is always considered valid - validation is optional
	return nil
}

// Step defines the interface for individual workflow steps
type Step interface {
	Name() string
	Execute(ctx context.Context, state *WorkflowState) (*StepResult, error)
}

// WorkflowState holds all the state that flows between workflow steps
type WorkflowState struct {
	// Workflow identification
	WorkflowID string

	// Input arguments
	Args *ContainerizeAndDeployArgs

	// Cached repository identifier (computed once from Args)
	RepoIdentifier string

	// Result object that accumulates information
	Result *ContainerizeAndDeployResult

	// Step outputs
	AnalyzeResult    *AnalyzeResult
	DockerfileResult *DockerfileResult
	BuildResult      *BuildResult
	K8sResult        *K8sResult
	ScanReport       map[string]interface{}

	// Progress tracking
	ProgressEmitter  api.ProgressEmitter
	WorkflowProgress *WorkflowProgress
	CurrentStep      int
	TotalSteps       int

	// Utilities
	Logger *slog.Logger

	// AI Enhancement fields
	allSteps []Step // Cache of all workflow steps

	// Fixing context for redirect mechanism
	FixingMode    bool   `json:"fixing_mode,omitempty"`
	PreviousError string `json:"previous_error,omitempty"`
	FailedTool    string `json:"failed_tool,omitempty"`
	// Additional request parameters for AI-generated content
	RequestParams map[string]interface{} `json:"request_params,omitempty"`
}

// GetAllSteps returns all workflow steps (used for optimization analysis)
func (ws *WorkflowState) GetAllSteps() []Step {
	return ws.allSteps
}

// SetAllSteps sets all workflow steps (called during initialization)
func (ws *WorkflowState) SetAllSteps(steps []Step) {
	ws.allSteps = steps
}

// IsTestMode determines whether the current workflow execution is running in test mode.
// Precedence: Args.TestMode > RequestParams["test_mode"] > CONTAINERIZATION_ASSIST_TEST_MODE env var.
func (ws *WorkflowState) IsTestMode() bool {
	if ws != nil && ws.Args != nil && ws.Args.TestMode {
		return true
	}
	if ws != nil && ws.RequestParams != nil {
		if v, ok := ws.RequestParams["test_mode"].(bool); ok && v {
			return true
		}
	}
	return os.Getenv("CONTAINERIZATION_ASSIST_TEST_MODE") == "true"
}

// UpdateProgress advances the progress emitter and returns progress info
func (ws *WorkflowState) UpdateProgress() (int, string) {
	ws.CurrentStep++
	progress := fmt.Sprintf("%d/%d", ws.CurrentStep, ws.TotalSteps)
	percentage := int((float64(ws.CurrentStep) / float64(ws.TotalSteps)) * 100)

	// Emit progress update
	_ = ws.ProgressEmitter.Emit(context.Background(), "step", percentage, progress)

	return percentage, progress
}

// AddStepResult adds a step result to the workflow result
func (ws *WorkflowState) AddStepResult(name, status, duration, message string, retries int, err error) {
	step := WorkflowStep{
		Name:     name,
		Status:   status,
		Duration: duration,
		Progress: fmt.Sprintf("%d/%d", ws.CurrentStep, ws.TotalSteps),
		Message:  message,
		Retries:  retries,
	}

	if err != nil {
		step.Error = err.Error()
	}

	ws.Result.Steps = append(ws.Result.Steps, step)
}
