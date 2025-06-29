package build

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp"
	mcptypes "github.com/Azure/container-kit/pkg/mcp"
)

// OperationType represents the type of Docker operation
type OperationType string

const (
	OperationPull OperationType = "pull"
	OperationPush OperationType = "push"
	OperationTag  OperationType = "tag"
)

// DockerOperation provides a unified, configurable wrapper for all Docker operations
// (pull, push, tag) with built-in retry logic, progress reporting, and error analysis.
// This eliminates the need for separate PullOperationWrapper, PushOperationWrapper, and TagOperationWrapper.
type DockerOperation struct {
	// Operation identification
	Type OperationType
	Name string

	// Configuration
	RetryAttempts int
	Timeout       time.Duration

	// Dependencies
	Progress mcptypes.ProgressReporter

	// Operation-specific functions (configurable)
	ExecuteFunc  func(ctx context.Context) error
	AnalyzeFunc  func() error
	PrepareFunc  func() error
	ValidateFunc func() error

	// State
	lastError error
	attempt   int
}

// DockerOperationConfig provides configuration for creating a DockerOperation
type DockerOperationConfig struct {
	Type          OperationType
	Name          string
	RetryAttempts int
	Timeout       time.Duration

	ExecuteFunc  func(ctx context.Context) error
	AnalyzeFunc  func() error
	PrepareFunc  func() error
	ValidateFunc func() error
}

// NewDockerOperation creates a new generic Docker operation with the specified configuration
func NewDockerOperation(config DockerOperationConfig, progress mcptypes.ProgressReporter) mcptypes.FixableOperation {
	// Set default values
	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Minute
	}
	if config.Name == "" {
		config.Name = string(config.Type)
	}

	return &DockerOperation{
		Type:          config.Type,
		Name:          config.Name,
		RetryAttempts: config.RetryAttempts,
		Timeout:       config.Timeout,
		Progress:      progress,
		ExecuteFunc:   config.ExecuteFunc,
		AnalyzeFunc:   config.AnalyzeFunc,
		PrepareFunc:   config.PrepareFunc,
		ValidateFunc:  config.ValidateFunc,
	}
}

// ExecuteOnce runs the Docker operation once with timeout
func (op *DockerOperation) ExecuteOnce(ctx context.Context) error {
	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, op.Timeout)
	defer cancel()

	// Execute the operation
	if op.ExecuteFunc == nil {
		op.lastError = fmt.Errorf("no execute function configured for %s operation", op.Type)
		return op.lastError
	}

	err := op.ExecuteFunc(timeoutCtx)
	op.lastError = err
	return err
}

// Execute runs the Docker operation with full retry logic and progress reporting
func (op *DockerOperation) Execute(ctx context.Context) error {
	// Start progress tracking
	token := op.Progress.StartStage(fmt.Sprintf("%s_%s", op.Type, op.Name))

	// Pre-operation steps
	if err := op.runPreOperation(ctx, token); err != nil {
		op.Progress.CompleteStage(token, false, err.Error())
		return err
	}

	// Main operation with retry logic
	var lastError error
	for op.attempt = 1; op.attempt <= op.RetryAttempts; op.attempt++ {
		op.Progress.UpdateProgress(token,
			fmt.Sprintf("Attempt %d/%d: %s", op.attempt, op.RetryAttempts, op.Name),
			30+(60*op.attempt/op.RetryAttempts))

		if err := op.ExecuteOnce(ctx); err == nil {
			op.Progress.CompleteStage(token, true, "Operation completed successfully")
			return nil
		} else {
			lastError = err
			op.lastError = err

			if op.attempt < op.RetryAttempts && op.shouldRetry(err) {
				// Exponential backoff
				waitTime := time.Duration(op.attempt) * time.Second
				op.Progress.UpdateProgress(token,
					fmt.Sprintf("Attempt %d failed, retrying in %v", op.attempt, waitTime),
					30+(50*op.attempt/op.RetryAttempts))
				time.Sleep(waitTime)
			}
		}
	}

	op.Progress.CompleteStage(token, false, fmt.Sprintf("Operation failed after %d attempts: %v", op.RetryAttempts, lastError))
	return fmt.Errorf("operation failed after %d attempts: %w", op.RetryAttempts, lastError)
}

