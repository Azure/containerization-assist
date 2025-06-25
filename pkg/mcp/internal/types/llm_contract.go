package contract

import (
	"time"
)

// PromptEnvelope is sent to the hosting LLM for guided conversation flow
// This defines the formal contract between MCP server and hosting LLM
type PromptEnvelope struct {
	SessionID           string              `json:"session_id"`
	CorrelationID       string              `json:"correlation_id"` // For tracing and debugging
	Stage               string              `json:"stage"`
	UserMessage         string              `json:"user_message"`
	Context             ConversationContext `json:"context"`
	AvailableTools      []ToolDescriptor    `json:"available_tools"`
	SystemPrompt        string              `json:"system_prompt"`
	PossibleTransitions []StageTransition   `json:"possible_transitions"`
	Timestamp           time.Time           `json:"timestamp"`
}

// ToolCall is populated by the LLM and executed by MCP
// CRITICAL CONTRACT RULE: Only the LLM populates ToolCall.Arguments; MCP never calls out to another model
type ToolCall struct {
	ToolName      string                 `json:"tool_name"`
	Arguments     map[string]interface{} `json:"arguments"` // Only LLM populates this
	CallID        string                 `json:"call_id"`
	CorrelationID string                 `json:"correlation_id"` // Copied from PromptEnvelope for tracing
	Timestamp     time.Time              `json:"timestamp"`
}

// ToolResult is returned by MCP after executing a tool call
type ToolResult struct {
	CallID        string        `json:"call_id"`        // Matches ToolCall.CallID
	CorrelationID string        `json:"correlation_id"` // For tracing
	ToolName      string        `json:"tool_name"`
	Success       bool          `json:"success"`
	Result        interface{}   `json:"result,omitempty"`
	Error         string        `json:"error,omitempty"`
	ExecutionTime time.Duration `json:"execution_time"`
	Timestamp     time.Time     `json:"timestamp"`
}

// ConversationContext provides stateless context for LLM interactions
type ConversationContext struct {
	SessionID       string                 `json:"session_id"`
	WorkspacePath   string                 `json:"workspace_path"`
	CurrentStage    string                 `json:"current_stage"`
	PreviousStage   string                 `json:"previous_stage,omitempty"`
	CompletedStages []string               `json:"completed_stages"`
	CompletedTools  []string               `json:"completed_tools"`
	SessionState    map[string]interface{} `json:"session_state"`  // Persistent state
	StageMetadata   map[string]interface{} `json:"stage_metadata"` // Current stage context
	UserPreferences UserPreferences        `json:"user_preferences"`
	ProjectContext  ProjectContext         `json:"project_context"`
	LastError       string                 `json:"last_error,omitempty"`
	RetryCount      int                    `json:"retry_count"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// UserPreferences captures user's workflow preferences for consistent experience
type UserPreferences struct {
	PreferredRegistry   string            `json:"preferred_registry,omitempty"`
	PreferredNamespace  string            `json:"preferred_namespace,omitempty"`
	DefaultOptimization string            `json:"default_optimization,omitempty"` // size, speed, security
	SkipConfirmations   bool              `json:"skip_confirmations"`
	VerboseOutput       bool              `json:"verbose_output"`
	AutoCleanup         bool              `json:"auto_cleanup"`
	CustomBuildArgs     map[string]string `json:"custom_build_args,omitempty"`
	TargetPlatform      string            `json:"target_platform,omitempty"`
}

// ProjectContext captures detected information about the project being containerized
type ProjectContext struct {
	Language           string            `json:"language,omitempty"`
	Framework          string            `json:"framework,omitempty"`
	Version            string            `json:"version,omitempty"`
	Dependencies       []string          `json:"dependencies,omitempty"`
	BuildTool          string            `json:"build_tool,omitempty"`
	EntryPoint         string            `json:"entry_point,omitempty"`
	ExposedPorts       []int             `json:"exposed_ports,omitempty"`
	EnvironmentVars    map[string]string `json:"environment_vars,omitempty"`
	RequiredSecrets    []string          `json:"required_secrets,omitempty"`
	DetectedFiles      []string          `json:"detected_files,omitempty"`
	EstimatedImageSize string            `json:"estimated_image_size,omitempty"`
}

// StageTransition defines possible state transitions to simplify LLM prompts
type StageTransition struct {
	FromStage   string `json:"from_stage"`
	ToStage     string `json:"to_stage"`
	Trigger     string `json:"trigger"` // "success", "retry", "skip", "fail", "user_choice"
	Description string `json:"description"`
	Condition   string `json:"condition,omitempty"` // Optional condition for transition
}

// ToolDescriptor describes available tools to the LLM (derived from tool registry)
type ToolDescriptor struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Version      string                 `json:"version"`
	InputSchema  map[string]interface{} `json:"input_schema"`
	OutputSchema map[string]interface{} `json:"output_schema"`
	Capabilities ToolCapabilities       `json:"capabilities"`
	Stage        string                 `json:"stage,omitempty"` // Which stage this tool is available in
}

// ConversationResponse is sent back to the user during conversation flow
type ConversationResponse struct {
	SessionID     string                 `json:"session_id"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	Stage         string                 `json:"stage"`
	Message       string                 `json:"message"`
	Options       []ResponseOption       `json:"options,omitempty"`     // User choices
	Suggestions   []string               `json:"suggestions,omitempty"` // Auto-complete suggestions
	Progress      *ProgressIndicator     `json:"progress,omitempty"`    // Overall progress
	NextStage     string                 `json:"next_stage,omitempty"`  // Expected next stage
	RequiresInput bool                   `json:"requires_input"`        // Whether user input is needed
	Metadata      map[string]interface{} `json:"metadata,omitempty"`    // Additional context
	Timestamp     time.Time              `json:"timestamp"`
}

