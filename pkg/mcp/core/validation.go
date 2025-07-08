// Package types provides type definitions for MCP
package core

import (
	validation "github.com/Azure/container-kit/pkg/mcp/security"
)

// Re-export validation types for backward compatibility
type ValidationResult = validation.Result
type ValidationWarning = validation.Warning
type ValidationError = validation.Error
type Error = validation.Error
type Warning = validation.Warning
type ValidationMetadata = validation.Metadata

// NewValidationResult creates a new validation result
func NewValidationResult() *ValidationResult {
	return validation.NewResult()
}

// Re-export ErrorSeverity type
type ErrorSeverity = validation.ErrorSeverity

// Severity levels
const (
	SeverityLow      ErrorSeverity = validation.SeverityLow
	SeverityMedium   ErrorSeverity = validation.SeverityMedium
	SeverityHigh     ErrorSeverity = validation.SeverityHigh
	SeverityCritical ErrorSeverity = validation.SeverityCritical
)

// Build-specific validation types
type BuildValidationResult = ValidationResult
type BuildError = ValidationError
type BuildWarning = ValidationWarning

// NewBuildResult creates a new build validation result
func NewBuildResult() *BuildValidationResult {
	return NewValidationResult()
}
