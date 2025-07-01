package transport

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/errors/rich"
	"github.com/Azure/container-kit/pkg/mcp/internal/errors"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// StdioErrorHandler provides enhanced error handling for stdio transport
type StdioErrorHandler struct {
	logger zerolog.Logger
}

// NewStdioErrorHandler creates a new stdio error handler
func NewStdioErrorHandler(logger zerolog.Logger) *StdioErrorHandler {
	return &StdioErrorHandler{
		logger: logger.With().Str("component", "stdio_error_handler").Logger(),
	}
}

// HandleToolError converts tool errors into appropriate JSON-RPC error responses
func (h *StdioErrorHandler) HandleToolError(ctx context.Context, toolName string, err error) (interface{}, error) {
	h.logger.Error().
		Err(err).
		Str("tool", toolName).
		Msg("Handling tool error for stdio transport")

	// Check for context cancellation first
	if ctx.Err() != nil {
		return h.createCancellationResponse(ctx.Err(), toolName), nil
	}

	// Handle different error types
	switch typedErr := err.(type) {
	case *rich.RichError:
		return h.handleRichError(typedErr, toolName), nil
	case *errors.CoreError:
		return h.handleCoreError(typedErr, toolName), nil
	case *types.ToolError:
		return h.handleToolError(typedErr, toolName), nil
	case *server.InvalidParametersError:
		return nil, h.createInvalidParametersError(typedErr.Message)
	default:
		// Convert generic errors to RichError for better handling
		richErr := h.enrichGenericError(err, toolName)
		return h.handleRichError(richErr, toolName), nil
	}
}

// handleCoreError creates a comprehensive error response from CoreError
func (h *StdioErrorHandler) handleCoreError(coreErr *errors.CoreError, toolName string) interface{} {
	// Create MCP-compatible error response
	response := map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": h.formatCoreErrorMessage(coreErr),
			},
		},
		"isError": true,
		"error": map[string]interface{}{
			"code":      coreErr.Code,
			"category":  string(coreErr.Category),
			"severity":  string(coreErr.Severity),
			"message":   coreErr.Message,
			"tool":      toolName,
			"timestamp": coreErr.Timestamp,
			"retryable": coreErr.Retryable,
			"fatal":     coreErr.Fatal,
		},
	}

	// Add context information if available
	if coreErr.Operation != "" {
		if errorMap, ok := response["error"].(map[string]interface{}); ok {
			errorMap["operation"] = coreErr.Operation
			errorMap["stage"] = coreErr.Stage
			errorMap["component"] = coreErr.Component
			errorMap["module"] = coreErr.Module
		}
	}

	// Add resolution steps if available
	if coreErr.Resolution != nil && len(coreErr.Resolution.ImmediateSteps) > 0 {
		steps := make([]map[string]interface{}, len(coreErr.Resolution.ImmediateSteps))
		for i, step := range coreErr.Resolution.ImmediateSteps {
			steps[i] = map[string]interface{}{
				"step":        step.Step,
				"action":      step.Action,
				"description": step.Description,
				"command":     step.Command,
				"expected":    step.Expected,
			}
		}
		response["resolution_steps"] = steps
	}

	// Add alternatives if available
	if coreErr.Resolution != nil && len(coreErr.Resolution.Alternatives) > 0 {
		alternatives := make([]map[string]interface{}, len(coreErr.Resolution.Alternatives))
		for i, alt := range coreErr.Resolution.Alternatives {
			alternatives[i] = map[string]interface{}{
				"approach":    alt.Approach,
				"description": alt.Description,
				"effort":      alt.Effort,
				"risk":        alt.Risk,
			}
		}
		response["alternatives"] = alternatives
	}

	// Add retry information
	if coreErr.Resolution != nil && coreErr.Resolution.RetryStrategy != nil && coreErr.Resolution.RetryStrategy.Retryable {
		response["retry_strategy"] = map[string]interface{}{
			"retryable":      coreErr.Resolution.RetryStrategy.Retryable,
			"max_attempts":   coreErr.Resolution.RetryStrategy.MaxAttempts,
			"backoff_ms":     coreErr.Resolution.RetryStrategy.BackoffMs,
			"exponential_ms": coreErr.Resolution.RetryStrategy.ExponentialMs,
			"conditions":     coreErr.Resolution.RetryStrategy.Conditions,
		}
	}

	// Add diagnostic information
	if coreErr.Diagnostics != nil && coreErr.Diagnostics.RootCause != "" {
		response["diagnostics"] = map[string]interface{}{
			"root_cause":    coreErr.Diagnostics.RootCause,
			"error_pattern": coreErr.Diagnostics.ErrorPattern,
			"symptoms":      coreErr.Diagnostics.Symptoms,
		}
	}

	return response
}

