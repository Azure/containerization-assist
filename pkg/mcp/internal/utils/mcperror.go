package utils

import (
	"fmt"
	"strings"

	v20250326 "github.com/localrivet/gomcp/mcp/v20250326"
)

// MCPError represents an error with MCP error code and structured data
type MCPError struct {
	Code    v20250326.ErrorCode `json:"code"`
	Message string              `json:"message"`
	Data    interface{}         `json:"data,omitempty"`
}

// Error implements the error interface
func (e *MCPError) Error() string {
	return e.Message
}

// GetCode returns the MCP error code
func (e *MCPError) GetCode() v20250326.ErrorCode {
	return e.Code
}

// GetData returns the error data
func (e *MCPError) GetData() interface{} {
	return e.Data
}

// New creates a new MCP error with the specified code and message
func New(code v20250326.ErrorCode, message string) *MCPError {
	return &MCPError{
		Code:    code,
		Message: message,
	}
}

// NewWithData creates a new MCP error with code, message, and additional data
func NewWithData(code v20250326.ErrorCode, message string, data interface{}) *MCPError {
	return &MCPError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// Wrap creates a new MCP error by wrapping an existing error
func Wrap(code v20250326.ErrorCode, message string, err error) *MCPError {
	var data interface{}
	if err != nil {
		data = map[string]interface{}{
			"original_error": err.Error(),
		}
	}

	fullMessage := message
	if err != nil {
		fullMessage = fmt.Sprintf("%s: %v", message, err)
	}

	return &MCPError{
		Code:    code,
		Message: fullMessage,
		Data:    data,
	}
}

// MCP error codes mapped to common application scenarios
// These map our custom error types to standardized MCP error codes

// Session-related errors
var (
	CodeSessionNotFound        = v20250326.ErrorCodeInvalidRequest      // Session doesn't exist
	CodeSessionExpired         = v20250326.ErrorCodeInvalidRequest      // Session has expired
	CodeSessionExists          = v20250326.ErrorCodeInvalidArguments    // Session already exists
	CodeWorkspaceQuotaExceeded = v20250326.ErrorCodeInternalServerError // Workspace quota exceeded
	CodeMaxSessionsReached     = v20250326.ErrorCodeInternalServerError // Max sessions reached
	CodeSessionCorrupted       = v20250326.ErrorCodeInternalServerError // Session data corrupted
)

// Workflow/State errors
var (
	CodeAnalysisRequired   = v20250326.ErrorCodeInvalidRequest // Repository analysis required
	CodeDockerfileRequired = v20250326.ErrorCodeInvalidRequest // Dockerfile required
	CodeBuildRequired      = v20250326.ErrorCodeInvalidRequest // Successful build required
	CodeImageRequired      = v20250326.ErrorCodeInvalidRequest // Built image required
	CodeManifestsRequired  = v20250326.ErrorCodeInvalidRequest // K8s manifests required
	CodeStageNotReady      = v20250326.ErrorCodeInvalidRequest // Stage prerequisites not met
)

// Validation errors
var (
	CodeRequiredFieldMissing = v20250326.ErrorCodeInvalidArguments // Required field missing
	CodeInvalidFormat        = v20250326.ErrorCodeInvalidArguments // Invalid format
	CodeInvalidPath          = v20250326.ErrorCodeInvalidArguments // Invalid path
	CodeInvalidImageName     = v20250326.ErrorCodeInvalidArguments // Invalid image name
	CodeInvalidNamespace     = v20250326.ErrorCodeInvalidArguments // Invalid namespace
	CodeUnsupportedOperation = v20250326.ErrorCodeInvalidRequest   // Unsupported operation
)

// Infrastructure errors
var (
	CodeServiceUnavailable = v20250326.ErrorCodeInternalServerError // Service unavailable
	CodeTimeoutError       = v20250326.ErrorCodeInternalServerError // Operation timeout
	CodePermissionDenied   = v20250326.ErrorCodeInvalidRequest      // Permission denied
	CodeNetworkError       = v20250326.ErrorCodeInternalServerError // Network error
	CodeDiskFull           = v20250326.ErrorCodeInternalServerError // Disk full
	CodeQuotaExceeded      = v20250326.ErrorCodeInternalServerError // Quota exceeded
)

// Build/Deploy specific errors
var (
	CodeDockerfileInvalid = v20250326.ErrorCodeInvalidArguments    // Dockerfile invalid
	CodeBuildFailed       = v20250326.ErrorCodeInternalServerError // Build failed
	CodeImagePushFailed   = v20250326.ErrorCodeInternalServerError // Image push failed
	CodeManifestInvalid   = v20250326.ErrorCodeInvalidArguments    // Manifest invalid
	CodeDeploymentFailed  = v20250326.ErrorCodeInternalServerError // Deployment failed
	CodeHealthCheckFailed = v20250326.ErrorCodeInternalServerError // Health check failed
)

// Helper functions for creating wrapped errors with context

// WrapSessionError wraps session-related errors with additional context
func WrapSessionError(err error, sessionID string) *MCPError {
	if err == nil {
		return nil
	}

	data := map[string]interface{}{
		"session_id":     sessionID,
		"original_error": err.Error(),
	}

	return &MCPError{
		Code:    CodeSessionNotFound,
		Message: fmt.Sprintf("session %s: %v", sessionID, err),
		Data:    data,
	}
}

// WrapValidationError wraps validation errors with field information
func WrapValidationError(err error, field string) *MCPError {
	if err == nil {
		return nil
	}

	data := map[string]interface{}{
		"field":          field,
		"original_error": err.Error(),
	}

	return &MCPError{
		Code:    CodeInvalidFormat,
		Message: fmt.Sprintf("field '%s': %v", field, err),
		Data:    data,
	}
}

// WrapWorkflowError wraps workflow errors with stage information
func WrapWorkflowError(err error, stage string) *MCPError {
	if err == nil {
		return nil
	}

	data := map[string]interface{}{
		"stage":          stage,
		"original_error": err.Error(),
	}

	return &MCPError{
		Code:    CodeStageNotReady,
		Message: fmt.Sprintf("stage %s: %v", stage, err),
		Data:    data,
	}
}

// WrapInfrastructureError wraps infrastructure errors with service information
func WrapInfrastructureError(err error, service string) *MCPError {
	if err == nil {
		return nil
	}

	data := map[string]interface{}{
		"service":        service,
		"original_error": err.Error(),
	}

	return &MCPError{
		Code:    CodeServiceUnavailable,
		Message: fmt.Sprintf("service %s: %v", service, err),
		Data:    data,
	}
}

// Common error creation functions

// NewSessionNotFound creates a session not found error
func NewSessionNotFound(sessionID string) *MCPError {
	return NewWithData(CodeSessionNotFound, "session not found", map[string]interface{}{
		"session_id": sessionID,
	})
}

// NewSessionExpired creates a session expired error
func NewSessionExpired(sessionID string) *MCPError {
	return NewWithData(CodeSessionExpired, "session expired", map[string]interface{}{
		"session_id": sessionID,
	})
}

// NewBuildFailed creates a build failed error
func NewBuildFailed(message string) *MCPError {
	return New(CodeBuildFailed, fmt.Sprintf("docker build failed: %s", message))
}

// NewDockerfileInvalid creates a dockerfile invalid error
func NewDockerfileInvalid(message string) *MCPError {
	return New(CodeDockerfileInvalid, fmt.Sprintf("dockerfile invalid: %s", message))
}

// NewDeploymentFailed creates a deployment failed error
func NewDeploymentFailed(message string) *MCPError {
	return New(CodeDeploymentFailed, fmt.Sprintf("deployment failed: %s", message))
}

// NewRequiredFieldMissing creates a required field missing error
func NewRequiredFieldMissing(field string) *MCPError {
	return NewWithData(CodeRequiredFieldMissing, "required field missing", map[string]interface{}{
		"field": field,
	})
}

// IsSessionError checks if an error is session-related by examining the error data
func IsSessionError(err error) bool {
	if mcpErr, ok := err.(*MCPError); ok {
		// Check if this error has session-related data
		if data, ok := mcpErr.Data.(map[string]interface{}); ok {
			if _, hasSessionID := data["session_id"]; hasSessionID {
				return true
			}
		}
		// Also check error message for session-related content
		return strings.Contains(strings.ToLower(mcpErr.Message), "session")
	}
	return false
}

// IsWorkflowError checks if an error is workflow/state-related
func IsWorkflowError(err error) bool {
	if mcpErr, ok := err.(*MCPError); ok {
		return mcpErr.Code == CodeAnalysisRequired ||
			mcpErr.Code == CodeDockerfileRequired ||
			mcpErr.Code == CodeBuildRequired ||
			mcpErr.Code == CodeImageRequired ||
			mcpErr.Code == CodeManifestsRequired ||
			mcpErr.Code == CodeStageNotReady
	}
	return false
}

// IsValidationError checks if an error is validation-related
func IsValidationError(err error) bool {
	if mcpErr, ok := err.(*MCPError); ok {
		return mcpErr.Code == CodeRequiredFieldMissing ||
			mcpErr.Code == CodeInvalidFormat ||
			mcpErr.Code == CodeInvalidPath ||
			mcpErr.Code == CodeInvalidImageName ||
			mcpErr.Code == CodeInvalidNamespace ||
			mcpErr.Code == CodeUnsupportedOperation
	}
	return false
}

// IsInfrastructureError checks if an error is infrastructure-related
func IsInfrastructureError(err error) bool {
	if mcpErr, ok := err.(*MCPError); ok {
		return mcpErr.Code == CodeServiceUnavailable ||
			mcpErr.Code == CodeTimeoutError ||
			mcpErr.Code == CodePermissionDenied ||
			mcpErr.Code == CodeNetworkError ||
			mcpErr.Code == CodeDiskFull ||
			mcpErr.Code == CodeQuotaExceeded
	}
	return false
}

// IsBuildError checks if an error is build/deploy-related
func IsBuildError(err error) bool {
	if mcpErr, ok := err.(*MCPError); ok {
		return mcpErr.Code == CodeDockerfileInvalid ||
			mcpErr.Code == CodeBuildFailed ||
			mcpErr.Code == CodeImagePushFailed ||
			mcpErr.Code == CodeManifestInvalid ||
			mcpErr.Code == CodeDeploymentFailed ||
			mcpErr.Code == CodeHealthCheckFailed
	}
	return false
}

// ToMCPErrorResponse converts an MCPError to a JSON-RPC error response
func (e *MCPError) ToMCPErrorResponse(id interface{}) *v20250326.ErrorResponse {
	return &v20250326.ErrorResponse{
		Code:    e.Code,
		Message: e.Message,
	}
}

// FromError creates an MCPError from a standard Go error, trying to map it to appropriate MCP codes
func FromError(err error) *MCPError {
	if err == nil {
		return nil
	}

	if mcpErr, ok := err.(*MCPError); ok {
		return mcpErr
	}

	// Try to map common error patterns to MCP codes
	errStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errStr, "not found"):
		if strings.Contains(errStr, "session") {
			return NewSessionNotFound("")
		}
		return New(v20250326.ErrorCodeResourceNotFound, err.Error())

	case strings.Contains(errStr, "build") && strings.Contains(errStr, "failed"):
		return NewBuildFailed(err.Error())

	case strings.Contains(errStr, "dockerfile") && strings.Contains(errStr, "invalid"):
		return NewDockerfileInvalid(err.Error())

	case strings.Contains(errStr, "deploy") && strings.Contains(errStr, "failed"):
		return NewDeploymentFailed(err.Error())

	case strings.Contains(errStr, "invalid") || strings.Contains(errStr, "malformed"):
		return New(v20250326.ErrorCodeInvalidArguments, err.Error())

	case strings.Contains(errStr, "permission") || strings.Contains(errStr, "forbidden"):
		return New(v20250326.ErrorCodeInvalidRequest, err.Error())

	default:
		return New(v20250326.ErrorCodeInternalServerError, err.Error())
	}
}

