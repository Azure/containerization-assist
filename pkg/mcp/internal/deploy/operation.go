package deploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp"
	mcptypes "github.com/Azure/container-kit/pkg/mcp"
	"github.com/rs/zerolog"
)

// OperationType represents the type of deployment operation
type OperationType string

const (
	OperationDeploy      OperationType = "deploy"
	OperationHealthCheck OperationType = "health_check"
)

// Operation provides a unified, configurable wrapper for all deployment operations
// (deploy, health check) with built-in retry logic, error analysis, and progress reporting.
// This consolidates the functionality of DeployOperationWrapper and HealthCheckOperationWrapper.
type Operation struct {
	// Operation identification
	Type OperationType
	Name string

	// Configuration
	RetryAttempts int
	Timeout       time.Duration

	// Dependencies
	Progress mcptypes.ProgressReporter
	Logger   zerolog.Logger

	// Operation-specific functions (configurable)
	ExecuteFunc  func(ctx context.Context) error
	AnalyzeFunc  func(ctx context.Context, err error) (*mcp.RichError, error)
	PrepareFunc  func(ctx context.Context, fixAttempt *mcp.FixAttempt) error
	CanRetryFunc func(error) bool

	// State
	lastError error
	attempt   int
}

// OperationConfig provides configuration for creating an Operation
type OperationConfig struct {
	Type          OperationType
	Name          string
	RetryAttempts int
	Timeout       time.Duration
	Progress      mcptypes.ProgressReporter
	Logger        zerolog.Logger
}

// NewOperation creates a new operation with the given configuration
func NewOperation(config OperationConfig) *Operation {
	if config.RetryAttempts <= 0 {
		config.RetryAttempts = 3
	}
	if config.Timeout <= 0 {
		config.Timeout = 5 * time.Minute
	}

	return &Operation{
		Type:          config.Type,
		Name:          config.Name,
		RetryAttempts: config.RetryAttempts,
		Timeout:       config.Timeout,
		Progress:      config.Progress,
		Logger:        config.Logger,
		CanRetryFunc:  defaultCanRetry,
	}
}

// ExecuteOnce runs the operation once
func (op *Operation) ExecuteOnce(ctx context.Context) error {
	if op.ExecuteFunc == nil {
		return fmt.Errorf("no execute function configured for operation %s", op.Name)
	}

	err := op.ExecuteFunc(ctx)
	op.lastError = err
	return err
}

// Execute runs the operation
func (op *Operation) Execute(ctx context.Context) error {
	return op.ExecuteOnce(ctx)
}

// GetFailureAnalysis analyzes operation failures for categorization
func (op *Operation) GetFailureAnalysis(ctx context.Context, err error) (*mcp.RichError, error) {
	if op.AnalyzeFunc != nil {
		return op.AnalyzeFunc(ctx, err)
	}

	// Use type-specific default analysis
	switch op.Type {
	case OperationDeploy:
		return analyzeDeploymentError(err)
	case OperationHealthCheck:
		return analyzeHealthCheckError(err)
	default:
		return &mcp.RichError{
			Message: fmt.Sprintf("Operation failed: %v", err),
			Code:    "OPERATION_FAILED",
		}, nil
	}
}

// PrepareForRetry prepares the environment for retry
func (op *Operation) PrepareForRetry(ctx context.Context, fixAttempt *mcp.FixAttempt) error {
	if op.PrepareFunc != nil {
		return op.PrepareFunc(ctx, fixAttempt)
	}

	op.Logger.Debug().Msg("No retry preparation needed for operation")
	return nil
}

// CanRetry determines if the operation can be retried based on error type
func (op *Operation) CanRetry() bool {
	if op.lastError == nil {
		return false
	}

	if op.CanRetryFunc != nil {
		return op.CanRetryFunc(op.lastError)
	}

	return defaultCanRetry(op.lastError)
}

