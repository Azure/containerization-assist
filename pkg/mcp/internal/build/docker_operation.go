package build

import (
	"context"
	"fmt"
	"strings"
	"time"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
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
		return fmt.Errorf("execute function not defined")
	}
	return op.ExecuteFunc(timeoutCtx)
}

// Execute runs the Docker operation with retry logic
func (op *DockerOperation) Execute(ctx context.Context) error {
	var lastErr error
	for attempt := 1; attempt <= op.RetryAttempts; attempt++ {
		op.attempt = attempt
		err := op.ExecuteOnce(ctx)
		if err == nil {
			return nil
		}

		op.lastError = err

		// Check if we can retry
		if !op.CanRetry(err) {
			return fmt.Errorf("operation failed and cannot be retried: %w", err)
		}

		lastErr = err

		// Don't sleep after the last attempt
		if attempt < op.RetryAttempts {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", op.RetryAttempts, lastErr)
}

// CanRetry determines if the operation can be retried after failure
func (op *DockerOperation) CanRetry(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(fmt.Sprintf("%v", err))

	// Non-retryable errors
	nonRetryableErrors := []string{
		"permission denied",
		"authentication required",
		"invalid",
		"syntax error",
		"malformed",
		"not found",
	}

	for _, nonRetryable := range nonRetryableErrors {
		if contains(errStr, nonRetryable) {
			return false
		}
	}

	// Retryable errors
	retryableErrors := []string{
		"timeout",
		"network",
		"connection refused",
		"temporary failure",
		"rate limit",
		"too many requests",
		"service unavailable",
	}

	for _, retryable := range retryableErrors {
		if contains(errStr, retryable) {
			return true
		}
	}

	// Default to retryable for unknown errors, but limit by attempt count
	return op.attempt < op.RetryAttempts
}

// GetFailureAnalysis analyzes the failure for potential fixes
func (op *DockerOperation) GetFailureAnalysis(ctx context.Context, err error) (*mcptypes.FailureAnalysis, error) {
	if err == nil {
		return nil, fmt.Errorf("no error to analyze")
	}

	errStr := strings.ToLower(fmt.Sprintf("%v", err))
	failureType := "unknown"
	isCritical := false
	isRetryable := op.CanRetry(err)
	rootCauses := []string{}
	suggestedFixes := []string{}

	// Analyze different error types
	if contains(errStr, "timeout") {
		failureType = "timeout"
		rootCauses = []string{"Network timeout", "Operation took too long"}
		suggestedFixes = []string{"Increase timeout duration", "Check network connectivity", "Retry with backoff"}
	} else if contains(errStr, "permission denied") {
		failureType = "permission_error"
		isCritical = true
		rootCauses = []string{"Insufficient permissions"}
		suggestedFixes = []string{"Check Docker daemon permissions", "Run with appropriate privileges", "Check file permissions"}
	} else if contains(errStr, "not found") {
		failureType = "resource_not_found"
		rootCauses = []string{"Missing resource or dependency"}
		suggestedFixes = []string{"Verify resource exists", "Check spelling and path", "Ensure prerequisites are met"}
	} else if contains(errStr, "network") || contains(errStr, "connection") {
		failureType = "network_error"
		rootCauses = []string{"Network connectivity issues"}
		suggestedFixes = []string{"Check network connection", "Verify proxy settings", "Retry after network stabilizes"}
	} else {
		failureType = "unknown"
		rootCauses = []string{"Unrecognized error pattern"}
		suggestedFixes = []string{"Check logs for more details", "Retry operation", "Contact support if issue persists"}
	}

	return &mcptypes.FailureAnalysis{
		FailureType:    failureType,
		IsCritical:     isCritical,
		IsRetryable:    isRetryable,
		RootCauses:     rootCauses,
		SuggestedFixes: suggestedFixes,
		ErrorContext:   fmt.Sprintf("Docker %s operation failed after %d attempts", op.Type, op.attempt),
	}, nil
}

// PrepareForRetry prepares the operation for retry (e.g., cleanup, state reset)
func (op *DockerOperation) PrepareForRetry(ctx context.Context, fixAttempt interface{}) error {
	// Reset operation state
	op.lastError = nil

	// Call prepare function if available
	if op.PrepareFunc != nil {
		return op.PrepareFunc()
	}

	return nil
}

// Helper function to check if string contains substring (case insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