// ResponseOption represents a choice the user can make
type ResponseOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Default     bool   `json:"default,omitempty"`
}

// ProgressIndicator shows overall conversation progress
type ProgressIndicator struct {
	CurrentStep int    `json:"current_step"`
	TotalSteps  int    `json:"total_steps"`
	StageLabel  string `json:"stage_label"`
	Percentage  int    `json:"percentage"`
}

// LLMResponse represents the LLM's response to a PromptEnvelope
type LLMResponse struct {
	CorrelationID   string                 `json:"correlation_id"`
	ResponseMessage string                 `json:"response_message"`      // Message to show user
	ToolCalls       []ToolCall             `json:"tool_calls,omitempty"`  // Tools to execute
	NextStage       string                 `json:"next_stage,omitempty"`  // Stage transition
	RequiresInput   bool                   `json:"requires_input"`        // Whether to wait for user
	Confidence      float64                `json:"confidence,omitempty"`  // LLM confidence (0-1)
	Reasoning       string                 `json:"reasoning,omitempty"`   // LLM's reasoning (for debugging)
	Metadata        map[string]interface{} `json:"metadata,omitempty"`    // Additional context
	TokenUsage      *TokenUsage            `json:"token_usage,omitempty"` // Token usage metrics
	Timestamp       time.Time              `json:"timestamp"`
}

// TokenUsage tracks LLM token consumption for cost tracking
type TokenUsage struct {
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	Model            string `json:"model,omitempty"`
}

// ContractValidationError represents validation errors in the LLM contract
type ContractValidationError struct {
	Field      string `json:"field"`
	Message    string `json:"message"`
	Severity   string `json:"severity"` // "error", "warning"
	Suggestion string `json:"suggestion,omitempty"`
}

// Conversation stages defined as constants for consistency
const (
	StageWelcome    = "welcome"
	StageAnalysis   = "analysis"
	StageDockerfile = "dockerfile"
	StageManifests  = "manifests"
	StageBuild      = "build"
	StageDeploy     = "deploy"
	StageValidation = "validation"
	StageCleanup    = "cleanup"
	StageComplete   = "complete"
	StageError      = "error"
)

// Transition triggers
const (
	TriggerSuccess    = "success"
	TriggerRetry      = "retry"
	TriggerSkip       = "skip"
	TriggerFail       = "fail"
	TriggerUserChoice = "user_choice"
	TriggerTimeout    = "timeout"
)
