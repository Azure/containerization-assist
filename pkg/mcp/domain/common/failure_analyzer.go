package common

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/rs/zerolog"
)

type DefaultFailureAnalyzer struct {
	toolName  string
	operation string
	logger    zerolog.Logger
	failures  []AnalysisFailure
}

type AnalysisFailure struct {
	Timestamp   time.Time              `json:"timestamp"`
	ErrorType   string                 `json:"error_type"`
	Message     string                 `json:"message"`
	Context     map[string]interface{} `json:"context"`
	Recoverable bool                   `json:"recoverable"`
}

func NewDefaultFailureAnalyzer(toolName, operation string, logger zerolog.Logger) *DefaultFailureAnalyzer {
	return &DefaultFailureAnalyzer{
		toolName:  toolName,
		operation: operation,
		logger:    logger,
		failures:  make([]AnalysisFailure, 0),
	}
}

func (a *DefaultFailureAnalyzer) RecordFailure(errorType, message string, context map[string]interface{}) error {
	if errorType == "" {
		return errors.NewError().Messagef("error type cannot be empty").WithLocation().Build()
	}
	if message == "" {
		return errors.NewError().Messagef("message cannot be empty").WithLocation().Build()
	}

	failure := AnalysisFailure{
		Timestamp:   time.Now(),
		ErrorType:   errorType,
		Message:     message,
		Context:     context,
		Recoverable: a.IsRecoverable(errorType),
	}

	a.failures = append(a.failures, failure)

	a.logger.Error().
		Str("tool", a.toolName).
		Str("operation", a.operation).
		Str("error_type", errorType).
		Bool("recoverable", failure.Recoverable).
		Msg(message)

	return nil
}

func (a *DefaultFailureAnalyzer) GetFailures() []AnalysisFailure {
	return a.failures
}

func (a *DefaultFailureAnalyzer) GetFailureCount() int {
	return len(a.failures)
}

func (a *DefaultFailureAnalyzer) ClearFailures() {
	a.failures = make([]AnalysisFailure, 0)
}

func (a *DefaultFailureAnalyzer) IsRecoverable(errorType string) bool {
	recoverableErrors := map[string]bool{
		"network_timeout":   true,
		"temporary_failure": true,
		"rate_limit":        true,
		"connection_reset":  true,
		"disk_full":         false,
		"invalid_config":    false,
		"permission_denied": false,
		"authentication":    false,
		"syntax_error":      false,
	}

	recoverable, exists := recoverableErrors[errorType]
	if !exists {
		return false
	}
	return recoverable
}

// AnalyzeFailure provides failure analysis - satisfies the FailureAnalyzer interface
func (a *DefaultFailureAnalyzer) AnalyzeFailure(ctx context.Context, operation, sessionID string, params map[string]interface{}) error {
	// Use the existing RecordFailure method to record the analysis
	errorType := "unknown"
	message := fmt.Sprintf("Failure in operation %s for session %s", operation, sessionID)

	// Extract error type from params if available
	if errType, ok := params["error_type"].(string); ok && errType != "" {
		errorType = errType
	}

	// Extract message from params if available
	if msg, ok := params["message"].(string); ok && msg != "" {
		message = msg
	}

	return a.RecordFailure(errorType, message, params)
}