// GetLastError returns the last error encountered
func (op *Operation) GetLastError() error {
	return op.lastError
}

// defaultCanRetry provides default retry logic based on error content
func defaultCanRetry(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Network and connectivity issues are retryable
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "dial") ||
		strings.Contains(errStr, "network") {
		return true
	}

	// Authentication issues might be fixable
	if strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "authentication") ||
		strings.Contains(errStr, "forbidden") {
		return true
	}

	// API server errors are retryable
	if strings.Contains(errStr, "internal server error") ||
		strings.Contains(errStr, "service unavailable") {
		return true
	}

	// Resource not found might be retryable if resources are being created
	if strings.Contains(errStr, "not found") {
		return true
	}

	return false
}

// analyzeDeploymentError provides rich error analysis for deployment failures
func analyzeDeploymentError(err error) (*mcp.RichError, error) {
	if err == nil {
		return nil, nil
	}

	errStr := err.Error()
	lowerErr := strings.ToLower(errStr)

	// Categorize the error
	var errorType, severity, code string
	var retryable bool

	switch {
	case strings.Contains(lowerErr, "imagepullbackoff") || strings.Contains(lowerErr, "errimage"):
		errorType = "image_error"
		severity = "High"
		code = "IMAGE_PULL_ERROR"
		retryable = true

	case strings.Contains(lowerErr, "crashloopbackoff"):
		errorType = "runtime_error"
		severity = "Critical"
		code = "CRASH_LOOP_BACKOFF"
		retryable = true

	case strings.Contains(lowerErr, "insufficient") || strings.Contains(lowerErr, "resource"):
		errorType = "resource_error"
		severity = "High"
		code = "INSUFFICIENT_RESOURCES"
		retryable = true

	case strings.Contains(lowerErr, "forbidden") || strings.Contains(lowerErr, "unauthorized"):
		errorType = "permission_error"
		severity = "Critical"
		code = "PERMISSION_DENIED"
		retryable = false

	case strings.Contains(lowerErr, "timeout"):
		errorType = "timeout_error"
		severity = "Medium"
		code = "DEPLOYMENT_TIMEOUT"
		retryable = true

	case strings.Contains(lowerErr, "manifest") || strings.Contains(lowerErr, "yaml"):
		errorType = "manifest_error"
		severity = "High"
		code = "INVALID_MANIFEST"
		retryable = true

	case strings.Contains(lowerErr, "network") || strings.Contains(lowerErr, "connection"):
		errorType = "network_error"
		severity = "Medium"
		code = "NETWORK_ERROR"
		retryable = true

	default:
		errorType = "deployment_error"
		severity = "High"
		code = "DEPLOYMENT_FAILED"
		retryable = true
	}

	// Create context information in the message
	contextInfo := fmt.Sprintf("error_category=%s, is_retryable=%v", errorType, retryable)

	// Add specific context based on error type
	if errorType == "image_error" {
		contextInfo += ", suggested_fix=verify image exists and pull permissions, escalation_target=build_image"
	} else if errorType == "manifest_error" {
		contextInfo += ", suggested_fix=regenerate or validate manifests, escalation_target=generate_manifests"
	} else if errorType == "resource_error" {
		contextInfo += ", suggested_fix=reduce resource requests or scale cluster, escalation_target=generate_manifests"
	}

	return &mcp.RichError{
		Code:     code,
		Type:     errorType,
		Severity: severity,
		Message:  fmt.Sprintf("Deployment failed: %s (%s)", err.Error(), contextInfo),
	}, nil
}

// analyzeHealthCheckError provides rich error analysis for health check failures
func analyzeHealthCheckError(err error) (*mcp.RichError, error) {
	if err == nil {
		return nil, nil
	}

	return &mcp.RichError{
		Message: fmt.Sprintf("Health check operation failed: %v", err),
		Code:    "HEALTH_CHECK_FAILED",
	}, nil
}