package build

import (
	"context"

	"github.com/Azure/container-kit/pkg/common/validation-core/validators"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/rs/zerolog"
)

// UnifiedSyntaxValidator provides Dockerfile syntax validation using the unified validation framework.
//
// This validator focuses on:
// 1. Dockerfile syntax validation using external tools (hadolint, etc.)
// 2. Rule-based filtering and severity filtering (business logic)
// 3. Result transformation and formatting
//
// Simple argument validation is handled by the tag-based validation system in the calling tools.
type UnifiedSyntaxValidator struct {
	logger          zerolog.Logger
	dockerValidator *validators.DockerfileValidator
}

// NewUnifiedSyntaxValidator creates a syntax validator that uses the unified framework
func NewUnifiedSyntaxValidator(logger zerolog.Logger) *UnifiedSyntaxValidator {
	return &UnifiedSyntaxValidator{
		logger:          logger.With().Str("component", "unified_syntax_validator").Logger(),
		dockerValidator: validators.NewDockerfileValidator(),
	}
}

// Validate performs syntax validation using the unified framework
func (v *UnifiedSyntaxValidator) Validate(content string, options ValidationOptions) (*BuildValidationResult, error) {
	v.logger.Info().
		Str("mode", "unified").
		Msg("Starting Dockerfile syntax validation with unified framework")

	// Use unified validator directly
	ctx := context.Background()
	coreOptions := ConvertToUnifiedOptions(options)
	unifiedResult := v.dockerValidator.Validate(ctx, content, coreOptions)

	// Convert to build result format
	result := ConvertToUnifiedResult(unifiedResult)

	// Apply any additional filtering if needed
	if options.Severity != "" {
		v.filterBySeverity(result, options.Severity)
	}

	if len(options.IgnoreRules) > 0 {
		v.filterByRules(result, options.IgnoreRules)
	}

	v.logger.Info().
		Bool("valid", result.Valid).
		Int("github.com/Azure/container-kit/pkg/mcp/errors", len(result.Errors)).
		Int("warnings", len(result.Warnings)).
		Msg("Syntax validation completed")

	return result, nil
}

// filterBySeverity filters results based on severity level
func (v *UnifiedSyntaxValidator) filterBySeverity(result *BuildValidationResult, severity string) {
	// For now, we'll keep all errors and warnings as the unified framework
	// already handles severity levels internally
	v.logger.Debug().
		Str("severity", severity).
		Msg("Severity filtering requested but handled by unified framework")
}

// filterByRules filters out specified rules from the results
func (v *UnifiedSyntaxValidator) filterByRules(result *BuildValidationResult, ignoreRules []string) {
	filteredErrors := make([]core.Error, 0, len(result.Errors))
	filteredWarnings := make([]core.Warning, 0, len(result.Warnings))

	// Create a map for faster lookup
	ignoreMap := make(map[string]bool)
	for _, rule := range ignoreRules {
		ignoreMap[rule] = true
	}

	// Filter errors - check if the error has a rule in the Path field or Context
	for _, err := range result.Errors {
		// Check if rule is in Path field (used for Dockerfile errors)
		shouldIgnore := false
		if err.Path != "" && ignoreMap[err.Path] {
			shouldIgnore = true
		}
		// Check if rule is in Context
		if !shouldIgnore {
			if rule, ok := err.Context["rule"]; ok && ignoreMap[rule] {
				shouldIgnore = true
			}
		}
		// Check Code field
		if !shouldIgnore && err.Code != "" && ignoreMap[err.Code] {
			shouldIgnore = true
		}

		if !shouldIgnore {
			filteredErrors = append(filteredErrors, err)
		}
	}

	// Filter warnings - similar logic
	for _, warn := range result.Warnings {
		// Check various fields where rule might be stored
		shouldIgnore := false

		// Check Path
		if warn.Path != "" && ignoreMap[warn.Path] {
			shouldIgnore = true
		}

		// Check Context
		if !shouldIgnore {
			if rule, ok := warn.Context["rule"]; ok && ignoreMap[rule] {
				shouldIgnore = true
			}
		}

		// Check Field (sometimes used for rules)
		if !shouldIgnore && warn.Field != "" && ignoreMap[warn.Field] {
			shouldIgnore = true
		}

		// Check Code
		if !shouldIgnore && warn.Code != "" && ignoreMap[warn.Code] {
			shouldIgnore = true
		}

		if !shouldIgnore {
			filteredWarnings = append(filteredWarnings, warn)
		}
	}

	result.Errors = filteredErrors
	result.Warnings = filteredWarnings

	// Update validity based on remaining errors
	result.Valid = len(result.Errors) == 0
}

// MigrateSyntaxValidatorToUnified provides a drop-in replacement for the old SyntaxValidator
// Legacy function - kept for compatibility during migration period
func MigrateSyntaxValidatorToUnified(logger zerolog.Logger) *UnifiedSyntaxValidator {
	return NewUnifiedSyntaxValidator(logger)
}

// CreateSyntaxValidatorWithUnified creates a syntax validator using the unified framework
// This function can be used as a drop-in replacement for NewSyntaxValidator
func CreateSyntaxValidatorWithUnified(logger zerolog.Logger) *UnifiedSyntaxValidator {
	logger.Info().Msg("Creating syntax validator with unified validation framework")
	return NewUnifiedSyntaxValidator(logger)
}
