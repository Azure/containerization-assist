// Package validation provides a unified validation framework for the Container Kit project.
//
// This package consolidates validation utilities that were previously scattered across
// multiple packages, providing a consistent interface and reducing code duplication.
//
// # Key Features
//
//   - Unified validation types and interfaces
//   - Composable validator chains
//   - Rich error context with suggestions
//   - Built-in validators for common use cases
//   - Type-safe validation with generics
//   - Async and batch validation support
//   - Comprehensive validation registry
//
// # Architecture
//
// The validation package is organized into several sub-packages:
//
//   - core: Core types, interfaces, and registry
//   - utils: Common validation utilities (strings, paths, formats)
//   - validators: Domain-specific validators (Docker, Kubernetes, etc.)
//   - chains: Validator composition and chaining utilities
//
// # Basic Usage
//
//	import (
//		"context"
//		"github.com/Azure/container-kit/pkg/mcp/validation/core"
//		"github.com/Azure/container-kit/pkg/mcp/validation/validators"
//	)
//
//	func validateDockerfile(content string) *core.ValidationResult {
//		ctx := context.Background()
//		validator := validators.NewDockerfileValidator()
//		options := core.NewValidationOptions()
//
//		return validator.Validate(ctx, content, options)
//	}
//
// # Validator Chains
//
// You can chain multiple validators together for comprehensive validation:
//
//	import "github.com/Azure/container-kit/pkg/mcp/validation/chains"
//
//	func createDockerChain() core.ValidatorChain {
//		chain := chains.NewCompositeValidator("docker-full", "1.0.0")
//
//		chain.Add(validators.NewDockerfileValidator())
//		chain.Add(validators.NewDockerImageValidator())
//
//		return chain
//	}
//
// # Registry Usage
//
// Validators can be registered globally and retrieved by name:
//
//	// Register a validator
//	core.RegisterValidator("dockerfile", validators.NewDockerfileValidator())
//
//	// Retrieve and use a validator
//	if validator, exists := core.GetValidator("dockerfile"); exists {
//		result := validator.Validate(ctx, data, options)
//	}
//
// # Validation Options
//
// ValidationOptions provide fine-grained control over validation behavior:
//
//	options := core.NewValidationOptions().
//		WithStrictMode(true).
//		WithMaxErrors(10).
//		WithFailFast(false).
//		WithTimeout(30 * time.Second)
//
// # Error Handling
//
// The unified ValidationResult provides rich error context:
//
//	result := validator.Validate(ctx, data, options)
//
//	if !result.Valid {
//		for _, err := range result.Errors {
//			fmt.Printf("Error: %s (Line %d, Rule: %s)\n",
//				err.Message, err.Line, err.Rule)
//
//			for _, suggestion := range err.Suggestions {
//				fmt.Printf("  Suggestion: %s\n", suggestion)
//			}
//		}
//	}
//
// # Custom Validators
//
// You can create custom validators by implementing the Validator interface:
//
//	type MyValidator struct {
//		*validators.BaseValidatorImpl
//	}
//
//	func NewMyValidator() *MyValidator {
//		return &MyValidator{
//			BaseValidatorImpl: validators.NewBaseValidator("my-validator", "1.0.0", []string{"string"}),
//		}
//	}
//
//	func (m *MyValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.ValidationResult {
//		// Custom validation logic here
//		result := m.BaseValidatorImpl.Validate(ctx, data, options)
//
//		// Add your validation logic
//		if someCondition {
//			result.AddError(core.NewValidationError("MY_ERROR", "Something is wrong", core.ErrTypeValidation, core.SeverityMedium))
//		}
//
//		return result
//	}
//
// # Migration from Legacy Validation
//
// The package includes migration utilities to help transition from scattered
// validation code to the unified system. See migration.go for examples and helpers.
//
// # Performance Considerations
//
//   - Validators can be cached and reused across multiple validations
//   - Use parallel validation chains for independent validations
//   - Consider async validation for expensive operations
//   - Batch validation is available for validating multiple items
//
// # Extensibility
//
// The framework is designed to be extensible:
//
//   - Implement Validator for basic validation
//   - Implement ConditionalValidator for context-dependent validation
//   - Implement SecurityValidator for security-specific validation
//   - Implement AsyncValidator for asynchronous validation
//   - Implement BatchValidator for batch operations
//
// # Security Considerations
//
// When implementing custom validators:
//
//   - Validate input data types and bounds
//   - Avoid exposing sensitive information in error messages
//   - Use secure defaults for validation options
//   - Consider timeout and resource limits for expensive validations
//   - Sanitize user input before validation
//
// # Best Practices
//
//   - Use specific error codes and messages
//   - Provide actionable suggestions for fixing errors
//   - Chain validators logically (syntax -> security -> best practices)
//   - Use appropriate severity levels for different issues
//   - Include context information in validation errors
//   - Register validators once at application startup
//   - Use validation options to customize behavior per use case
//   - Prefer composition over inheritance for complex validators
//
// # Examples
//
// See the examples in the migration.go file for complete usage examples
// and patterns for migrating from legacy validation code.
package validation
