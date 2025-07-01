package validation

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/validation/chains"
	"github.com/Azure/container-kit/pkg/mcp/validation/core"
	"github.com/Azure/container-kit/pkg/mcp/validation/utils"
	"github.com/Azure/container-kit/pkg/mcp/validation/validators"
)

// This file provides migration utilities and examples for moving from
// scattered validation code to the unified validation system.

// MigrationHelper helps migrate existing validation code to the unified system
type MigrationHelper struct {
	registry core.ValidatorRegistry
}

// NewMigrationHelper creates a new migration helper
func NewMigrationHelper() *MigrationHelper {
	helper := &MigrationHelper{
		registry: core.NewValidatorRegistry(),
	}

	// Register common validators
	helper.registerCommonValidators()

	return helper
}

// registerCommonValidators registers commonly used validators
func (m *MigrationHelper) registerCommonValidators() {
	// Register Docker validators
	dockerfileValidator := validators.NewDockerfileValidator()
	dockerImageValidator := validators.NewDockerImageValidator()

	core.RegisterValidator("dockerfile", dockerfileValidator)
	core.RegisterValidator("docker-image", dockerImageValidator)

	// Register base validators for testing
	core.RegisterValidator("noop", validators.NewNoOpValidator())
	core.RegisterValidator("always-pass", validators.NewAlwaysPassValidator())
}

// ConvertBuildValidationResult converts old build package ValidationResult to unified format
func ConvertBuildValidationResult(oldResult interface{}) *core.ValidationResult {
	// This function shows how to convert from the old build.ValidationResult
	// to the new unified core.ValidationResult

	result := &core.ValidationResult{
		Valid:       true,
		Errors:      make([]*core.ValidationError, 0),
		Warnings:    make([]*core.ValidationWarning, 0),
		Suggestions: make([]string, 0),
		Metadata: core.ValidationMetadata{
			ValidatorName:    "legacy-converter",
			ValidatorVersion: "1.0.0",
		},
	}

	// Type switch to handle different old result types
	switch v := oldResult.(type) {
	case map[string]interface{}:
		// Handle generic validation result
		if valid, ok := v["valid"].(bool); ok {
			result.Valid = valid
		}

		if errors, ok := v["errors"].([]interface{}); ok {
			for _, errInterface := range errors {
				if errMap, ok := errInterface.(map[string]interface{}); ok {
					err := convertLegacyError(errMap)
					if err != nil {
						result.AddError(err)
					}
				}
			}
		}

	default:
		// Unknown format - add error
		result.AddError(core.NewValidationError(
			"CONVERSION_ERROR",
			"Unknown legacy validation result format",
			core.ErrTypeValidation,
			core.SeverityMedium,
		))
	}

	return result
}

// convertLegacyError converts legacy error formats to unified ValidationError
func convertLegacyError(errData map[string]interface{}) *core.ValidationError {
	var code, message, rule string
	var line, column int

	if c, ok := errData["code"].(string); ok {
		code = c
	}
	if m, ok := errData["message"].(string); ok {
		message = m
	}
	if r, ok := errData["rule"].(string); ok {
		rule = r
	}
	if l, ok := errData["line"].(int); ok {
		line = l
	}
	if col, ok := errData["column"].(int); ok {
		column = col
	}

	if code == "" {
		code = "LEGACY_ERROR"
	}
	if message == "" {
		message = "Converted legacy validation error"
	}

	err := core.NewValidationError(code, message, core.ErrTypeValidation, core.SeverityMedium)

	if rule != "" {
		err.WithRule(rule)
	}
	if line > 0 {
		err.WithLine(line)
	}
	if column > 0 {
		err.WithColumn(column)
	}

	return err
}

// CreateDockerValidationChain creates a comprehensive Docker validation chain
func CreateDockerValidationChain() core.ValidatorChain {
	chain := chains.NewCompositeValidator("docker-comprehensive", "1.0.0")

	// Add Dockerfile validator
	dockerfileValidator := validators.NewDockerfileValidator().
		WithSecurityChecks(true).
		WithSyntaxChecks(true).
		WithBestPractices(true)

	chain.Add(dockerfileValidator)

	return chain
}

