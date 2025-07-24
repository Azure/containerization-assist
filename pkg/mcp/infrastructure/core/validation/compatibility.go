// Package validation provides compatibility aliases for existing validation types
package validation

// Compatibility aliases for existing validation result types.
// These allow existing code to continue working while gradually migrating
// to the unified validation framework.

// Legacy type aliases - these should be gradually phased out
type (
	// LegacyValidationResult provides backward compatibility
	LegacyValidationResult = ValidationResult

	// LegacyValidationError provides backward compatibility
	LegacyValidationError = ValidationError

	// LegacyValidationWarning provides backward compatibility
	LegacyValidationWarning = ValidationWarning
)

// Factory functions for backward compatibility
func NewLegacyValidationResult() *ValidationResult {
	return NewValidationResult()
}

func NewLegacyBuildValidationResult() *BuildValidationResult {
	return NewBuildValidationResult()
}

func NewLegacyManifestValidationResult() *ManifestValidationResult {
	return NewManifestValidationResult()
}
