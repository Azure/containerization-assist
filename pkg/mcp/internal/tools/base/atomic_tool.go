package base

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/api/contract"
	"github.com/Azure/container-copilot/pkg/mcp/internal/interfaces"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
	"github.com/rs/zerolog"
)

// AtomicTool defines the interface for all atomic tools
type AtomicTool interface {
	GetName() string
	GetDescription() string
	GetVersion() string
	GetCapabilities() contract.ToolCapabilities
	Validate(ctx context.Context, args interface{}) error
	Execute(ctx context.Context, args interface{}) (interface{}, error)
}

// SessionManager defines the interface for session management
type SessionManager interface {
	GetSession(sessionID string) (*sessiontypes.SessionState, error)
	GetOrCreateSession(sessionID string) (*sessiontypes.SessionState, error)
	UpdateSession(sessionID string, updateFunc func(*sessiontypes.SessionState)) error
}

// BaseAtomicTool provides common functionality for atomic tools
type BaseAtomicTool struct {
	Name           string
	Description    string
	Version        string
	Capabilities   contract.ToolCapabilities
	SessionManager SessionManager
	Logger         zerolog.Logger
}

// NewBaseAtomicTool creates a new base atomic tool
func NewBaseAtomicTool(name, description, version string, sessionManager SessionManager, logger zerolog.Logger) *BaseAtomicTool {
	return &BaseAtomicTool{
		Name:           name,
		Description:    description,
		Version:        version,
		SessionManager: sessionManager,
		Logger:         logger.With().Str("tool", name).Logger(),
		Capabilities: contract.ToolCapabilities{
			SupportsDryRun:    true,
			SupportsStreaming: false,
			IsLongRunning:     false,
			RequiresAuth:      false,
		},
	}
}

// GetName returns the tool name
func (t *BaseAtomicTool) GetName() string {
	return t.Name
}

// GetDescription returns the tool description
func (t *BaseAtomicTool) GetDescription() string {
	return t.Description
}

// GetVersion returns the tool version
func (t *BaseAtomicTool) GetVersion() string {
	return t.Version
}

// GetCapabilities returns the tool capabilities
func (t *BaseAtomicTool) GetCapabilities() contract.ToolCapabilities {
	return t.Capabilities
}

// ValidateSessionID validates that a session ID is provided
func (t *BaseAtomicTool) ValidateSessionID(sessionID string) error {
	if sessionID == "" {
		return types.NewValidationErrorBuilder("SessionID is required", "session_id", sessionID).
			WithField("field", "session_id").
			Build()
	}
	return nil
}

// GetSessionWithValidation gets a session and handles common error cases
func (t *BaseAtomicTool) GetSessionWithValidation(sessionID string) (*sessiontypes.SessionState, error) {
	if err := t.ValidateSessionID(sessionID); err != nil {
		return nil, err
	}

	session, err := t.SessionManager.GetSession(sessionID)
	if err != nil {
		return nil, types.NewRichError("SESSION_NOT_FOUND",
			fmt.Sprintf("session not found: %s", sessionID),
			types.ErrTypeSession)
	}

	return session, nil
}

// UpdateSessionState updates the session state with tool execution info
func (t *BaseAtomicTool) UpdateSessionState(session *sessiontypes.SessionState, toolName string, success bool, duration time.Duration) error {
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}

	// Add execution info
	now := time.Now()
	startTime := now.Add(-duration)
	execution := sessiontypes.ToolExecution{
		Tool:      toolName,
		StartTime: startTime,
		EndTime:   &now,
		Duration:  &duration,
		Success:   success,
		DryRun:    false,
	}
	session.AddToolExecution(execution)
	session.UpdateLastAccessed()

	// Update session
	return t.SessionManager.UpdateSession(session.SessionID, func(s *sessiontypes.SessionState) {
		*s = *session
	})
}

// BaseToolArgs provides common arguments for all tools
type BaseToolArgs struct {
	SessionID string `json:"session_id" description:"Session ID for the operation"`
	DryRun    bool   `json:"dry_run,omitempty" description:"Preview changes without applying them"`
}