// handleRichError creates a comprehensive error response from RichError
func (h *StdioErrorHandler) handleRichError(richErr *rich.RichError, toolName string) interface{} {
	// Create MCP-compatible error response with rich context
	response := map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": h.formatRichErrorMessage(richErr),
			},
		},
		"isError": true,
		"error": map[string]interface{}{
			"code":      string(richErr.Code),
			"type":      string(richErr.Type),
			"severity":  string(richErr.Severity),
			"message":   richErr.Message,
			"tool":      toolName,
			"timestamp": richErr.Timestamp,
			"context":   richErr.Context,
			"location":  richErr.Location,
		},
	}

	// Add suggestion if available
	if richErr.Suggestion != "" {
		response["suggestion"] = richErr.Suggestion
	}

	// Add cause chain if available
	if richErr.Cause != nil {
		response["cause"] = richErr.Cause.Error()
	}

	// Add stack trace if available
	if len(richErr.Stack) > 0 {
		response["stack"] = richErr.Stack
	}

	return response
}

// handleToolError creates an error response from ToolError
func (h *StdioErrorHandler) handleToolError(toolErr *types.ToolError, toolName string) interface{} {
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": h.formatToolErrorMessage(toolErr),
			},
		},
		"isError": true,
		"error": map[string]interface{}{
			"type":        toolErr.Type,
			"message":     toolErr.Message,
			"retryable":   toolErr.Retryable,
			"retry_count": toolErr.RetryCount,
			"max_retries": toolErr.MaxRetries,
			"suggestions": toolErr.Suggestions,
			"tool":        toolName,
			"timestamp":   toolErr.Timestamp,
			"context":     toolErr.Context,
		},
	}
}

// handleGenericError creates a basic error response for generic errors
func (h *StdioErrorHandler) handleGenericError(err error, toolName string) interface{} {
	// Try to categorize the error
	errorType := h.categorizeError(err)
	isRetryable := h.isRetryableError(err)

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Tool '%s' failed: %v", toolName, err),
			},
		},
		"isError": true,
		"error": map[string]interface{}{
			"type":      errorType,
			"message":   err.Error(),
			"retryable": isRetryable,
			"tool":      toolName,
			"timestamp": time.Now(),
		},
	}
}

// createCancellationResponse creates a response for cancelled operations
func (h *StdioErrorHandler) createCancellationResponse(ctxErr error, toolName string) interface{} {
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Tool '%s' was cancelled: %v", toolName, ctxErr),
			},
		},
		"isError":   true,
		"cancelled": true,
		"error": map[string]interface{}{
			"type":      "cancellation",
			"message":   ctxErr.Error(),
			"retryable": true,
			"tool":      toolName,
			"timestamp": time.Now(),
		},
	}
}

// createInvalidParametersError creates a JSON-RPC invalid parameters error
func (h *StdioErrorHandler) createInvalidParametersError(message string) error {
	return &server.InvalidParametersError{
		Message: message,
	}
}

// formatRichErrorMessage creates a user-friendly error message from RichError
func (h *StdioErrorHandler) formatRichErrorMessage(richErr *rich.RichError) string {
	var msg strings.Builder

	// Start with severity indicator and message
	severityIcon := h.getSeverityIcon(richErr.Severity)
	msg.WriteString(fmt.Sprintf("%s %s: %s\n", severityIcon, richErr.Type, richErr.Message))

	// Add location context if available
	if richErr.Location != nil {
		msg.WriteString(fmt.Sprintf("\n📍 Location: %s:%d in %s\n",
			richErr.Location.File, richErr.Location.Line, richErr.Location.Function))
	}

	// Add context information
	if len(richErr.Context) > 0 {
		msg.WriteString("\n🔍 Context:\n")
		for key, value := range richErr.Context {
			msg.WriteString(fmt.Sprintf("  • %s: %v\n", key, value))
		}
	}

	// Add suggestion if available
	if richErr.Suggestion != "" {
		msg.WriteString(fmt.Sprintf("\n💡 Suggestion: %s\n", richErr.Suggestion))
	}

	// Add cause chain if available
	if richErr.Cause != nil {
		msg.WriteString(fmt.Sprintf("\n🔗 Caused by: %v\n", richErr.Cause))
	}

	return msg.String()
}