// ErrorCategory represents a category of errors with common handling
type ErrorCategory struct {
	Code           string
	Name           string
	Description    string
	DefaultMessage string
	Retryable      bool
	UserGuidance   string
	RecoverySteps  []string
}

// GetErrorCategory returns error category information for an MCP error code
func GetErrorCategory(code v20250326.ErrorCode) (*ErrorCategory, bool) {
	category, exists := errorCategoryMapping[string(code)]
	if exists {
		return &category, true
	}
	return nil, false
}

// errorCategoryMapping provides centralized error code to category mapping using MCP error codes
var errorCategoryMapping = map[string]ErrorCategory{
	// Invalid arguments errors (Dockerfile invalid, manifest invalid, etc.)
	string(v20250326.ErrorCodeInvalidArguments): {
		Code:           string(v20250326.ErrorCodeInvalidArguments),
		Name:           "Invalid Arguments",
		Description:    "The provided arguments or configuration are invalid",
		DefaultMessage: "Invalid arguments provided. Please check the input parameters.",
		Retryable:      false,
		UserGuidance:   "Review and fix the invalid parameters",
		RecoverySteps: []string{
			"Check argument syntax and format",
			"Verify required fields are present",
			"Ensure values match expected patterns",
			"Review documentation for correct usage",
		},
	},

	// Internal server errors (build failed, deploy failed, etc.)
	string(v20250326.ErrorCodeInternalServerError): {
		Code:           string(v20250326.ErrorCodeInternalServerError),
		Name:           "Internal Server Error",
		Description:    "An internal error occurred during operation",
		DefaultMessage: "An internal error occurred. Please retry or contact support.",
		Retryable:      true,
		UserGuidance:   "Check system resources and connectivity",
		RecoverySteps: []string{
			"Retry the operation",
			"Check system resource availability",
			"Verify network connectivity",
			"Review system logs for details",
			"Contact support if issue persists",
		},
	},

	// Invalid request errors (session not found, permission denied, etc.)
	string(v20250326.ErrorCodeInvalidRequest): {
		Code:           string(v20250326.ErrorCodeInvalidRequest),
		Name:           "Invalid Request",
		Description:    "The request is invalid or cannot be processed",
		DefaultMessage: "Invalid request. Please check the request parameters.",
		Retryable:      false,
		UserGuidance:   "Verify request format and permissions",
		RecoverySteps: []string{
			"Check request syntax",
			"Verify you have necessary permissions",
			"Ensure required resources exist",
			"Review API documentation",
		},
	},

	// Resource not found errors
	string(v20250326.ErrorCodeResourceNotFound): {
		Code:           string(v20250326.ErrorCodeResourceNotFound),
		Name:           "Resource Not Found",
		Description:    "The requested resource could not be found",
		DefaultMessage: "Resource not found. Please check the resource identifier.",
		Retryable:      false,
		UserGuidance:   "Verify the resource exists and is accessible",
		RecoverySteps: []string{
			"Check resource identifier spelling",
			"Verify resource exists",
			"Ensure you have access permissions",
			"Create the resource if needed",
		},
	},
}

// GetUserFriendlyMessage returns a user-friendly message for an MCP error
func GetUserFriendlyMessage(mcpErr *MCPError) string {
	if category, ok := GetErrorCategory(mcpErr.Code); ok {
		return category.DefaultMessage
	}
	return mcpErr.Message
}

// ShouldRetry determines if an MCP error is retryable
func ShouldRetry(mcpErr *MCPError) bool {
	if category, ok := GetErrorCategory(mcpErr.Code); ok {
		return category.Retryable
	}
	return false
}

// GetRecoverySteps returns recovery steps for an MCP error
func GetRecoverySteps(mcpErr *MCPError) []string {
	if category, ok := GetErrorCategory(mcpErr.Code); ok {
		return category.RecoverySteps
	}
	return []string{"Check error details", "Review logs for more information"}
}
