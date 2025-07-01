package build

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/validation/core"
	"github.com/Azure/container-kit/pkg/mcp/validation/validators"
	"github.com/rs/zerolog"
)

// UnifiedSyntaxValidator demonstrates migration to the unified validation system
// This shows how the existing SyntaxValidator can be updated to use the new framework
type UnifiedSyntaxValidator struct {
	logger    zerolog.Logger
	validator core.Validator
}

// NewUnifiedSyntaxValidator creates a new syntax validator using unified validation
func NewUnifiedSyntaxValidator(logger zerolog.Logger) *UnifiedSyntaxValidator {
	// Create the unified Dockerfile validator
	dockerfileValidator := validators.NewDockerfileValidator().
		WithSyntaxChecks(true).
		WithSecurityChecks(false). // Syntax validator focuses only on syntax
		WithBestPractices(false)

	return &UnifiedSyntaxValidator{
		logger:    logger.With().Str("component", "unified_syntax_validator").Logger(),
		validator: dockerfileValidator,
	}
}

// Validate performs syntax validation using the unified system
func (v *UnifiedSyntaxValidator) Validate(content string, options ValidationOptions) (*ValidationResult, error) {
	v.logger.Info().
		Bool("use_hadolint", options.UseHadolint).
		Str("severity", options.Severity).
		Msg("Starting unified Dockerfile syntax validation")

	ctx := context.Background()

	// Convert legacy options to unified options
	unifiedOptions := v.convertOptions(options)

	// Perform validation using unified validator
	result := v.validator.Validate(ctx, content, unifiedOptions)

	// Convert unified result back to legacy format for compatibility
	legacyResult := v.convertToLegacyResult(result)

	v.logger.Info().
		Bool("valid", legacyResult.Valid).
		Int("errors", len(legacyResult.Errors)).
		Int("warnings", len(legacyResult.Warnings)).
		Msg("Unified syntax validation completed")

	return legacyResult, nil
}

// convertOptions converts legacy ValidationOptions to unified ValidationOptions
func (v *UnifiedSyntaxValidator) convertOptions(legacy ValidationOptions) *core.ValidationOptions {
	options := core.NewValidationOptions()

	// Map legacy options to unified options
	if legacy.Severity == "strict" {
		options.StrictMode = true
	}

	// Enable only syntax rules for syntax validator
	options.EnabledRules = []string{"syntax", "syntax-invalid-instruction", "syntax-from-missing-image"}

	// Configure based on legacy hadolint option
	if legacy.UseHadolint {
		options.WithContext("use_hadolint", true)
	}

	return options
}

// convertToLegacyResult converts unified ValidationResult to legacy ValidationResult
func (v *UnifiedSyntaxValidator) convertToLegacyResult(unified *core.ValidationResult) *ValidationResult {
	legacy := &ValidationResult{
		Valid:    unified.Valid,
		Errors:   make([]ValidationError, 0),
		Warnings: make([]ValidationWarning, 0),
		Info:     make([]string, 0),
	}

	// Convert errors
	for _, err := range unified.Errors {
		legacyError := ValidationError{
			Line:    err.Line,
			Column:  err.Column,
			Message: err.Message,
			Rule:    err.Rule,
		}
		legacy.Errors = append(legacy.Errors, legacyError)
	}

	// Convert warnings
	for _, warning := range unified.Warnings {
		legacyWarning := ValidationWarning{
			Line:    warning.Line,
			Column:  warning.Column,
			Message: warning.Message,
			Rule:    warning.Rule,
		}
		legacy.Warnings = append(legacy.Warnings, legacyWarning)
	}

	// Convert suggestions to info
	legacy.Info = append(legacy.Info, unified.Suggestions...)

	return legacy
}

// ValidateWithUnifiedChain demonstrates using validator chains
func (v *UnifiedSyntaxValidator) ValidateWithUnifiedChain(content string) *core.ValidationResult {
	ctx := context.Background()
	options := core.NewValidationOptions()

	// This shows how multiple validators can be chained together
	// For syntax validation, we might want to chain:
	// 1. Basic syntax validator
	// 2. Dockerfile-specific syntax validator
	// 3. Security syntax checks

	return v.validator.Validate(ctx, content, options)
}

// GetUnifiedValidator returns the underlying unified validator
// This allows access to the unified validation system for advanced use cases
func (v *UnifiedSyntaxValidator) GetUnifiedValidator() core.Validator {
	return v.validator
}

// Example of how to completely migrate to unified validation without legacy compatibility

// PureSyntaxValidator shows a pure unified validation approach
type PureSyntaxValidator struct {
	logger    zerolog.Logger
	validator core.Validator
}

// NewPureSyntaxValidator creates a syntax validator that only uses unified validation
func NewPureSyntaxValidator(logger zerolog.Logger) *PureSyntaxValidator {
	dockerfileValidator := validators.NewDockerfileValidator().
		WithSyntaxChecks(true).
		WithSecurityChecks(false).
		WithBestPractices(false)

	return &PureSyntaxValidator{
		logger:    logger.With().Str("component", "pure_syntax_validator").Logger(),
		validator: dockerfileValidator,
	}
}

// Validate performs validation using only the unified system
func (v *PureSyntaxValidator) Validate(ctx context.Context, content string, options *core.ValidationOptions) *core.ValidationResult {
	v.logger.Info().Msg("Starting pure unified syntax validation")

	result := v.validator.Validate(ctx, content, options)

	v.logger.Info().
		Bool("valid", result.Valid).
		Int("errors", result.ErrorCount()).
		Int("warnings", result.WarningCount()).
		Float64("score", result.Score).
		Msg("Pure unified syntax validation completed")

	return result
}

// Migration helper functions

// MigrateToUnified shows how to migrate existing validation calls
func MigrateToUnified() {
	logger := zerolog.New(nil)

	// Old way - using legacy validator
	legacyValidator := NewSyntaxValidator(logger)
	legacyOptions := ValidationOptions{UseHadolint: true, Severity: "high"}

	dockerfileContent := "FROM ubuntu:20.04\nRUN echo 'hello'"

	// Legacy validation call
	_, _ = legacyValidator.Validate(dockerfileContent, legacyOptions)

	// New way - using unified validator with compatibility layer
	unifiedValidator := NewUnifiedSyntaxValidator(logger)
	_, _ = unifiedValidator.Validate(dockerfileContent, legacyOptions)

	// Pure unified way - no legacy compatibility needed
	pureValidator := NewPureSyntaxValidator(logger)
	ctx := context.Background()
	options := core.NewValidationOptions().WithStrictMode(true)

	_ = pureValidator.Validate(ctx, dockerfileContent, options)
}