// formatToolErrorMessage creates a user-friendly error message from ToolError
func (h *StdioErrorHandler) formatToolErrorMessage(toolErr *types.ToolError) string {
	var msg strings.Builder

	// Start with the basic error
	msg.WriteString(fmt.Sprintf("❌ %s: %s\n", toolErr.Type, toolErr.Message))

	// Add retry information
	if toolErr.Retryable {
		msg.WriteString(fmt.Sprintf("\n🔄 Retryable: %d/%d attempts\n",
			toolErr.RetryCount, toolErr.MaxRetries))
	}

	// Add suggestions
	if len(toolErr.Suggestions) > 0 {
		msg.WriteString("\n💡 Suggestions:\n")
		for i, suggestion := range toolErr.Suggestions {
			if i < 3 { // Limit to top 3 suggestions
				msg.WriteString(fmt.Sprintf("  • %s\n", suggestion))
			}
		}
	}

	return msg.String()
}

// formatCoreErrorMessage creates a user-friendly error message from CoreError
func (h *StdioErrorHandler) formatCoreErrorMessage(coreErr *errors.CoreError) string {
	var msg strings.Builder

	// Start with the basic error
	msg.WriteString(fmt.Sprintf("❌ %s/%s: %s\n", coreErr.Category, coreErr.Module, coreErr.Message))

	// Add context if available
	if coreErr.Operation != "" {
		msg.WriteString(fmt.Sprintf("\n🔍 Context: %s → %s → %s\n",
			coreErr.Operation, coreErr.Stage, coreErr.Component))
	}

	// Add root cause if available
	if coreErr.Diagnostics != nil && coreErr.Diagnostics.RootCause != "" {
		msg.WriteString(fmt.Sprintf("\n🎯 Root Cause: %s\n", coreErr.Diagnostics.RootCause))
	}

	// Add immediate resolution steps
	if coreErr.Resolution != nil && len(coreErr.Resolution.ImmediateSteps) > 0 {
		msg.WriteString("\n🔧 Immediate Steps:\n")
		for _, step := range coreErr.Resolution.ImmediateSteps {
			msg.WriteString(fmt.Sprintf("  %d. %s\n", step.Step, step.Action))
			if step.Command != "" {
				msg.WriteString(fmt.Sprintf("     Command: %s\n", step.Command))
			}
		}
	}

	// Add alternatives if available
	if coreErr.Resolution != nil && len(coreErr.Resolution.Alternatives) > 0 {
		msg.WriteString("\n💡 Alternatives:\n")
		// Limit to top 2 alternatives
		limit := len(coreErr.Resolution.Alternatives)
		if limit > 2 {
			limit = 2
		}
		for i := 0; i < limit; i++ {
			alt := coreErr.Resolution.Alternatives[i]
			msg.WriteString(fmt.Sprintf("  %d. %s (effort: %s, risk: %s)\n",
				i+1, alt.Approach, alt.Effort, alt.Risk))
		}
	}

	// Add retry information if recommended
	if coreErr.Resolution != nil && coreErr.Resolution.RetryStrategy != nil && coreErr.Resolution.RetryStrategy.Retryable {
		msg.WriteString(fmt.Sprintf("\n🔄 Retry: Max %d attempts, backoff %dms\n",
			coreErr.Resolution.RetryStrategy.MaxAttempts, coreErr.Resolution.RetryStrategy.BackoffMs))
	}

	// Add severity information
	if coreErr.Severity == errors.SeverityCritical || coreErr.Severity == errors.SeverityHigh {
		msg.WriteString(fmt.Sprintf("\n⚠️  Severity: %s", coreErr.Severity))
		if coreErr.Fatal {
			msg.WriteString(" (FATAL)")
		}
		msg.WriteString("\n")
	}

	return msg.String()
}

