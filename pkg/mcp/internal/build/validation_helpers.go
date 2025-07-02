package build

import (
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/internal/common/utils"
	"github.com/rs/zerolog"
)

// ValidateSessionID provides standardized session ID validation across all atomic tools
func ValidateSessionID(sessionID string, toolName string, logger zerolog.Logger) error {
	mixin := utils.NewStandardizedValidationMixin(logger)
	result := mixin.StandardValidateRequiredFields(
		struct{ SessionID string }{SessionID: sessionID},
		[]string{"SessionID"},
	)
	if result.HasErrors() {
		return fmt.Errorf("session_id is required and cannot be empty for tool %s", toolName)
	}
	return nil
}

// ValidateImageReference provides standardized Docker image reference validation
func ValidateImageReference(imageRef, fieldName string, logger zerolog.Logger) error {
	mixin := utils.NewStandardizedValidationMixin(logger)
	result := mixin.StandardValidateImageRef(imageRef, fieldName)
	if result.HasErrors() {
		firstError := result.GetFirstError()
		return fmt.Errorf("invalid image reference for field %s: %s", fieldName, firstError.Message)
	}
	return nil
}
