package runtime

import (
	"context"
	"fmt"
	"strings"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
	"github.com/rs/zerolog"
)

// AtomicToolBase provides common functionality for all atomic tools
type AtomicToolBase struct {
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  *session.SessionManager
	validationMixin *utils.StandardizedValidationMixin
	logger          zerolog.Logger
	name            string // Tool name for logging
}

// NewAtomicToolBase creates a new atomic tool base
func NewAtomicToolBase(
	name string,
	adapter mcptypes.PipelineOperations,
	sessionManager *session.SessionManager,
	logger zerolog.Logger,
) *AtomicToolBase {
	toolLogger := logger.With().Str("tool", name).Logger()
	return &AtomicToolBase{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		validationMixin: utils.NewStandardizedValidationMixin(toolLogger),
		logger:          toolLogger,
		name:            name,
	}
}

// ValidatedExecution represents a validated session and tool execution context
type ValidatedExecution struct {
	Session      interface{}
	SessionID    string
	WorkspaceDir string
	Logger       zerolog.Logger
}

// ValidateAndPrepareExecution performs common validation and preparation steps
func (base *AtomicToolBase) ValidateAndPrepareExecution(
	ctx context.Context,
	sessionID string,
	requiredFields []string,
	args interface{},
) (*ValidatedExecution, error) {
	// Validate required fields if specified
	if len(requiredFields) > 0 {
		validationResult := base.validationMixin.StandardValidateRequiredFields(args, requiredFields)
		if validationResult.HasErrors() {
			base.logger.Error().Interface("validation_errors", validationResult.Errors).Msg("Input validation failed")
			return nil, fmt.Errorf("atomic tool operation failed")
		}
	}

	// Validate session ID
	if strings.TrimSpace(sessionID) == "" {
		base.logger.Error().Msg("Session ID is required and cannot be empty")
		return nil, fmt.Errorf("atomic tool operation failed")
	}

	// Get session using our *session.SessionManager interface
	session, err := base.sessionManager.GetSession(sessionID)
	if err != nil {
		base.logger.Error().Err(err).Str("session_id", sessionID).Msg("Failed to get session")
		return nil, fmt.Errorf("atomic tool operation failed")
	}

	// Get workspace directory - use pipeline adapter method
	workspaceDir := base.pipelineAdapter.GetSessionWorkspace(sessionID)

	// Create execution context
	execution := &ValidatedExecution{
		Session:      session,
		SessionID:    sessionID,
		WorkspaceDir: workspaceDir,
		Logger: base.logger.With().
			Str("session_id", sessionID).
			Str("workspace", workspaceDir).
			Logger(),
	}

	base.logger.Info().
		Str("session_id", execution.SessionID).
		Str("workspace_dir", execution.WorkspaceDir).
		Msgf("Starting %s operation", base.name)

	return execution, nil
}

// GetPipelineAdapter returns the pipeline adapter
func (base *AtomicToolBase) GetPipelineAdapter() mcptypes.PipelineOperations {
	return base.pipelineAdapter
}

// GetSessionManager returns the session manager
func (base *AtomicToolBase) GetSessionManager() *session.SessionManager {
	return base.sessionManager
}

// GetValidationMixin returns the validation mixin
func (base *AtomicToolBase) GetValidationMixin() *utils.StandardizedValidationMixin {
	return base.validationMixin
}

// GetLogger returns the tool logger
func (base *AtomicToolBase) GetLogger() zerolog.Logger {
	return base.logger
}

// GetName returns the tool name
func (base *AtomicToolBase) GetName() string {
	return base.name
}

// LogOperationStart logs the start of a tool operation with standard fields
func (base *AtomicToolBase) LogOperationStart(operation string, details map[string]interface{}) {
	event := base.logger.Info().Str("operation", operation)
	for key, value := range details {
		switch v := value.(type) {
		case string:
			event = event.Str(key, v)
		case int:
			event = event.Int(key, v)
		case bool:
			event = event.Bool(key, v)
		case float64:
			event = event.Float64(key, v)
		default:
			event = event.Interface(key, v)
		}
	}
	event.Msgf("Starting %s operation", operation)
}

// LogOperationComplete logs the completion of a tool operation
func (base *AtomicToolBase) LogOperationComplete(operation string, success bool, duration interface{}) {
	event := base.logger.Info().
		Str("operation", operation).
		Bool("success", success)

	if duration != nil {
		event = event.Interface("duration", duration)
	}

	if success {
		event.Msgf("Completed %s operation successfully", operation)
	} else {
		event.Msgf("Failed %s operation", operation)
	}
}
