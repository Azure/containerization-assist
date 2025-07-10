package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/Azure/container-kit/pkg/mcp/domain/logging"
)

// AtomicOperationFramework provides foundation for atomic tool operations
type AtomicOperationFramework struct {
	sessionManager session.SessionManager
	operations     *Operations
	logger         logging.Standards
}

// NewAtomicOperationFramework creates a new atomic operation framework
func NewAtomicOperationFramework(
	sessionManager session.SessionManager,
	operations *Operations,
	logger logging.Standards,
) *AtomicOperationFramework {
	return &AtomicOperationFramework{
		sessionManager: sessionManager,
		operations:     operations,
		logger:         logger.WithComponent("atomic_framework"),
	}
}

// AtomicOperationConfig configures an atomic operation
type AtomicOperationConfig struct {
	SessionID     string
	OperationType string
	DryRun        bool
	Timeout       time.Duration
	RetryCount    int
	Force         bool
	Metadata      map[string]interface{}
}

// AtomicOperationResult provides standardized result structure
type AtomicOperationResult struct {
	Success   bool                   `json:"success"`
	SessionID string                 `json:"session_id"`
	Operation string                 `json:"operation"`
	Duration  time.Duration          `json:"duration"`
	Result    interface{}            `json:"result"`
	Error     error                  `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata"`
	JobID     string                 `json:"job_id,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// ExecuteAtomicDockerPull executes atomic Docker pull operation
func (af *AtomicOperationFramework) ExecuteAtomicDockerPull(ctx context.Context, config AtomicOperationConfig, imageRef string) (*AtomicOperationResult, error) {
	startTime := time.Now()

	result := &AtomicOperationResult{
		SessionID: config.SessionID,
		Operation: "docker_pull",
		Timestamp: startTime,
		Metadata:  config.Metadata,
	}

	_, err := af.sessionManager.GetSessionTyped(config.SessionID)
	if err != nil {
		result.Success = false
		result.Error = errors.NewError().Message("session not found").Cause(err).Build()
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	af.logger.Info().
		Str("session_id", config.SessionID).
		Str("image_ref", imageRef).
		Bool("dry_run", config.DryRun).
		Msg("Starting atomic Docker pull operation")

	if config.DryRun {
		result.Success = true
		result.Result = map[string]interface{}{
			"dry_run": true,
			"message": fmt.Sprintf("Would pull image: %s", imageRef),
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	err = af.operations.PullDockerImage(config.SessionID, imageRef)
	result.Duration = time.Since(startTime)

	if err != nil {
		result.Success = false
		result.Error = err

		af.logger.Error().Err(err).Str("image_ref", imageRef).Msg("Atomic Docker pull failed")
		return result, err
	}

	result.Success = true
	result.Result = map[string]interface{}{
		"image_ref": imageRef,
		"pulled":    true,
	}

	af.logger.Info().
		Str("image_ref", imageRef).
		Str("duration", result.Duration.String()).
		Msg("Atomic Docker pull completed successfully")

	return result, nil
}

// ExecuteAtomicDockerPush executes atomic Docker push operation
func (af *AtomicOperationFramework) ExecuteAtomicDockerPush(ctx context.Context, config AtomicOperationConfig, imageRef string) (*AtomicOperationResult, error) {
	startTime := time.Now()

	result := &AtomicOperationResult{
		SessionID: config.SessionID,
		Operation: "docker_push",
		Timestamp: startTime,
		Metadata:  config.Metadata,
	}

	_, err := af.sessionManager.GetSessionTyped(config.SessionID)
	if err != nil {
		result.Success = false
		result.Error = errors.NewError().Message("session not found").Cause(err).Build()
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	af.logger.Info().
		Str("session_id", config.SessionID).
		Str("image_ref", imageRef).
		Bool("dry_run", config.DryRun).
		Msg("Starting atomic Docker push operation")

	if config.DryRun {
		result.Success = true
		result.Result = map[string]interface{}{
			"dry_run": true,
			"message": fmt.Sprintf("Would push image: %s", imageRef),
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	err = af.operations.PushDockerImage(config.SessionID, imageRef)
	result.Duration = time.Since(startTime)

	if err != nil {
		result.Success = false
		result.Error = err

		af.logger.Error().Err(err).Str("image_ref", imageRef).Msg("Atomic Docker push failed")
		return result, err
	}

	result.Success = true
	result.Result = map[string]interface{}{
		"image_ref": imageRef,
		"pushed":    true,
	}

	af.logger.Info().
		Str("image_ref", imageRef).
		Str("duration", result.Duration.String()).
		Msg("Atomic Docker push completed successfully")

	return result, nil
}

// ExecuteAtomicDockerTag executes atomic Docker tag operation
func (af *AtomicOperationFramework) ExecuteAtomicDockerTag(ctx context.Context, config AtomicOperationConfig, sourceRef, targetRef string) (*AtomicOperationResult, error) {
	startTime := time.Now()

	result := &AtomicOperationResult{
		SessionID: config.SessionID,
		Operation: "docker_tag",
		Timestamp: startTime,
		Metadata:  config.Metadata,
	}

	_, err := af.sessionManager.GetSessionTyped(config.SessionID)
	if err != nil {
		result.Success = false
		result.Error = errors.NewError().Message("session not found").Cause(err).Build()
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	af.logger.Info().
		Str("session_id", config.SessionID).
		Str("source_ref", sourceRef).
		Str("target_ref", targetRef).
		Bool("dry_run", config.DryRun).
		Msg("Starting atomic Docker tag operation")

	if config.DryRun {
		result.Success = true
		result.Result = map[string]interface{}{
			"dry_run": true,
			"message": fmt.Sprintf("Would tag image: %s -> %s", sourceRef, targetRef),
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	err = af.operations.TagDockerImage(config.SessionID, sourceRef, targetRef)
	result.Duration = time.Since(startTime)

	if err != nil {
		result.Success = false
		result.Error = err

		af.logger.Error().Err(err).
			Str("source_ref", sourceRef).
			Str("target_ref", targetRef).
			Msg("Atomic Docker tag failed")
		return result, err
	}

	result.Success = true
	result.Result = map[string]interface{}{
		"source_ref": sourceRef,
		"target_ref": targetRef,
		"tagged":     true,
	}

	af.logger.Info().
		Str("source_ref", sourceRef).
		Str("target_ref", targetRef).
		Str("duration", result.Duration.String()).
		Msg("Atomic Docker tag completed successfully")

	return result, nil
}

// GetSessionManager returns the session manager for external access
func (af *AtomicOperationFramework) GetSessionManager() session.SessionManager {
	return af.sessionManager
}

// GetOperations returns the operations instance for external access
func (af *AtomicOperationFramework) GetOperations() *Operations {
	return af.operations
}

// ValidateAtomicConfig validates atomic operation configuration
func (af *AtomicOperationFramework) ValidateAtomicConfig(config AtomicOperationConfig) error {
	if config.SessionID == "" {
		return errors.NewError().Messagef("session ID is required").Build()
	}

	if config.OperationType == "" {
		return errors.NewError().Messagef("operation type is required").Build()
	}

	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Minute
	}

	return nil
}
