package build

import (
	"context"
	"fmt"
	"strings"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
)

// PushOperationWrapper wraps push operations for retry and fixing
type PushOperationWrapper struct {
	operation func(context.Context) error
	analyzer  func() error
	preparer  func() error
	lastError error
}

// NewPushOperationWrapper creates a new wrapper for push operations
func NewPushOperationWrapper(operation func(context.Context) error, analyzer func() error, preparer func() error) mcptypes.FixableOperation {
	return &PushOperationWrapper{
		operation: operation,
		analyzer:  analyzer,
		preparer:  preparer,
	}
}

// ExecuteOnce runs the push operation once
func (w *PushOperationWrapper) ExecuteOnce(ctx context.Context) error {
	err := w.operation(ctx)
	w.lastError = err
	return err
}

// Execute runs the push operation
func (w *PushOperationWrapper) Execute(ctx context.Context) error {
	return w.ExecuteOnce(ctx)
}

// GetFailureAnalysis analyzes push failures for categorization
func (w *PushOperationWrapper) GetFailureAnalysis(ctx context.Context, err error) (*mcptypes.RichError, error) {
	if w.analyzer != nil {
		if analyzerErr := w.analyzer(); analyzerErr != nil {
			return nil, analyzerErr
		}
	}

	// Create a simple RichError for now - this would be enhanced with proper analysis
	return &mcptypes.RichError{
		Message: fmt.Sprintf("Push operation failed: %v", err),
		Code:    "PUSH_FAILED",
	}, nil
}

// PrepareForRetry prepares the environment for retry
func (w *PushOperationWrapper) PrepareForRetry(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error {
	if w.preparer != nil {
		return w.preparer()
	}
	return nil
}

// CanRetry determines if the operation can be retried based on error type
func (w *PushOperationWrapper) CanRetry() bool {
	if w.lastError == nil {
		return false
	}

	errStr := strings.ToLower(w.lastError.Error())
	// Registry connectivity issues are retryable
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
	return false
}

// GetLastError returns the last error encountered
func (w *PushOperationWrapper) GetLastError() error {
	return w.lastError
}
