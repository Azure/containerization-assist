package build

import (
	"context"
	"fmt"
	"strings"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
)

// TagOperationWrapper wraps tag operations for retry and fixing
type TagOperationWrapper struct {
	operation func(context.Context) error
	analyzer  func() error
	preparer  func() error
	lastError error
}

// NewTagOperationWrapper creates a new wrapper for tag operations
func NewTagOperationWrapper(operation func(context.Context) error, analyzer func() error, preparer func() error) mcptypes.FixableOperation {
	return &TagOperationWrapper{
		operation: operation,
		analyzer:  analyzer,
		preparer:  preparer,
	}
}

// ExecuteOnce runs the tag operation once
func (w *TagOperationWrapper) ExecuteOnce(ctx context.Context) error {
	err := w.operation(ctx)
	w.lastError = err
	return err
}

// Execute runs the tag operation
func (w *TagOperationWrapper) Execute(ctx context.Context) error {
	return w.ExecuteOnce(ctx)
}

// GetFailureAnalysis analyzes tag failures for categorization
func (w *TagOperationWrapper) GetFailureAnalysis(ctx context.Context, err error) (*mcptypes.RichError, error) {
	if w.analyzer != nil {
		if analyzerErr := w.analyzer(); analyzerErr != nil {
			return nil, analyzerErr
		}
	}
	// Create a simple RichError for now - this would be enhanced with proper analysis
	return &mcptypes.RichError{
		Message: fmt.Sprintf("Tag operation failed: %v", err),
		Code:    "TAG_FAILED",
	}, nil
}

// PrepareForRetry prepares the environment for retry
func (w *TagOperationWrapper) PrepareForRetry(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error {
	if w.preparer != nil {
		return w.preparer()
	}
	return nil
}

// CanRetry determines if the operation can be retried based on error type
func (w *TagOperationWrapper) CanRetry() bool {
	if w.lastError == nil {
		return false
	}
	errStr := strings.ToLower(w.lastError.Error())
	// Docker daemon connectivity issues are retryable
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "daemon") {
		return true
	}
	// Source image not found is usually not retryable
	if strings.Contains(errStr, "no such image") ||
		strings.Contains(errStr, "not found") {
		return false
	}
	// Permission issues might be fixable
	if strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "unauthorized") {
		return true
	}
	// Tag format issues are usually not retryable
	if strings.Contains(errStr, "invalid tag") ||
		strings.Contains(errStr, "invalid reference") {
		return false
	}
	return false
}

// GetLastError returns the last error encountered
func (w *TagOperationWrapper) GetLastError() error {
	return w.lastError
}
