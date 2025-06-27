package deploy

import (
	"context"
	"fmt"
	"strings"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
)

// HealthCheckOperationWrapper wraps health check operations for retry and fixing
type HealthCheckOperationWrapper struct {
	operation func(context.Context) error
	analyzer  func() error
	preparer  func() error
	lastError error
}

// NewHealthCheckOperationWrapper creates a new wrapper for health check operations
func NewHealthCheckOperationWrapper(operation func(context.Context) error, analyzer func() error, preparer func() error) mcptypes.FixableOperation {
	return &HealthCheckOperationWrapper{
		operation: operation,
		analyzer:  analyzer,
		preparer:  preparer,
	}
}

// ExecuteOnce runs the health check operation once
func (w *HealthCheckOperationWrapper) ExecuteOnce(ctx context.Context) error {
	err := w.operation(ctx)
	w.lastError = err
	return err
}

// Execute runs the health check operation
func (w *HealthCheckOperationWrapper) Execute(ctx context.Context) error {
	return w.ExecuteOnce(ctx)
}

// GetFailureAnalysis analyzes health check failures for categorization
func (w *HealthCheckOperationWrapper) GetFailureAnalysis(ctx context.Context, err error) (*mcptypes.RichError, error) {
	if w.analyzer != nil {
		if analyzerErr := w.analyzer(); analyzerErr != nil {
			return nil, analyzerErr
		}
	}

	// Create a simple RichError for now - this would be enhanced with proper analysis
	return &mcptypes.RichError{
		Message: fmt.Sprintf("Health check operation failed: %v", err),
		Code:    "HEALTH_CHECK_FAILED",
	}, nil
}

// PrepareForRetry prepares the environment for retry
func (w *HealthCheckOperationWrapper) PrepareForRetry(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error {
	if w.preparer != nil {
		return w.preparer()
	}
	return nil
}

// CanRetry determines if the operation can be retried based on error type
func (w *HealthCheckOperationWrapper) CanRetry() bool {
	if w.lastError == nil {
		return false
	}

	errStr := strings.ToLower(w.lastError.Error())
	// Kubernetes API connectivity issues are retryable
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "dial") ||
		strings.Contains(errStr, "network") {
		return true
	}
	// Authentication/authorization issues might be fixable
	if strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "authentication") ||
		strings.Contains(errStr, "forbidden") {
		return true
	}
	// Namespace not found is usually not retryable immediately
	if strings.Contains(errStr, "namespace") && strings.Contains(errStr, "not found") {
		return false
	}
	// Resource not found might be retryable if deployment is in progress
	if strings.Contains(errStr, "not found") {
		return true
	}
	// API server errors are retryable
	if strings.Contains(errStr, "internal server error") ||
		strings.Contains(errStr, "service unavailable") {
		return true
	}
	return false
}

// GetLastError returns the last error encountered
func (w *HealthCheckOperationWrapper) GetLastError() error {
	return w.lastError
}