// runPreOperation executes pre-operation steps (validation, analysis, preparation)
func (op *DockerOperation) runPreOperation(ctx context.Context, token mcptypes.ProgressToken) error {
	if op.ValidateFunc != nil {
		op.Progress.UpdateProgress(token, "Validating operation", 10)
		if err := op.ValidateFunc(); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	if op.AnalyzeFunc != nil {
		op.Progress.UpdateProgress(token, "Analyzing operation", 20)
		if err := op.AnalyzeFunc(); err != nil {
			return fmt.Errorf("analysis failed: %w", err)
		}
	}

	if op.PrepareFunc != nil {
		op.Progress.UpdateProgress(token, "Preparing operation", 25)
		if err := op.PrepareFunc(); err != nil {
			return fmt.Errorf("preparation failed: %w", err)
		}
	}

	return nil
}

// shouldRetry determines if the operation should be retried based on the error and operation type
func (op *DockerOperation) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Common retryable errors for all operation types
	commonRetryablePatterns := []string{
		"connection refused",
		"timeout",
		"network",
		"dial",
		"rate limit",
		"too many requests",
		"temporary failure",
		"service unavailable",
	}

	for _, pattern := range commonRetryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Operation-specific retry logic
	switch op.Type {
	case OperationPull:
		return op.shouldRetryPull(errStr)
	case OperationPush:
		return op.shouldRetryPush(errStr)
	case OperationTag:
		return op.shouldRetryTag(errStr)
	default:
		return false
	}
}

// shouldRetryPull determines retry logic specific to pull operations
func (op *DockerOperation) shouldRetryPull(errStr string) bool {
	// Authentication issues might be fixable
	authPatterns := []string{"unauthorized", "authentication", "login"}
	for _, pattern := range authPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Manifest not found is usually permanent
	if strings.Contains(errStr, "manifest unknown") || strings.Contains(errStr, "not found") {
		return false
	}

	return false
}

// shouldRetryPush determines retry logic specific to push operations
func (op *DockerOperation) shouldRetryPush(errStr string) bool {
	// Authentication issues might be fixable
	authPatterns := []string{"unauthorized", "authentication", "login"}
	for _, pattern := range authPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// shouldRetryTag determines retry logic specific to tag operations
func (op *DockerOperation) shouldRetryTag(errStr string) bool {
	// Docker daemon connectivity issues are retryable
	if strings.Contains(errStr, "daemon") {
		return true
	}

	// Source image not found is usually not retryable
	if strings.Contains(errStr, "no such image") || strings.Contains(errStr, "not found") {
		return false
	}

	// Permission issues might be fixable
	permissionPatterns := []string{"permission denied", "unauthorized"}
	for _, pattern := range permissionPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Tag format issues are usually not retryable
	formatPatterns := []string{"invalid tag", "invalid reference"}
	for _, pattern := range formatPatterns {
		if strings.Contains(errStr, pattern) {
			return false
		}
	}

	return false
}

// GetFailureAnalysis analyzes failures for categorization (implements FixableOperation)
func (op *DockerOperation) GetFailureAnalysis(ctx context.Context, err error) (*mcp.RichError, error) {
	if op.AnalyzeFunc != nil {
		if analyzerErr := op.AnalyzeFunc(); analyzerErr != nil {
			return nil, analyzerErr
		}
	}

	// Create operation-specific error codes
	var errorCode string
	switch op.Type {
	case OperationPull:
		errorCode = "PULL_FAILED"
	case OperationPush:
		errorCode = "PUSH_FAILED"
	case OperationTag:
		errorCode = "TAG_FAILED"
	default:
		errorCode = "DOCKER_OPERATION_FAILED"
	}

	return &mcp.RichError{
		Message: fmt.Sprintf("%s operation failed: %v", op.Type, err),
		Code:    errorCode,
	}, nil
}

// PrepareForRetry prepares the environment for retry (implements FixableOperation)
func (op *DockerOperation) PrepareForRetry(ctx context.Context, fixAttempt *mcp.FixAttempt) error {
	if op.PrepareFunc != nil {
		return op.PrepareFunc()
	}
	return nil
}

// CanRetry determines if the operation can be retried (implements FixableOperation)
func (op *DockerOperation) CanRetry() bool {
	return op.shouldRetry(op.lastError)
}

// GetLastError returns the last error encountered (implements FixableOperation)
func (op *DockerOperation) GetLastError() error {
	return op.lastError
}

// GetOperationType returns the operation type for inspection
func (op *DockerOperation) GetOperationType() OperationType {
	return op.Type
}

// GetAttemptCount returns the current attempt number
func (op *DockerOperation) GetAttemptCount() int {
	return op.attempt
}
