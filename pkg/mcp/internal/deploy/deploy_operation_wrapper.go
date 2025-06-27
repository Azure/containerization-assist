package deploy

import (
	"context"
	"fmt"
	"strings"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// OperationWrapper wraps deployment operations with fixing capabilities
type OperationWrapper struct {
	originalOperation func(ctx context.Context) error
	failureAnalyzer   func(ctx context.Context, err error) (*mcptypes.RichError, error)
	retryPreparer     func(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error
	canRetry          func() bool
	lastError         error
	logger            zerolog.Logger
}

// NewDeployOperationWrapper creates a wrapper for deployment operations
func NewDeployOperationWrapper(
	operation func(ctx context.Context) error,
	analyzer func(ctx context.Context, err error) (*mcptypes.RichError, error),
	preparer func(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error,
	logger zerolog.Logger,
) *OperationWrapper {
	return &OperationWrapper{
		originalOperation: operation,
		failureAnalyzer:   analyzer,
		retryPreparer:     preparer,
		canRetry: func() bool {
			return true // Default to always retriable
		},
		logger: logger,
	}
}

// SetCanRetryFunc sets a custom function to determine if retry is possible
func (w *OperationWrapper) SetCanRetryFunc(f func() bool) {
	w.canRetry = f
}

// ExecuteOnce implements mcptypes.FixableOperation
func (w *OperationWrapper) ExecuteOnce(ctx context.Context) error {
	err := w.originalOperation(ctx)
	w.lastError = err
	return err
}

// Execute implements mcptypes.FixableOperation (delegates to ExecuteOnce)
func (w *OperationWrapper) Execute(ctx context.Context) error {
	return w.ExecuteOnce(ctx)
}

// GetFailureAnalysis implements mcptypes.FixableOperation
func (w *OperationWrapper) GetFailureAnalysis(ctx context.Context, err error) (*mcptypes.RichError, error) {
	if w.failureAnalyzer != nil {
		return w.failureAnalyzer(ctx, err)
	}

	// Default deployment failure analysis
	return analyzeDeploymentError(err)
}

// PrepareForRetry implements mcptypes.FixableOperation
func (w *OperationWrapper) PrepareForRetry(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error {
	if w.retryPreparer != nil {
		return w.retryPreparer(ctx, fixAttempt)
	}

	w.logger.Debug().Msg("No retry preparation needed for deployment")
	return nil
}

// CanRetry implements mcptypes.FixableOperation
func (w *OperationWrapper) CanRetry() bool {
	return w.canRetry()
}

// GetLastError implements mcptypes.FixableOperation
func (w *OperationWrapper) GetLastError() error {
	return w.lastError
}

// analyzeDeploymentError provides rich error analysis for deployment failures
func analyzeDeploymentError(err error) (*mcptypes.RichError, error) {
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

	return &mcptypes.RichError{
		Code:     code,
		Type:     errorType,
		Severity: severity,
		Message:  fmt.Sprintf("Deployment failed: %s (%s)", err.Error(), contextInfo),
	}, nil
}
