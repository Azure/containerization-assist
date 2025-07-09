package runtime

import "github.com/Azure/container-kit/pkg/mcp/domain/errors"

// NewRichSessionError creates a rich error for session-related issues
func NewRichSessionError(sessionID, message string) error {
	return errors.NewError().
		Code(errors.CodeInternalError).
		Type(errors.ErrTypeInternal).
		Severity(errors.SeverityMedium).
		Message(message).
		Context("session_id", sessionID).
		WithLocation().
		Build()
}

// NewRichValidationError creates a rich error for validation failures
func NewRichValidationError(field, message string, value interface{}) error {
	return errors.NewError().
		Code(errors.CodeValidationFailed).
		Type(errors.ErrTypeValidation).
		Severity(errors.SeverityMedium).
		Message(message).
		Context("field", field).
		Context("value", value).
		WithLocation().
		Build()
}
