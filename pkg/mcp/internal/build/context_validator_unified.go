package build

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/validation/core"
	"github.com/rs/zerolog"
)

// UnifiedContextValidator provides context validation using the unified validation framework
type UnifiedContextValidator struct {
	logger  zerolog.Logger
	adapter *ValidationAdapter
}

// NewUnifiedContextValidator creates a context validator that uses the unified framework
func NewUnifiedContextValidator(logger zerolog.Logger) *UnifiedContextValidator {
	return &UnifiedContextValidator{
		logger:  logger.With().Str("component", "unified_context_validator").Logger(),
		adapter: NewValidationAdapter(logger),
	}
}

// Validate performs context validation using the unified framework
func (v *UnifiedContextValidator) Validate(content string, options ValidationOptions) (*ValidationResult, error) {
	v.logger.Info().
		Str("mode", "unified").
		Msg("Starting build context validation with unified framework")

	// Extract context operations from Dockerfile content
	instructions := extractContextInstructions(content)

	// Get context path from options or use default
	contextPath := "./"

	// Use the adapter to validate
	result, err := v.adapter.ValidateContext(contextPath, instructions)
	if err != nil {
		return nil, err
	}

	v.logger.Info().
		Bool("valid", result.Valid).
		Int("errors", len(result.Errors)).
		Int("warnings", len(result.Warnings)).
		Msg("Context validation completed")

	return result, nil
}

// ValidateWithContext performs validation with additional context information
func (v *UnifiedContextValidator) ValidateWithContext(content string, contextPath string, buildFiles []string) (*ValidationResult, error) {
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
		filesResult := v.adapter.contextValidator.Validate(
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
			result.Info = append(result.Info, mergedResult.Info...)
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
