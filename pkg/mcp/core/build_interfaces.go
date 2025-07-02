package core

import (
	"context"
	"fmt"
	"time"
)

// Build and failure handling interfaces
// These interfaces are primarily used by build and deployment operations

// FixableOperation represents an operation that can be fixed when it fails
type FixableOperation interface {
	// Execute performs the operation
	Execute(ctx context.Context) error
	// ExecuteOnce runs the operation once
	ExecuteOnce(ctx context.Context) error
	// CanRetry determines if the operation can be retried after failure
	CanRetry(err error) bool
	// GetFailureAnalysis analyzes the failure for potential fixes
	GetFailureAnalysis(ctx context.Context, err error) (*FailureAnalysis, error)
	// PrepareForRetry prepares the operation for retry (e.g., cleanup, state reset)
	PrepareForRetry(ctx context.Context, fixAttempt interface{}) error
}

// FailureAnalysis represents analysis of an operation failure
type FailureAnalysis struct {
	FailureType    string   `json:"failure_type"`
	IsCritical     bool     `json:"is_critical"`
	IsRetryable    bool     `json:"is_retryable"`
	RootCauses     []string `json:"root_causes"`
	SuggestedFixes []string `json:"suggested_fixes"`
	ErrorContext   string   `json:"error_context"`
}

// Error implements the error interface for FailureAnalysis
func (fa *FailureAnalysis) Error() string {
	if fa == nil {
		return "failure analysis: <nil>"
	}
	return fmt.Sprintf("failure analysis: %s (%s)", fa.FailureType, fa.ErrorContext)
}

// ErrorContext provides contextual information about errors
type ErrorContext struct {
	SessionID     string            `json:"session_id"`
	OperationType string            `json:"operation_type"`
	Phase         string            `json:"phase"`
	ErrorCode     string            `json:"error_code"`
	Metadata      map[string]string `json:"metadata"`
	Timestamp     time.Time         `json:"timestamp"`
}

// LocalProgressStage represents a local progress stage
type LocalProgressStage struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Progress    int     `json:"progress"`
	Status      string  `json:"status"`
	Weight      float64 `json:"weight"`
}
