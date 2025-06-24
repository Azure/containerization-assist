package fixing

import (
	"context"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
)

// FixAttempt represents a single attempt to fix a failure
type FixAttempt struct {
	AttemptNumber  int           `json:"attempt_number"`
	StartTime      time.Time     `json:"start_time"`
	EndTime        time.Time     `json:"end_time"`
	Duration       time.Duration `json:"duration"`
	FixStrategy    FixStrategy   `json:"fix_strategy"`
	FixedContent   string        `json:"fixed_content,omitempty"`
	Success        bool          `json:"success"`
	Error          error         `json:"error,omitempty"`
	AnalysisPrompt string        `json:"analysis_prompt"`
	AnalysisResult string        `json:"analysis_result"`
}

// FixStrategy defines how a failure should be addressed
type FixStrategy struct {
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	Priority      int          `json:"priority"` // 1-10, 1 being highest
	Type          string       `json:"type"`     // dockerfile, manifest, config, etc.
	Dependencies  []string     `json:"dependencies,omitempty"`
	Commands      []string     `json:"commands,omitempty"`
	FileChanges   []FileChange `json:"file_changes,omitempty"`
	Validation    string       `json:"validation"`
	EstimatedTime string       `json:"estimated_time"`
}

// FileChange represents a change to be made to a file
type FileChange struct {
	FilePath   string `json:"file_path"`
	Operation  string `json:"operation"` // create, update, delete
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content"`
	Reason     string `json:"reason"`
}

// FixingContext contains all context needed for fixing operations
type FixingContext struct {
	SessionID       string                 `json:"session_id"`
	WorkspaceDir    string                 `json:"workspace_dir"`
	ToolName        string                 `json:"tool_name"`
	OperationType   string                 `json:"operation_type"` // build, deploy, scan, etc.
	OriginalError   error                  `json:"original_error"`
	ErrorDetails    *types.RichError       `json:"error_details,omitempty"`
	AttemptHistory  []FixAttempt           `json:"attempt_history"`
	MaxAttempts     int                    `json:"max_attempts"`
	BaseDir         string                 `json:"base_dir"`
	EnvironmentInfo map[string]interface{} `json:"environment_info"`
	SessionMetadata map[string]interface{} `json:"session_metadata"`
}

// FixingResult contains the outcome of a fixing operation
type FixingResult struct {
	Success         bool          `json:"success"`
	TotalAttempts   int           `json:"total_attempts"`
	TotalDuration   time.Duration `json:"total_duration"`
	FinalAttempt    *FixAttempt   `json:"final_attempt,omitempty"`
	AllAttempts     []FixAttempt  `json:"all_attempts"`
	RecommendedNext []string      `json:"recommended_next"`
	Error           error         `json:"error,omitempty"`
}

// IterativeFixer provides AI-driven iterative fixing capabilities
type IterativeFixer interface {
	// AttemptFix tries to fix a failure using AI analysis
	AttemptFix(ctx context.Context, fixingCtx *FixingContext) (*FixingResult, error)

	// GetFixStrategies analyzes an error and returns potential fix strategies
	GetFixStrategies(ctx context.Context, fixingCtx *FixingContext) ([]FixStrategy, error)

	// ApplyFix applies a specific fix strategy
	ApplyFix(ctx context.Context, fixingCtx *FixingContext, strategy FixStrategy) (*FixAttempt, error)

	// ValidateFix checks if a fix was successful
	ValidateFix(ctx context.Context, fixingCtx *FixingContext, attempt *FixAttempt) (bool, error)
}

// FixingConfigurationProvider provides tool-specific fixing configuration
type FixingConfigurationProvider interface {
	// GetMaxAttempts returns the maximum number of fix attempts for this tool
	GetMaxAttempts() int

	// GetFixingPromptTemplate returns the prompt template for AI analysis
	GetFixingPromptTemplate(operationType string) string

	// GetValidationSteps returns steps to validate if a fix worked
	GetValidationSteps(operationType string) []string

	// ShouldRetryAfterFailure determines if fixing should be attempted
	ShouldRetryAfterFailure(err error, attempt int) bool
}

// ContextSharer enables cross-tool context sharing for failure routing
type ContextSharer interface {
	// ShareContext saves context for other tools to use
	ShareContext(ctx context.Context, sessionID string, contextType string, data interface{}) error

	// GetSharedContext retrieves shared context
	GetSharedContext(ctx context.Context, sessionID string, contextType string) (interface{}, error)

	// GetFailureRouting determines which tool should handle a specific failure
	GetFailureRouting(ctx context.Context, sessionID string, failure *types.RichError) (string, error)
}

// FixableOperation represents an operation that can be fixed iteratively
type FixableOperation interface {
	// ExecuteOnce performs the operation once
	ExecuteOnce(ctx context.Context) error

	// GetFailureAnalysis analyzes why the operation failed
	GetFailureAnalysis(ctx context.Context, err error) (*types.RichError, error)

	// PrepareForRetry prepares the operation for another attempt
	PrepareForRetry(ctx context.Context, fixAttempt *FixAttempt) error
}
