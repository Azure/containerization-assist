package build

import (
	"context"
	"fmt"
	"strings"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
)

// PullOperationWrapper wraps pull operations for retry and fixing
type PullOperationWrapper struct {
	operation func(context.Context) error
	analyzer  func() error
	preparer  func() error
	lastError error
}

// NewPullOperationWrapper creates a new wrapper for pull operations
func NewPullOperationWrapper(operation func(context.Context) error, analyzer func() error, preparer func() error) mcptypes.FixableOperation {
	return &PullOperationWrapper{
		operation: operation,
		analyzer:  analyzer,
		preparer:  preparer,
	}
}

// ExecuteOnce runs the pull operation once
func (w *PullOperationWrapper) ExecuteOnce(ctx context.Context) error {
	err := w.operation(ctx)
	w.lastError = err
	return err
}

// Execute runs the pull operation
func (w *PullOperationWrapper) Execute(ctx context.Context) error {
	return w.ExecuteOnce(ctx)
}

// GetFailureAnalysis analyzes pull failures for categorization
func (w *PullOperationWrapper) GetFailureAnalysis(ctx context.Context, err error) (*mcptypes.RichError, error) {
	if w.analyzer != nil {
		if analyzerErr := w.analyzer(); analyzerErr != nil {
			return nil, analyzerErr
		}
	}

	// Create a simple RichError for now - this would be enhanced with proper analysis
	return &mcptypes.RichError{
		Message: fmt.Sprintf("Pull operation failed: %v", err),
		Code:    "PULL_FAILED",
	}, nil
}

// PrepareForRetry prepares the environment for retry
func (w *PullOperationWrapper) PrepareForRetry(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error {
	if w.preparer != nil {
		return w.preparer()
	}
	return nil
}

// CanRetry determines if the operation can be retried based on error type
func (w *PullOperationWrapper) CanRetry() bool {
	if w.lastError == nil {
		return false
	}

	errStr := strings.ToLower(w.lastError.Error())
	// Network connectivity issues are retryable
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "dial") {
		return true
	}
	// Authentication issues might be fixable
	if strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "authentication") ||
		strings.Contains(errStr, "login") {
		return true
	}
	// Rate limiting is retryable
	if strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "too many requests") {
		return true
	}
	// Manifest not found might be retryable if tag exists
	if strings.Contains(errStr, "manifest unknown") ||
		strings.Contains(errStr, "not found") {
		return false // Usually permanent
	}
	return false
}

// GetLastError returns the last error encountered
func (w *PullOperationWrapper) GetLastError() error {
	return w.lastError
}