// categorizeError attempts to categorize generic errors
func (h *StdioErrorHandler) categorizeError(err error) string {
	errMsg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errMsg, "network") || strings.Contains(errMsg, "connection"):
		return "network_error"
	case strings.Contains(errMsg, "timeout"):
		return "timeout_error"
	case strings.Contains(errMsg, "permission") || strings.Contains(errMsg, "denied"):
		return "permission_error"
	case strings.Contains(errMsg, "not found"):
		return "not_found_error"
	case strings.Contains(errMsg, "invalid") || strings.Contains(errMsg, "malformed"):
		return "validation_error"
	case strings.Contains(errMsg, "disk") || strings.Contains(errMsg, "space"):
		return "disk_error"
	default:
		return "generic_error"
	}
}

// isRetryableError determines if a generic error is retryable
func (h *StdioErrorHandler) isRetryableError(err error) bool {
	errMsg := strings.ToLower(err.Error())

	// Retryable errors
	retryablePatterns := []string{
		"network", "connection", "timeout", "temporary", "busy", "locked",
		"resource temporarily unavailable", "try again",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	// Non-retryable errors
	nonRetryablePatterns := []string{
		"permission", "denied", "invalid", "malformed", "not found",
		"unauthorized", "forbidden", "bad request",
	}

	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return false
		}
	}

	// Default to non-retryable for unknown errors
	return false
}

// CreateErrorResponse creates a standardized error response for stdio transport
func (h *StdioErrorHandler) CreateErrorResponse(id interface{}, code int, message string, data interface{}) map[string]interface{} {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}

	if data != nil {
		if errorMap, ok := response["error"].(map[string]interface{}); ok {
			errorMap["data"] = data
		}
	}

	return response
}

// EnhanceErrorWithContext adds additional context to error responses
func (h *StdioErrorHandler) EnhanceErrorWithContext(errorResponse map[string]interface{}, sessionID, toolName string) {
	if errorResp, ok := errorResponse["error"].(map[string]interface{}); ok {
		// Add session context
		if sessionID != "" {
			errorResp["session_id"] = sessionID
		}

		// Add tool context
		if toolName != "" {
			errorResp["tool"] = toolName
		}

		// Add transport information
		errorResp["transport"] = "stdio"
		errorResp["timestamp"] = time.Now()

		// Add debugging information for development
		errorResp["debug"] = map[string]interface{}{
			"transport_type": "stdio",
			"error_handler":  "stdio_error_handler",
			"mcp_version":    "2024-11-05",
		}
	}
}

// LogErrorMetrics logs error metrics for observability
func (h *StdioErrorHandler) LogErrorMetrics(toolName, errorType string, duration time.Duration, retryable bool) {
	h.logger.Info().
		Str("tool", toolName).
		Str("error_type", errorType).
		Dur("duration", duration).
		Bool("retryable", retryable).
		Str("transport", "stdio").
		Msg("Tool error handled")
}

// enrichGenericError converts a generic error to RichError with transport context
func (h *StdioErrorHandler) enrichGenericError(err error, toolName string) *rich.RichError {
	// Categorize the error type and severity
	errorType := h.categorizeErrorType(err)
	severity := h.determineSeverity(err)
	retryable := h.isRetryableError(err)

	// Build RichError with transport context
	builder := rich.NewError().
		Code(rich.ErrorCode(h.generateErrorCode(errorType))).
		Message(err.Error()).
		Type(errorType).
		Severity(severity).
		Context("transport", "stdio").
		Context("tool_name", toolName).
		Context("error_category", h.categorizeError(err)).
		Context("retryable", retryable).
		WithLocation()

	// Add specific suggestion based on error type
	suggestion := h.generateSuggestion(err, errorType)
	builder = builder.Suggestion(suggestion)

	return builder.Build()
}

// categorizeErrorType converts string categorization to RichError types
func (h *StdioErrorHandler) categorizeErrorType(err error) rich.ErrorType {
	errorCategory := h.categorizeError(err)
	switch errorCategory {
	case "network_error":
		return rich.ErrTypeNetwork
	case "timeout_error":
		return rich.ErrTypeTimeout
	case "permission_error":
		return rich.ErrTypePermission
	case "not_found_error":
		return rich.ErrTypeNotFound
	case "validation_error":
		return rich.ErrTypeValidation
	case "disk_error":
		return rich.ErrTypeSystem
	default:
		return rich.ErrTypeBusiness
	}
}

