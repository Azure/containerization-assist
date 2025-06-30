package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

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

// ProgressCallback is defined in registry.go

// executeWithoutProgress executes an operation without progress tracking
// This is the base method that BuildSecBot's atomic tools can use
func (base *AtomicToolBase) ExecuteWithoutProgress(ctx context.Context, sessionID string, operation func() error) error {
	// Start tracking the tool execution
	if base.sessionManager != nil {
		if err := base.sessionManager.TrackToolExecution(sessionID, base.name, nil); err != nil {
			base.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to track tool execution start")
		}
	}

	base.logger.Info().
		Str("session_id", sessionID).
		Str("tool", base.name).
		Msg("Starting atomic tool execution without progress")

	startTime := time.Now()
	err := operation()
	duration := time.Since(startTime)

	// Complete the tool execution tracking
	if base.sessionManager != nil {
		success := err == nil
		if trackErr := base.sessionManager.CompleteToolExecution(sessionID, base.name, success, err, 0); trackErr != nil {
			base.logger.Warn().Err(trackErr).Str("session_id", sessionID).Msg("Failed to complete tool execution tracking")
		}
	}

	if err != nil {
		base.logger.Error().
			Err(err).
			Str("session_id", sessionID).
			Str("tool", base.name).
			Dur("duration", duration).
			Msg("Atomic tool execution failed")

		// Track the error
		if base.sessionManager != nil {
			if trackErr := base.sessionManager.TrackError(sessionID, err, map[string]interface{}{
				"tool":     base.name,
				"duration": duration.String(),
			}); trackErr != nil {
				base.logger.Warn().Err(trackErr).Msg("Failed to track error")
			}
		}

		return err
	}

	base.logger.Info().
		Str("session_id", sessionID).
		Str("tool", base.name).
		Dur("duration", duration).
		Msg("Atomic tool execution completed successfully")

	return nil
}

// ExecuteWithProgress executes an operation with progress tracking
func (base *AtomicToolBase) ExecuteWithProgress(ctx context.Context, sessionID string, operation func(ProgressCallback) error) error {
	// Start tracking the tool execution
	if base.sessionManager != nil {
		if err := base.sessionManager.TrackToolExecution(sessionID, base.name, nil); err != nil {
			base.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to track tool execution start")
		}
	}

	base.logger.Info().
		Str("session_id", sessionID).
		Str("tool", base.name).
		Msg("Starting atomic tool execution with progress tracking")

	// Create a progress callback that logs to the session
	progressCallback := func(stage string, percent float64, message string) {
		base.logger.Debug().
			Str("stage", stage).
			Float64("percent", percent).
			Str("message", message).
			Str("tool", base.name).
			Str("session_id", sessionID).
			Msg("Tool progress update")
	}

	startTime := time.Now()
	err := operation(progressCallback)
	duration := time.Since(startTime)

	// Complete the tool execution tracking
	if base.sessionManager != nil {
		success := err == nil
		if trackErr := base.sessionManager.CompleteToolExecution(sessionID, base.name, success, err, 0); trackErr != nil {
			base.logger.Warn().Err(trackErr).Str("session_id", sessionID).Msg("Failed to complete tool execution tracking")
		}
	}

	if err != nil {
		base.logger.Error().
			Err(err).
			Str("session_id", sessionID).
			Str("tool", base.name).
			Dur("duration", duration).
			Msg("Atomic tool execution with progress failed")

		// Track the error
		if base.sessionManager != nil {
			if trackErr := base.sessionManager.TrackError(sessionID, err, map[string]interface{}{
				"tool":     base.name,
				"duration": duration.String(),
			}); trackErr != nil {
				base.logger.Warn().Err(trackErr).Msg("Failed to track error")
			}
		}

		return err
	}

	base.logger.Info().
		Str("session_id", sessionID).
		Str("tool", base.name).
		Dur("duration", duration).
		Msg("Atomic tool execution with progress completed successfully")

	return nil
}

// StartJob starts a background job for long-running operations
func (base *AtomicToolBase) StartJob(sessionID, jobType string) (string, error) {
	if base.sessionManager == nil {
		return "", nil // Gracefully handle missing session manager
	}

	jobID, err := base.sessionManager.StartJob(sessionID, jobType)
	if err != nil {
		base.logger.Error().Err(err).Str("session_id", sessionID).Str("job_type", jobType).Msg("Failed to start job")
		return "", err
	}

	base.logger.Info().
		Str("session_id", sessionID).
		Str("job_id", jobID).
		Str("job_type", jobType).
		Msg("Started background job")

	return jobID, nil
}

// CompleteJob marks a job as completed
func (base *AtomicToolBase) CompleteJob(sessionID, jobID string, result interface{}) error {
	if base.sessionManager == nil {
		return nil // Gracefully handle missing session manager
	}

	err := base.sessionManager.CompleteJob(sessionID, jobID, result)
	if err != nil {
		base.logger.Error().Err(err).Str("session_id", sessionID).Str("job_id", jobID).Msg("Failed to complete job")
		return err
	}

	base.logger.Info().
		Str("session_id", sessionID).
		Str("job_id", jobID).
		Msg("Completed background job")

	return nil
}

// UpdateJobStatus updates the status of a running job
func (base *AtomicToolBase) UpdateJobStatus(sessionID, jobID string, status session.JobStatus, result interface{}, err error) error {
	if base.sessionManager == nil {
		return nil // Gracefully handle missing session manager
	}

	updateErr := base.sessionManager.UpdateJobStatus(sessionID, jobID, status, result, err)
	if updateErr != nil {
		base.logger.Error().Err(updateErr).Str("session_id", sessionID).Str("job_id", jobID).Msg("Failed to update job status")
		return updateErr
	}

	base.logger.Debug().
		Str("session_id", sessionID).
		Str("job_id", jobID).
		Str("status", string(status)).
		Msg("Updated job status")

	return nil
}
