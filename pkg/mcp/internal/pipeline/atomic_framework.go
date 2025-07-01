package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/rs/zerolog"
)

// AtomicOperationFramework provides foundation for atomic tool operations
type AtomicOperationFramework struct {
	sessionManager *session.SessionManager
	operations     *Operations
	logger         zerolog.Logger
}

// NewAtomicOperationFramework creates a new atomic operation framework
func NewAtomicOperationFramework(
	sessionManager *session.SessionManager,
	operations *Operations,
	logger zerolog.Logger,
) *AtomicOperationFramework {
	return &AtomicOperationFramework{
		sessionManager: sessionManager,
		operations:     operations,
		logger:         logger.With().Str("component", "atomic_framework").Logger(),
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

	// Validate session exists
	_, err := af.sessionManager.GetSession(config.SessionID)
	if err != nil {
		result.Success = false
		result.Error = fmt.Errorf("session not found: %w", err)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	af.logger.Info().
		Str("session_id", config.SessionID).
		Str("image_ref", imageRef).
		Bool("dry_run", config.DryRun).
		Msg("Starting atomic Docker pull operation")

	// Handle dry-run mode
	if config.DryRun {
		result.Success = true
		result.Result = map[string]interface{}{
			"dry_run": true,
			"message": fmt.Sprintf("Would pull image: %s", imageRef),
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Start job tracking
	jobID, err := af.sessionManager.StartJob(config.SessionID, "atomic_docker_pull")
	if err != nil {
		af.logger.Warn().Err(err).Msg("Failed to start job tracking")
	} else {
		result.JobID = jobID
	}

	// Track tool execution
	af.sessionManager.TrackToolExecution(config.SessionID, "atomic_docker_pull", map[string]interface{}{
		"image_ref": imageRef,
		"timeout":   config.Timeout,
	})

	// Execute the pull operation
	err = af.operations.PullDockerImage(config.SessionID, imageRef)
	result.Duration = time.Since(startTime)

	if err != nil {
		result.Success = false
		result.Error = err

		// Update job status
		if jobID != "" {
			af.sessionManager.UpdateJobStatus(config.SessionID, jobID, "failed", nil, err)
		}

		// Complete tool execution with error
		af.sessionManager.CompleteToolExecution(config.SessionID, "atomic_docker_pull", false, err, 0)

		af.logger.Error().Err(err).Str("image_ref", imageRef).Msg("Atomic Docker pull failed")
		return result, err
	}

	// Success
	result.Success = true
	result.Result = map[string]interface{}{
		"image_ref": imageRef,
		"pulled":    true,
	}

	// Complete job
	if jobID != "" {
		af.sessionManager.CompleteJob(config.SessionID, jobID, result.Result)
	}

	// Complete tool execution
	af.sessionManager.CompleteToolExecution(config.SessionID, "atomic_docker_pull", true, nil, 0)

	af.logger.Info().
		Str("image_ref", imageRef).
		Dur("duration", result.Duration).
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

	// Validate session exists
	_, err := af.sessionManager.GetSession(config.SessionID)
	if err != nil {
		result.Success = false
		result.Error = fmt.Errorf("session not found: %w", err)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	af.logger.Info().
		Str("session_id", config.SessionID).
		Str("image_ref", imageRef).
		Bool("dry_run", config.DryRun).
		Msg("Starting atomic Docker push operation")

	// Handle dry-run mode
	if config.DryRun {
		result.Success = true
		result.Result = map[string]interface{}{
			"dry_run": true,
			"message": fmt.Sprintf("Would push image: %s", imageRef),
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Start job tracking
	jobID, err := af.sessionManager.StartJob(config.SessionID, "atomic_docker_push")
	if err != nil {
		af.logger.Warn().Err(err).Msg("Failed to start job tracking")
	} else {
		result.JobID = jobID
	}

	// Track tool execution
	af.sessionManager.TrackToolExecution(config.SessionID, "atomic_docker_push", map[string]interface{}{
		"image_ref": imageRef,
		"timeout":   config.Timeout,
	})

	// Execute the push operation
	err = af.operations.PushDockerImage(config.SessionID, imageRef)
	result.Duration = time.Since(startTime)

	if err != nil {
		result.Success = false
		result.Error = err

		// Update job status
		if jobID != "" {
			af.sessionManager.UpdateJobStatus(config.SessionID, jobID, "failed", nil, err)
		}

		// Complete tool execution with error
		af.sessionManager.CompleteToolExecution(config.SessionID, "atomic_docker_push", false, err, 0)

		af.logger.Error().Err(err).Str("image_ref", imageRef).Msg("Atomic Docker push failed")
		return result, err
	}

	// Success
	result.Success = true
	result.Result = map[string]interface{}{
		"image_ref": imageRef,
		"pushed":    true,
	}

	// Complete job
	if jobID != "" {
		af.sessionManager.CompleteJob(config.SessionID, jobID, result.Result)
	}

	// Complete tool execution
	af.sessionManager.CompleteToolExecution(config.SessionID, "atomic_docker_push", true, nil, 0)

	af.logger.Info().
		Str("image_ref", imageRef).
		Dur("duration", result.Duration).
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

	// Validate session exists
	_, err := af.sessionManager.GetSession(config.SessionID)
	if err != nil {
		result.Success = false
		result.Error = fmt.Errorf("session not found: %w", err)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	af.logger.Info().
		Str("session_id", config.SessionID).
		Str("source_ref", sourceRef).
		Str("target_ref", targetRef).
		Bool("dry_run", config.DryRun).
		Msg("Starting atomic Docker tag operation")

	// Handle dry-run mode
	if config.DryRun {
		result.Success = true
		result.Result = map[string]interface{}{
			"dry_run": true,
			"message": fmt.Sprintf("Would tag image: %s -> %s", sourceRef, targetRef),
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Start job tracking
	jobID, err := af.sessionManager.StartJob(config.SessionID, "atomic_docker_tag")
	if err != nil {
		af.logger.Warn().Err(err).Msg("Failed to start job tracking")
	} else {
		result.JobID = jobID
	}

	// Track tool execution
	af.sessionManager.TrackToolExecution(config.SessionID, "atomic_docker_tag", map[string]interface{}{
		"source_ref": sourceRef,
		"target_ref": targetRef,
	})

	// Execute the tag operation
	err = af.operations.TagDockerImage(config.SessionID, sourceRef, targetRef)
	result.Duration = time.Since(startTime)

	if err != nil {
		result.Success = false
		result.Error = err

		// Update job status
		if jobID != "" {
			af.sessionManager.UpdateJobStatus(config.SessionID, jobID, "failed", nil, err)
		}

		// Complete tool execution with error
		af.sessionManager.CompleteToolExecution(config.SessionID, "atomic_docker_tag", false, err, 0)

		af.logger.Error().Err(err).
			Str("source_ref", sourceRef).
			Str("target_ref", targetRef).
			Msg("Atomic Docker tag failed")
		return result, err
	}

	// Success
	result.Success = true
	result.Result = map[string]interface{}{
		"source_ref": sourceRef,
		"target_ref": targetRef,
		"tagged":     true,
	}

	// Complete job
	if jobID != "" {
		af.sessionManager.CompleteJob(config.SessionID, jobID, result.Result)
	}

	// Complete tool execution
	af.sessionManager.CompleteToolExecution(config.SessionID, "atomic_docker_tag", true, nil, 0)

	af.logger.Info().
		Str("source_ref", sourceRef).
		Str("target_ref", targetRef).
		Dur("duration", result.Duration).
		Msg("Atomic Docker tag completed successfully")

	return result, nil
}

// GetSessionManager returns the session manager for external access
func (af *AtomicOperationFramework) GetSessionManager() *session.SessionManager {
	return af.sessionManager
}

// GetOperations returns the operations instance for external access
func (af *AtomicOperationFramework) GetOperations() *Operations {
	return af.operations
}

// ValidateAtomicConfig validates atomic operation configuration
func (af *AtomicOperationFramework) ValidateAtomicConfig(config AtomicOperationConfig) error {
	if config.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	if config.OperationType == "" {
		return fmt.Errorf("operation type is required")
	}

	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Minute // Default timeout
	}

	return nil
}
