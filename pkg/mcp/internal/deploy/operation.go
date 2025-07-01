package deploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
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
	AnalyzeFunc  func(ctx context.Context, err error) (error, error)
	PrepareFunc  func(ctx context.Context, fixAttempt interface{}) error
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
func (op *Operation) GetFailureAnalysis(ctx context.Context, err error) (*mcptypes.FailureAnalysis, error) {
	if err == nil {
		return nil, nil
	}

	// Analyze error and determine failure characteristics
	errorType, retryable := op.analyzeError(err)

	analysis := &mcptypes.FailureAnalysis{
		FailureType:    errorType,
		IsCritical:     strings.Contains(strings.ToLower(err.Error()), "critical") || strings.Contains(strings.ToLower(err.Error()), "fatal"),
		IsRetryable:    retryable,
		RootCauses:     []string{err.Error()},
		SuggestedFixes: op.getSuggestedFixes(errorType),
		ErrorContext:   fmt.Sprintf("operation=%s, type=%s", op.Name, op.Type),
	}

	return analysis, nil
}

// PrepareForRetry prepares the environment for retry
func (op *Operation) PrepareForRetry(ctx context.Context, fixAttempt interface{}) error {
	if op.PrepareFunc != nil {
		return op.PrepareFunc(ctx, fixAttempt)
	}

	op.Logger.Debug().Msg("No retry preparation needed for operation")
	return nil
}

// CanRetry determines if the operation can be retried based on error type
func (op *Operation) CanRetry(err error) bool {
	if err == nil {
		err = op.lastError
	}
	if err == nil {
		return false
	}

	if op.CanRetryFunc != nil {
		return op.CanRetryFunc(err)
	}

	return defaultCanRetry(err)
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

// analyzeError determines error type and retryability
func (op *Operation) analyzeError(err error) (string, bool) {
	if err == nil {
		return "unknown", false
	}

	errStr := err.Error()
	lowerErr := strings.ToLower(errStr)

	// Categorize the error type and determine retryability
	switch {
	case strings.Contains(lowerErr, "imagepullbackoff") || strings.Contains(lowerErr, "errimage"):
		return "image_error", true
	case strings.Contains(lowerErr, "crashloopbackoff"):
		return "runtime_error", true
	case strings.Contains(lowerErr, "insufficient") || strings.Contains(lowerErr, "resource"):
		return "resource_error", true
	case strings.Contains(lowerErr, "forbidden") || strings.Contains(lowerErr, "unauthorized"):
		return "permission_error", false
	case strings.Contains(lowerErr, "timeout"):
		return "timeout_error", true
	case strings.Contains(lowerErr, "manifest") || strings.Contains(lowerErr, "yaml"):
		return "manifest_error", true
	case strings.Contains(lowerErr, "network") || strings.Contains(lowerErr, "connection"):
		return "network_error", true
	default:
		return "deployment_error", true
	}
}

// getSuggestedFixes returns suggested fixes based on error type
func (op *Operation) getSuggestedFixes(errorType string) []string {
	switch errorType {
	case "image_error":
		return []string{
			"Verify the image exists and is accessible",
			"Check registry credentials and permissions",
			"Ensure correct image tag is specified",
		}
	case "runtime_error":
		return []string{
			"Check application logs for startup errors",
			"Verify resource requests and limits",
			"Review application configuration",
		}
	case "resource_error":
		return []string{
			"Check cluster resource availability",
			"Review resource requests and limits",
			"Consider scaling cluster or reducing requests",
		}
	case "permission_error":
		return []string{
			"Verify RBAC permissions",
			"Check service account configuration",
			"Review namespace access rights",
		}
	case "timeout_error":
		return []string{
			"Increase timeout values",
			"Check network connectivity",
			"Review application startup time",
		}
	case "manifest_error":
		return []string{
			"Validate YAML syntax",
			"Check Kubernetes API compatibility",
			"Review resource specifications",
		}
	case "network_error":
		return []string{
			"Check network connectivity",
			"Verify DNS resolution",
			"Review firewall rules",
		}
	default:
		return []string{
			"Check application logs",
			"Review deployment configuration",
			"Verify cluster health",
		}
	}
}

// analyzeDeploymentError provides rich error analysis for deployment failures
func analyzeDeploymentError(err error) (error, error) {
	if err == nil {
		return nil, nil
	}

	errStr := err.Error()
	lowerErr := strings.ToLower(errStr)

	// Categorize the error
	var errorType string
	var retryable bool
	_ = errorType // Use error type for analysis

	switch {
	case strings.Contains(lowerErr, "imagepullbackoff") || strings.Contains(lowerErr, "errimage"):
		errorType = "image_error"
		retryable = true

	case strings.Contains(lowerErr, "crashloopbackoff"):
		errorType = "runtime_error"
		retryable = true

	case strings.Contains(lowerErr, "insufficient") || strings.Contains(lowerErr, "resource"):
		errorType = "resource_error"
		retryable = true

	case strings.Contains(lowerErr, "forbidden") || strings.Contains(lowerErr, "unauthorized"):
		errorType = "permission_error"
		retryable = false

	case strings.Contains(lowerErr, "timeout"):
		errorType = "timeout_error"
		retryable = true

	case strings.Contains(lowerErr, "manifest") || strings.Contains(lowerErr, "yaml"):
		errorType = "manifest_error"
		retryable = true

	case strings.Contains(lowerErr, "network") || strings.Contains(lowerErr, "connection"):
		errorType = "network_error"
		retryable = true

	default:
		errorType = "deployment_error"
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

	return fmt.Errorf("deployment failed: %s (%s)", err.Error(), contextInfo), nil
}

// analyzeHealthCheckError provides rich error analysis for health check failures
func analyzeHealthCheckError(err error) (error, error) {
	if err == nil {
		return nil, nil
	}

	return fmt.Errorf("health check operation failed: %v", err), nil
}