// ValidateWithLegacyCompatibility provides a compatibility layer for legacy validation calls
func ValidateWithLegacyCompatibility(validationType string, data interface{}, options map[string]interface{}) *core.ValidationResult {
	ctx := context.Background()

	// Convert legacy options to unified ValidationOptions
	unifiedOptions := convertLegacyOptions(options)

	// Get appropriate validator
	validator, exists := core.GetValidator(validationType)
	if !exists {
		result := &core.ValidationResult{
			Valid:    false,
			Errors:   make([]*core.ValidationError, 0),
			Warnings: make([]*core.ValidationWarning, 0),
		}
		result.AddError(core.NewValidationError(
			"VALIDATOR_NOT_FOUND",
			fmt.Sprintf("Validator '%s' not found", validationType),
			core.ErrTypeValidation,
			core.SeverityHigh,
		))
		return result
	}

	return validator.Validate(ctx, data, unifiedOptions)
}

// convertLegacyOptions converts legacy validation options to unified format
func convertLegacyOptions(legacy map[string]interface{}) *core.ValidationOptions {
	options := core.NewValidationOptions()

	if strictMode, ok := legacy["strict_mode"].(bool); ok {
		options.StrictMode = strictMode
	}
	if maxErrors, ok := legacy["max_errors"].(int); ok {
		options.MaxErrors = maxErrors
	}
	if skipFields, ok := legacy["skip_fields"].([]string); ok {
		options.SkipFields = skipFields
	}
	if failFast, ok := legacy["fail_fast"].(bool); ok {
		options.FailFast = failFast
	}

	return options
}

// Example migration functions showing before/after patterns

// LegacyValidateDockerfile shows the old way of validating Dockerfiles
func LegacyValidateDockerfile(content string) (bool, []string, []string) {
	// This represents how validation might have been done before
	var errors []string
	var warnings []string

	if content == "" {
		errors = append(errors, "Dockerfile cannot be empty")
		return false, errors, warnings
	}

	// Simple validation logic (scattered across multiple files)
	if err := utils.ValidateRequired(content, "dockerfile"); err != nil {
		errors = append(errors, err.Error())
	}

	valid := len(errors) == 0
	return valid, errors, warnings
}

// UnifiedValidateDockerfile shows the new unified way
func UnifiedValidateDockerfile(content string) *core.ValidationResult {
	ctx := context.Background()

	// Use the unified validator
	validator := validators.NewDockerfileValidator()
	options := core.NewValidationOptions()

	return validator.Validate(ctx, content, options)
}

// ExampleMigration demonstrates a complete migration example
func ExampleMigration() {
	dockerfileContent := `FROM ubuntu:20.04
RUN apt-get update && apt-get install -y curl
USER root
EXPOSE 8080
CMD ["./app"]`

	fmt.Println("=== Legacy Validation ===")
	valid, errors, warnings := LegacyValidateDockerfile(dockerfileContent)
	fmt.Printf("Valid: %t\n", valid)
	fmt.Printf("Errors: %v\n", errors)
	fmt.Printf("Warnings: %v\n", warnings)

	fmt.Println("\n=== Unified Validation ===")
	result := UnifiedValidateDockerfile(dockerfileContent)
	fmt.Printf("Valid: %t\n", result.Valid)
	fmt.Printf("Errors: %d\n", result.ErrorCount())
	fmt.Printf("Warnings: %d\n", result.WarningCount())
	fmt.Printf("Score: %.2f\n", result.Score)

	for _, err := range result.Errors {
		fmt.Printf("Error: %s (Line %d)\n", err.Message, err.Line)
	}

	for _, warning := range result.Warnings {
		fmt.Printf("Warning: %s\n", warning.Message)
	}

	fmt.Printf("Suggestions: %v\n", result.Suggestions)
}

// MigrationChecklist provides a checklist for teams migrating to unified validation
var MigrationChecklist = []string{
	"1. Identify all existing validation code across packages",
	"2. Catalog current validation types and interfaces",
	"3. Create unified validators for domain-specific validation",
	"4. Update import statements to use unified validation package",
	"5. Convert ValidationResult types to unified format",
	"6. Update validation option passing to use ValidationOptions",
	"7. Replace direct validation calls with validator registry",
	"8. Add proper error handling for unified ValidationError",
	"9. Update tests to use unified validation types",
	"10. Remove duplicate validation utilities",
	"11. Update documentation to reference unified validation",
	"12. Verify all validation chains work as expected",
}

// GetMigrationStatus returns migration progress for a package
func GetMigrationStatus(packagePath string) map[string]interface{} {
	// This would analyze a package and return migration status
	// Implementation would scan for old validation patterns

	return map[string]interface{}{
		"package":                 packagePath,
		"legacy_validation_found": false,
		"unified_validation_used": true,
		"migration_complete":      true,
		"recommendations":         []string{},
	}
}
