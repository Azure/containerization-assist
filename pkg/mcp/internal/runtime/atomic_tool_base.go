package runtime

import (
	"context"
	"fmt"
	"time"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/core/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/common/utils"
	"github.com/Azure/container-kit/pkg/mcp/internal/observability"
	"github.com/rs/zerolog"
)

// AtomicToolBase provides common functionality for all atomic tools
type AtomicToolBase struct {
	PipelineAdapter mcptypes.PipelineOperations        // Pipeline adapter (exported for direct access)
	SessionManager  *session.SessionManager            // Session manager (exported for direct access)
	ValidationMixin *utils.StandardizedValidationMixin // Validation mixin (exported for direct access)
	Logger          zerolog.Logger                     // Tool logger (exported for direct access)
	Name            string                             // Tool name (exported for direct access)
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
		PipelineAdapter: adapter,
		SessionManager:  sessionManager,
		ValidationMixin: utils.NewStandardizedValidationMixin(toolLogger),
		Logger:          toolLogger,
		Name:            name,
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
		validationResult := base.ValidationMixin.StandardValidateRequiredFields(args, requiredFields)
		if validationResult.HasErrors() {
			base.Logger.Error().Interface("validation_errors", validationResult.GetErrors()).Msg("Input validation failed")
			return nil, fmt.Errorf("atomic tool operation failed")
		}
	}

	// Get or create session - sessionID can be empty for auto-creation
	session, err := base.SessionManager.GetOrCreateSession(sessionID)
	if err != nil {
		base.Logger.Error().Err(err).Str("session_id", sessionID).Msg("Failed to get session")
		return nil, fmt.Errorf("atomic tool operation failed")
	}

	// Get workspace directory - use pipeline adapter method
	workspaceDir := base.PipelineAdapter.GetSessionWorkspace(sessionID)

	// Create execution context
	execution := &ValidatedExecution{
		Session:      session,
		SessionID:    sessionID,
		WorkspaceDir: workspaceDir,
		Logger: base.Logger.With().
			Str("session_id", sessionID).
			Str("workspace", workspaceDir).
			Logger(),
	}

	base.Logger.Info().
		Str("session_id", execution.SessionID).
		Str("workspace_dir", execution.WorkspaceDir).
		Msgf("Starting %s operation", base.Name)

	return execution, nil
}

// Note: All fields are now exported for direct access
// Use base.PipelineAdapter, base.SessionManager, base.ValidationMixin, base.Logger, base.Name directly

// LogOperationStart logs the start of a tool operation with standard fields
func (base *AtomicToolBase) LogOperationStart(operation string, details map[string]interface{}) {
	event := base.Logger.Info().Str("operation", operation)
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
	event := base.Logger.Info().
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

// ProgressCallback is defined in registry.go

// executeWithoutProgress executes an operation without progress tracking
// This is the base method that BuildSecBot's atomic tools can use
func (base *AtomicToolBase) ExecuteWithoutProgress(ctx context.Context, sessionID string, operation func() error) error {
	// Start tracking the tool execution
	if base.SessionManager != nil {
		if err := base.SessionManager.TrackToolExecution(sessionID, base.Name, nil); err != nil {
			base.Logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to track tool execution start")
		}
	}

	base.Logger.Info().
		Str("session_id", sessionID).
		Str("tool", base.Name).
		Msg("Starting atomic tool execution without progress")

	startTime := time.Now()
	err := operation()
	duration := time.Since(startTime)

	// Simplified: removed job tracking

	if err != nil {
		base.Logger.Error().
			Err(err).
			Str("session_id", sessionID).
			Str("tool", base.Name).
			Dur("duration", duration).
			Msg("Atomic tool execution failed")

		// Track the error
		if base.SessionManager != nil {
			if trackErr := base.SessionManager.TrackError(sessionID, err, map[string]interface{}{
				"tool":     base.Name,
				"duration": duration.String(),
			}); trackErr != nil {
				base.Logger.Warn().Err(trackErr).Msg("Failed to track error")
			}
		}

		return err
	}

	base.Logger.Info().
		Str("session_id", sessionID).
		Str("tool", base.Name).
		Dur("duration", duration).
		Msg("Atomic tool execution completed successfully")

	return nil
}

// ExecuteWithProgress executes an operation with progress tracking
func (base *AtomicToolBase) ExecuteWithProgress(ctx context.Context, sessionID string, operation func(observability.ProgressCallback) error) error {
	// Start tracking the tool execution
	if base.SessionManager != nil {
		if err := base.SessionManager.TrackToolExecution(sessionID, base.Name, nil); err != nil {
			base.Logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to track tool execution start")
		}
	}

	base.Logger.Info().
		Str("session_id", sessionID).
		Str("tool", base.Name).
		Msg("Starting atomic tool execution with progress tracking")

	// Create a progress callback that logs to the session
	progressCallback := func(percent float64, message string) {
		base.Logger.Debug().
			Float64("percent", percent).
			Str("message", message).
			Str("tool", base.Name).
			Str("session_id", sessionID).
			Msg("Tool progress update")
	}

	startTime := time.Now()
	err := operation(progressCallback)
	duration := time.Since(startTime)

	// Simplified: removed job tracking

	if err != nil {
		base.Logger.Error().
			Err(err).
			Str("session_id", sessionID).
			Str("tool", base.Name).
			Dur("duration", duration).
			Msg("Atomic tool execution with progress failed")

		// Track the error
		if base.SessionManager != nil {
			if trackErr := base.SessionManager.TrackError(sessionID, err, map[string]interface{}{
				"tool":     base.Name,
				"duration": duration.String(),
			}); trackErr != nil {
				base.Logger.Warn().Err(trackErr).Msg("Failed to track error")
			}
		}

		return err
	}

	base.Logger.Info().
		Str("session_id", sessionID).
		Str("tool", base.Name).
		Dur("duration", duration).
		Msg("Atomic tool execution with progress completed successfully")

	return nil
}

// Removed job management methods - simplified implementation
