package build

import (
	"context"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/common/validation-core/validators"
	"github.com/rs/zerolog"
)

// UnifiedContextValidator provides context validation using the unified validation framework
type UnifiedContextValidator struct {
	logger           zerolog.Logger
	contextValidator *validators.ContextValidator
}

// NewUnifiedContextValidator creates a context validator that uses the unified framework
func NewUnifiedContextValidator(logger zerolog.Logger) *UnifiedContextValidator {
	return &UnifiedContextValidator{
		logger:           logger.With().Str("component", "unified_context_validator").Logger(),
		contextValidator: validators.NewContextValidator(),
	}
}

// Validate performs context validation using the unified framework
func (v *UnifiedContextValidator) Validate(content string, options ValidationOptions) (*BuildValidationResult, error) {
	v.logger.Info().
		Str("mode", "unified").
		Msg("Starting build context validation with unified framework")

	// Extract context operations from Dockerfile content
	instructions := extractContextInstructions(content)

	// Get context path from options or use default
	contextPath := "./"

	// Use unified validator directly
	ctx := context.Background()
	coreOptions := ConvertToUnifiedOptions(options)

	// Prepare context data for validation
	contextData := map[string]interface{}{
		"context_path": contextPath,
		"instructions": instructions,
		"content":      content,
	}

	unifiedResult := v.contextValidator.Validate(ctx, contextData, coreOptions)

	// Convert to build result format
	result := ConvertToUnifiedResult(unifiedResult)

	v.logger.Info().
		Bool("valid", result.Valid).
		Int("github.com/Azure/container-kit/pkg/mcp/errors", len(result.Errors)).
		Int("warnings", len(result.Warnings)).
		Msg("Context validation completed")

	return result, nil
}

// ValidateWithContext performs validation with additional context information
func (v *UnifiedContextValidator) ValidateWithContext(content string, contextPath string, buildFiles []string) (*BuildValidationResult, error) {
	v.logger.Info().
		Str("context_path", contextPath).
		Int("build_files", len(buildFiles)).
		Msg("Performing enhanced context validation")

	// First validate the Dockerfile operations
	options := ValidationOptions{}
	result, err := v.Validate(content, options)
	if err != nil {
		return nil, err
	}

	// Additionally validate build files if provided
	if len(buildFiles) > 0 {
		filesResult := v.contextValidator.Validate(
			context.Background(),
			buildFiles,
			&core.ValidationOptions{
				Context: map[string]interface{}{
					"validation_type": "build_files",
				},
			},
		)
		if filesResult != nil {
			// Merge the results
			mergedResult := ConvertToUnifiedResult(filesResult)
			result.Valid = result.Valid && mergedResult.Valid
			result.Errors = append(result.Errors, mergedResult.Errors...)
			result.Warnings = append(result.Warnings, mergedResult.Warnings...)
			// Note: Info field is no longer available in unified validation result
			// Suggestions are used instead of info messages
			result.Suggestions = append(result.Suggestions, mergedResult.Suggestions...)
		}
	}

	return result, nil
}

// MigrateContextValidatorToUnified provides a drop-in replacement for the old ContextValidator
// Legacy function - kept for compatibility during migration period
func MigrateContextValidatorToUnified(logger zerolog.Logger) *UnifiedContextValidator {
	return NewUnifiedContextValidator(logger)
}

// CreateContextValidatorWithUnified creates a context validator using the unified framework
// This function can be used as a drop-in replacement for NewContextValidator
func CreateContextValidatorWithUnified(logger zerolog.Logger) *UnifiedContextValidator {
	logger.Info().Msg("Creating context validator with unified validation framework")
	return NewUnifiedContextValidator(logger)
}
