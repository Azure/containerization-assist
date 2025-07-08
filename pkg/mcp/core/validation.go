// Package types provides type definitions for MCP
package core

import (
	validation "github.com/Azure/container-kit/pkg/mcp/security"
)

// Re-export validation types for backward compatibility
type ValidationResult = validation.Result
type ValidationError = validation.Error
type ValidationWarning = validation.Warning

// NewValidationResult creates a new validation result
func NewValidationResult() *ValidationResult {
	return validation.NewResult()
}

// Severity levels
const (
	SeverityLow      = validation.SeverityLow
	SeverityMedium   = validation.SeverityMedium
	SeverityHigh     = validation.SeverityHigh
	SeverityCritical = validation.SeverityCritical
)

// Build-specific validation types
type BuildValidationResult = ValidationResult
type BuildError = ValidationError
type BuildWarning = ValidationWarning

// NewBuildResult creates a new build validation result
func NewBuildResult() *BuildValidationResult {
	return NewValidationResult()
}
