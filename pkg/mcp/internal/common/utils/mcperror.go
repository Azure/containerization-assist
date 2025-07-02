package utils

import (
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/errors/rich"
	v20250326 "github.com/localrivet/gomcp/mcp/v20250326"
)

// MCPError represents a legacy MCP error
// Deprecated: Use rich.RichError instead for better error context and type safety
type MCPError struct {
	Code    v20250326.ErrorCode `json:"code"`
	Message string              `json:"message"`
	Data    interface{}         `json:"data,omitempty"` // Deprecated: interface{} usage
}

func (e *MCPError) Error() string {
	return e.Message
}

func (e *MCPError) GetCode() v20250326.ErrorCode {
	return e.Code
}

// GetData returns the error data
// Deprecated: Use rich.RichError.Context instead for type-safe context
func (e *MCPError) GetData() interface{} {
	return e.Data
}

// New creates a new MCPError
// Deprecated: Use rich.NewError() instead for better error context
func New(code v20250326.ErrorCode, message string) *MCPError {
	return &MCPError{
		Code:    code,
		Message: message,
	}
}

// NewWithData creates a new MCPError with data
// Deprecated: Use rich.NewError().Context() instead for type-safe context
func NewWithData(code v20250326.ErrorCode, message string, data interface{}) *MCPError {
	return &MCPError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// Wrap wraps an error with an MCPError
// Deprecated: Use rich.NewError().Cause() instead for proper error chaining
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

var (
	CodeSessionNotFound        = v20250326.ErrorCodeInvalidRequest
	CodeSessionExpired         = v20250326.ErrorCodeInvalidRequest
	CodeSessionExists          = v20250326.ErrorCodeInvalidArguments
	CodeWorkspaceQuotaExceeded = v20250326.ErrorCodeInternalServerError
	CodeMaxSessionsReached     = v20250326.ErrorCodeInternalServerError
	CodeSessionCorrupted       = v20250326.ErrorCodeInternalServerError
)

var (
	CodeAnalysisRequired   = v20250326.ErrorCodeInvalidRequest
	CodeDockerfileRequired = v20250326.ErrorCodeInvalidRequest
	CodeBuildRequired      = v20250326.ErrorCodeInvalidRequest
	CodeImageRequired      = v20250326.ErrorCodeInvalidRequest
	CodeManifestsRequired  = v20250326.ErrorCodeInvalidRequest
	CodeStageNotReady      = v20250326.ErrorCodeInvalidRequest
)

var (
	CodeRequiredFieldMissing = v20250326.ErrorCodeInvalidArguments
	CodeInvalidFormat        = v20250326.ErrorCodeInvalidArguments
	CodeInvalidPath          = v20250326.ErrorCodeInvalidArguments
	CodeInvalidImageName     = v20250326.ErrorCodeInvalidArguments
	CodeInvalidNamespace     = v20250326.ErrorCodeInvalidArguments
	CodeUnsupportedOperation = v20250326.ErrorCodeInvalidRequest
)

var (
	CodeServiceUnavailable = v20250326.ErrorCodeInternalServerError
	CodeTimeoutError       = v20250326.ErrorCodeInternalServerError
	CodePermissionDenied   = v20250326.ErrorCodeInvalidRequest
	CodeNetworkError       = v20250326.ErrorCodeInternalServerError
	CodeDiskFull           = v20250326.ErrorCodeInternalServerError
	CodeQuotaExceeded      = v20250326.ErrorCodeInternalServerError
)

var (
	CodeDockerfileInvalid = v20250326.ErrorCodeInvalidArguments
	CodeBuildFailed       = v20250326.ErrorCodeInternalServerError
	CodeImagePushFailed   = v20250326.ErrorCodeInternalServerError
	CodeManifestInvalid   = v20250326.ErrorCodeInvalidArguments
	CodeDeploymentFailed  = v20250326.ErrorCodeInternalServerError
	CodeHealthCheckFailed = v20250326.ErrorCodeInternalServerError
)

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

// NewSessionNotFound creates a session not found error
// Deprecated: Use NewRichSessionError instead
func NewSessionNotFound(sessionID string) *MCPError {
	return NewWithData(CodeSessionNotFound, "session not found", map[string]interface{}{
		"session_id": sessionID,
	})
}

// NewSessionExpired creates a session expired error
// Deprecated: Use NewRichSessionError instead
func NewSessionExpired(sessionID string) *MCPError {
	return NewWithData(CodeSessionExpired, "session expired", map[string]interface{}{
		"session_id": sessionID,
	})
}

// NewBuildFailed creates a build failed error
// Deprecated: Use rich.NewError() with rich.CodeImageBuildFailed instead
func NewBuildFailed(message string) *MCPError {
	return New(CodeBuildFailed, fmt.Sprintf("docker build failed: %s", message))
}

func NewDockerfileInvalid(message string) *MCPError {
	return New(CodeDockerfileInvalid, fmt.Sprintf("dockerfile invalid: %s", message))
}

func NewDeploymentFailed(message string) *MCPError {
	return New(CodeDeploymentFailed, fmt.Sprintf("deployment failed: %s", message))
}

func NewRequiredFieldMissing(field string) *MCPError {
	return NewWithData(CodeRequiredFieldMissing, "required field missing", map[string]interface{}{
		"field": field,
	})
}

func IsSessionError(err error) bool {
	if mcpErr, ok := err.(*MCPError); ok {
		if data, ok := mcpErr.Data.(map[string]interface{}); ok {
			if _, hasSessionID := data["session_id"]; hasSessionID {
				return true
			}
		}
		return strings.Contains(strings.ToLower(mcpErr.Message), "session")
	}
	return false
}

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

func (e *MCPError) ToMCPErrorResponse(id interface{}) *v20250326.ErrorResponse {
	return &v20250326.ErrorResponse{
		Code:    e.Code,
		Message: e.Message,
	}
}

func FromError(err error) *MCPError {
	if err == nil {
		return nil
	}

	if mcpErr, ok := err.(*MCPError); ok {
		return mcpErr
	}

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

type ErrorCategory struct {
	Code           string
	Name           string
	Description    string
	DefaultMessage string
	Retryable      bool
	UserGuidance   string
	RecoverySteps  []string
}

func GetErrorCategory(code v20250326.ErrorCode) (*ErrorCategory, bool) {
	category, exists := errorCategoryMapping[string(code)]
	if exists {
		return &category, true
	}
	return nil, false
}

var errorCategoryMapping = map[string]ErrorCategory{
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

func GetUserFriendlyMessage(mcpErr *MCPError) string {
	if category, ok := GetErrorCategory(mcpErr.Code); ok {
		return category.DefaultMessage
	}
	return mcpErr.Message
}

func ShouldRetry(mcpErr *MCPError) bool {
	if category, ok := GetErrorCategory(mcpErr.Code); ok {
		return category.Retryable
	}
	return false
}

func GetRecoverySteps(mcpErr *MCPError) []string {
	if category, ok := GetErrorCategory(mcpErr.Code); ok {
		return category.RecoverySteps
	}
	return []string{"Check error details", "Review logs for more information"}
}

// Rich Error Migration Helpers
// These functions help migrate from legacy MCPError to rich.RichError

// NewRichSessionError creates a rich error for session-related issues
func NewRichSessionError(sessionID, message string) *rich.RichError {
	return rich.NewError().
		Code(rich.CodeResourceNotFound).
		Type(rich.ErrTypeResource).
		Severity(rich.SeverityMedium).
		Message(message).
		Context("module", "session_manager").
		Context("session_id", sessionID).
		Suggestion("Check if the session exists and is not expired").
		WithLocation().
		Build()
}

// NewRichValidationError creates a rich error for validation issues
func NewRichValidationError(field, message string) *rich.RichError {
	return rich.NewError().
		Code(rich.CodeValidationFailed).
		Type(rich.ErrTypeValidation).
		Severity(rich.SeverityMedium).
		Message(message).
		Context("module", "validation").
		Context("field", field).
		Suggestion(fmt.Sprintf("Check the format and value of field '%s'", field)).
		WithLocation().
		Build()
}

// NewRichWorkflowError creates a rich error for workflow stage issues
func NewRichWorkflowError(stage, message string) *rich.RichError {
	return rich.NewError().
		Code(rich.CodeInvalidParameter).
		Type(rich.ErrTypeBusiness).
		Severity(rich.SeverityMedium).
		Message(message).
		Context("module", "workflow").
		Context("stage", stage).
		Suggestion(fmt.Sprintf("Complete the prerequisite steps for stage '%s'", stage)).
		WithLocation().
		Build()
}

// NewRichInfrastructureError creates a rich error for infrastructure issues
func NewRichInfrastructureError(service, message string) *rich.RichError {
	return rich.NewError().
		Code(rich.CodeNetworkTimeout).
		Type(rich.ErrTypeSystem).
		Severity(rich.SeverityHigh).
		Message(message).
		Context("module", "infrastructure").
		Context("service", service).
		Suggestion("Check service availability and network connectivity").
		WithLocation().
		Build()
}
