package report

import (
	"time"

	"github.com/Azure/container-kit/pkg/pipeline"
)

// MCP Report Constants
const (
	MCPReportDirectory  = ".container-kit-mcp"
	MCPReportFileName   = "mcp_report.json"
	MCPMarkdownFileName = "mcp_report.md"
)

// MCPOutcome represents the outcome of MCP workflow execution
type MCPOutcome string

const (
	MCPOutcomeSuccess    MCPOutcome = "success"
	MCPOutcomeFailure    MCPOutcome = "failure"
	MCPOutcomeInProgress MCPOutcome = "in_progress"
	MCPOutcomeTimeout    MCPOutcome = "timeout"
)

// MCPStepResult represents the result of executing a single workflow step
type MCPStepResult struct {
	StepName     string                 `json:"step_name"`
	Status       string                 `json:"status"`
	StartTime    time.Time              `json:"start_time"`
	EndTime      *time.Time             `json:"end_time,omitempty"`
	Duration     string                 `json:"duration,omitempty"`
	Success      bool                   `json:"success"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Outputs      map[string]interface{} `json:"outputs,omitempty"`
	Artifacts    []GeneratedArtifact    `json:"artifacts,omitempty"`
	RetryCount   int                    `json:"retry_count"`
}

// GeneratedArtifact represents a file or resource created during workflow execution
type GeneratedArtifact struct {
	Type        string    `json:"type"`        // "dockerfile", "manifest", "image", etc.
	Path        string    `json:"path"`        // File path or resource identifier
	Description string    `json:"description"` // Human-readable description
	CreatedAt   time.Time `json:"created_at"`
}

// MCPStageVisit represents a visit to a workflow stage (equivalent to CLI StageVisit)
type MCPStageVisit struct {
	StageID    string              `json:"stage_id"`
	RetryCount int                 `json:"retry_count"`
	Outcome    pipeline.RunOutcome `json:"outcome"`
	StartTime  time.Time           `json:"start_time"`
	EndTime    *time.Time          `json:"end_time,omitempty"`
}

// MCPTokenUsage tracks token consumption during MCP workflow execution
type MCPTokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// MCPReportSummary provides high-level overview of the workflow execution
type MCPReportSummary struct {
	TotalSteps     int        `json:"total_steps"`
	CompletedSteps int        `json:"completed_steps"`
	FailedSteps    int        `json:"failed_steps"`
	SuccessRate    float64    `json:"success_rate"`
	TotalDuration  string     `json:"total_duration"`
	LastUpdated    time.Time  `json:"last_updated"`
	CurrentStep    int        `json:"current_step"`
	Outcome        MCPOutcome `json:"outcome"`
	IterationCount int        `json:"iteration_count"`
	TotalArtifacts int        `json:"total_artifacts"`
}

// MCPProgressiveReport is the main structure for MCP workflow reporting
type MCPProgressiveReport struct {
	WorkflowID        string                             `json:"workflow_id"`
	StartTime         time.Time                          `json:"start_time"`
	LastUpdated       time.Time                          `json:"last_updated"`
	Summary           MCPReportSummary                   `json:"summary"`
	StepResults       []MCPStepResult                    `json:"step_results"`
	DetectedDatabases []pipeline.DatabaseDetectionResult `json:"detected_databases"`
	TokenUsage        MCPTokenUsage                      `json:"token_usage"`
	StageHistory      []MCPStageVisit                    `json:"stage_history"`
	GeneratedFiles    []GeneratedArtifact                `json:"generated_files"`
	Metadata          map[string]interface{}             `json:"metadata,omitempty"`
}
