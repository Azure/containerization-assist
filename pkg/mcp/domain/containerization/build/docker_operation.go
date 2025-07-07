package build

import (
	"context"
	"fmt"
	"strings"
	"time"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/application/core"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
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
func NewDockerOperation(config DockerOperationConfig, progress mcptypes.ProgressReporter) mcptypes.ConsolidatedFixableOperation {
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
		return errors.NewError().Messagef("execute function not defined").Build()
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
			return errors.NewError().Message("operation failed and cannot be retried").Cause(err).Build()
		}

		lastErr = err

		// Don't sleep after the last attempt
		if attempt < op.RetryAttempts {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	return errors.NewError().Message(fmt.Sprintf("operation failed after %d attempts", op.RetryAttempts)).Cause(lastErr).WithLocation(

	// CanRetry determines if the operation can be retried after failure
	).Build()
}

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
		if strings.Contains(strings.ToLower(errStr), strings.ToLower(nonRetryable)) {
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
		if strings.Contains(strings.ToLower(errStr), strings.ToLower(retryable)) {
			return true
		}
	}

	// Default to retryable for unknown errors, but limit by attempt count
	return op.attempt < op.RetryAttempts
}

// GetFailureAnalysis analyzes the failure for potential fixes
func (op *DockerOperation) GetFailureAnalysis(ctx context.Context, err error) (*mcptypes.ConsolidatedFailureAnalysis, error) {
	if err == nil {
		return nil, errors.NewError().Messagef("no error to analyze").Build()
	}

	errStr := strings.ToLower(fmt.Sprintf("%v", err))
	failureType := "unknown"
	isCritical := false
	isRetryable := op.CanRetry(err)
	rootCauses := []string{}
	suggestedFixes := []string{}

	// Analyze different error types
	if strings.Contains(strings.ToLower(errStr), "timeout") {
		failureType = "timeout"
		rootCauses = []string{"Network timeout", "Operation took too long"}
		suggestedFixes = []string{"Increase timeout duration", "Check network connectivity", "Retry with backoff"}
	} else if strings.Contains(strings.ToLower(errStr), "permission denied") {
		failureType = "permission_error"
		isCritical = true
		rootCauses = []string{"Insufficient permissions"}
		suggestedFixes = []string{"Check Docker daemon permissions", "Run with appropriate privileges", "Check file permissions"}
	} else if strings.Contains(strings.ToLower(errStr), "not found") {
		failureType = "resource_not_found"
		rootCauses = []string{"Missing resource or dependency"}
		suggestedFixes = []string{"Verify resource exists", "Check spelling and path", "Ensure prerequisites are met"}
	} else if strings.Contains(strings.ToLower(errStr), "network") || strings.Contains(strings.ToLower(errStr), "connection") {
		failureType = "network_error"
		rootCauses = []string{"Network connectivity issues"}
		suggestedFixes = []string{"Check network connection", "Verify proxy settings", "Retry after network stabilizes"}
	} else {
		failureType = "unknown"
		rootCauses = []string{"Unrecognized error pattern"}
		suggestedFixes = []string{"Check logs for more details", "Retry operation", "Contact support if issue persists"}
	}

	return &mcptypes.ConsolidatedFailureAnalysis{
		FailureType:              failureType,
		IsCritical:               isCritical,
		IsRetryable:              isRetryable,
		RootCauses:               rootCauses,
		SuggestedFixes:           suggestedFixes,
		ConsolidatedErrorContext: fmt.Sprintf("Docker %s operation failed after %d attempts", op.Type, op.attempt),
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