// determineSeverity determines error severity based on content
func (h *StdioErrorHandler) determineSeverity(err error) rich.ErrorSeverity {
	errMsg := strings.ToLower(err.Error())

	// Critical errors that prevent further operation
	if strings.Contains(errMsg, "fatal") || strings.Contains(errMsg, "panic") ||
		strings.Contains(errMsg, "corrupted") || strings.Contains(errMsg, "critical") {
		return rich.SeverityCritical
	}

	// High severity errors that significantly impact functionality
	if strings.Contains(errMsg, "permission denied") || strings.Contains(errMsg, "unauthorized") ||
		strings.Contains(errMsg, "disk full") || strings.Contains(errMsg, "out of memory") {
		return rich.SeverityHigh
	}

	// Medium severity for operational issues
	if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "connection") ||
		strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "invalid") {
		return rich.SeverityMedium
	}

	// Low severity for minor issues
	return rich.SeverityLow
}

// generateErrorCode creates a specific error code based on type
func (h *StdioErrorHandler) generateErrorCode(errorType rich.ErrorType) string {
	switch errorType {
	case rich.ErrTypeNetwork:
		return "STDIO_NETWORK_ERROR"
	case rich.ErrTypeTimeout:
		return "STDIO_TIMEOUT_ERROR"
	case rich.ErrTypePermission:
		return "STDIO_PERMISSION_ERROR"
	case rich.ErrTypeNotFound:
		return "STDIO_NOT_FOUND_ERROR"
	case rich.ErrTypeValidation:
		return "STDIO_VALIDATION_ERROR"
	case rich.ErrTypeSystem:
		return "STDIO_SYSTEM_ERROR"
	default:
		return "STDIO_GENERIC_ERROR"
	}
}

// generateSuggestion provides contextual suggestion based on error type
func (h *StdioErrorHandler) generateSuggestion(err error, errorType rich.ErrorType) string {
	switch errorType {
	case rich.ErrTypeNetwork:
		return "Check network connectivity and verify proxy settings if behind a corporate firewall"
	case rich.ErrTypeTimeout:
		return "Increase timeout values in configuration or check system performance"
	case rich.ErrTypePermission:
		return "Check file and directory permissions or run with appropriate user privileges"
	case rich.ErrTypeNotFound:
		return "Verify the requested resource exists and check path spelling"
	case rich.ErrTypeValidation:
		return "Review input parameters for correctness and validate against expected schema"
	default:
		return "Check logs for additional error details or retry if the error is transient"
	}
}

// getSeverityIcon returns an appropriate icon for error severity
func (h *StdioErrorHandler) getSeverityIcon(severity rich.ErrorSeverity) string {
	switch severity {
	case rich.SeverityCritical:
		return "🚨"
	case rich.SeverityHigh:
		return "❌"
	case rich.SeverityMedium:
		return "⚠️"
	case rich.SeverityLow:
		return "ℹ️"
	default:
		return "❓"
	}
}

// CreateRecoveryResponse creates a response with recovery guidance using RichError
func (h *StdioErrorHandler) CreateRecoveryResponse(originalError error, recoverySteps, alternatives []string) interface{} {
	// Convert to RichError with recovery context
	richErr := rich.NewError().
		Code(rich.ErrorCode("STDIO_RECOVERY_AVAILABLE")).
		Message(originalError.Error()).
		Type(rich.ErrTypeBusiness).
		Severity(rich.SeverityMedium).
		Context("transport", "stdio").
		Context("recovery_available", true).
		Context("recovery_steps_count", len(recoverySteps)).
		Context("alternatives_count", len(alternatives)).
		WithLocation()

	// Create combined recovery suggestion
	var suggestionBuilder strings.Builder
	if len(recoverySteps) > 0 {
		suggestionBuilder.WriteString("Recovery steps: ")
		for i, step := range recoverySteps {
			if i > 0 {
				suggestionBuilder.WriteString("; ")
			}
			suggestionBuilder.WriteString(step)
		}
	}

	if len(alternatives) > 0 {
		if suggestionBuilder.Len() > 0 {
			suggestionBuilder.WriteString(". ")
		}
		suggestionBuilder.WriteString("Alternatives: ")
		for i, alt := range alternatives {
			if i > 0 {
				suggestionBuilder.WriteString("; ")
			}
			suggestionBuilder.WriteString(alt)
		}
	}

	if suggestionBuilder.Len() > 0 {
		richErr = richErr.Suggestion(suggestionBuilder.String())
	}

	builtErr := richErr.Build()
	return h.handleRichError(builtErr, "recovery")
}
