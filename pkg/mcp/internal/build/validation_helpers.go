package build

import (
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/Azure/container-copilot/pkg/mcp/internal/utils"
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
		return types.NewRichError(
			"INVALID_ARGUMENTS",
			"session_id is required and cannot be empty",
			"validation_error",
		)
	}

	return nil
}

// ValidateImageReference provides standardized Docker image reference validation
func ValidateImageReference(imageRef, fieldName string, logger zerolog.Logger) error {
	mixin := utils.NewStandardizedValidationMixin(logger)
	result := mixin.StandardValidateImageRef(imageRef, fieldName)

	if result.HasErrors() {
		firstError := result.GetFirstError()
		return types.NewRichError(
			firstError.Code,
			firstError.Message,
			"validation_error",
		)
	}

	return nil
}