// BaseToolResponse provides common response fields for all tools
type BaseToolResponse struct {
	ToolName  string        `json:"tool_name"`
	SessionID string        `json:"session_id"`
	Success   bool          `json:"success"`
	Duration  time.Duration `json:"duration"`
	DryRun    bool          `json:"dry_run"`
	Timestamp time.Time     `json:"timestamp"`
}

// NewBaseToolResponse creates a new base tool response
func NewBaseToolResponse(toolName, sessionID string, success bool, duration time.Duration, dryRun bool) BaseToolResponse {
	return BaseToolResponse{
		ToolName:  toolName,
		SessionID: sessionID,
		Success:   success,
		Duration:  duration,
		DryRun:    dryRun,
		Timestamp: time.Now(),
	}
}

// ProgressReporter provides progress reporting functionality
type ProgressReporter interface {
	ReportStage(progress float64, message string)
	NextStage(stageName string)
	Complete(message string)
}

// NoOpProgressReporter is a no-op implementation of ProgressReporter
type NoOpProgressReporter struct{}

func (n *NoOpProgressReporter) ReportStage(progress float64, message string) {}
func (n *NoOpProgressReporter) NextStage(stageName string)                   {}
func (n *NoOpProgressReporter) Complete(message string)                      {}

// StageProgressReporter implements progress reporting with stages
type StageProgressReporter struct {
	stages       []interfaces.ProgressStage
	currentStage int
	reporter     func(progress float64, message string)
	logger       zerolog.Logger
}

// NewStageProgressReporter creates a new stage-based progress reporter
func NewStageProgressReporter(stages []interfaces.ProgressStage, reporter func(float64, string), logger zerolog.Logger) *StageProgressReporter {
	return &StageProgressReporter{
		stages:       stages,
		currentStage: 0,
		reporter:     reporter,
		logger:       logger,
	}
}

// ReportStage reports progress within the current stage
func (r *StageProgressReporter) ReportStage(progress float64, message string) {
	if r.currentStage >= len(r.stages) {
		return
	}

	stage := r.stages[r.currentStage]

	// Calculate overall progress
	var baseProgress float64
	for i := 0; i < r.currentStage; i++ {
		baseProgress += r.stages[i].Weight
	}

	overallProgress := baseProgress + (progress * stage.Weight)

	r.logger.Debug().
		Str("stage", stage.Name).
		Float64("stage_progress", progress).
		Float64("overall_progress", overallProgress).
		Str("message", message).
		Msg("Progress update")

	if r.reporter != nil {
		r.reporter(overallProgress, message)
	}
}

// NextStage moves to the next stage
func (r *StageProgressReporter) NextStage(stageName string) {
	if r.currentStage < len(r.stages)-1 {
		r.currentStage++
		r.ReportStage(0.0, stageName)
	}
}

// Complete marks the operation as complete
func (r *StageProgressReporter) Complete(message string) {
	r.ReportStage(1.0, message)
}

// StandardStages provides common stage definitions
var (
	StandardValidationStages = []interfaces.ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Setting up validation environment"},
		{Name: "Read", Weight: 0.15, Description: "Reading content"},
		{Name: "Validate", Weight: 0.40, Description: "Running validation checks"},
		{Name: "Analyze", Weight: 0.25, Description: "Performing analysis"},
		{Name: "Finalize", Weight: 0.10, Description: "Generating results"},
	}

	StandardBuildStages = []interfaces.ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Setting up build environment"},
		{Name: "Prepare", Weight: 0.20, Description: "Preparing build context"},
		{Name: "Build", Weight: 0.50, Description: "Building image"},
		{Name: "Push", Weight: 0.15, Description: "Pushing to registry"},
		{Name: "Finalize", Weight: 0.05, Description: "Cleaning up"},
	}

	StandardGenerateStages = []interfaces.ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Setting up generation environment"},
		{Name: "Analyze", Weight: 0.20, Description: "Analyzing requirements"},
		{Name: "Generate", Weight: 0.50, Description: "Generating artifacts"},
		{Name: "Validate", Weight: 0.15, Description: "Validating output"},
		{Name: "Finalize", Weight: 0.05, Description: "Saving results"},
	}
)
