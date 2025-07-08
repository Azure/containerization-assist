package build

import (
	"github.com/rs/zerolog"
)

// This file now serves as a compatibility layer and main entry point.
// The actual implementation has been split into focused modules:
// - security_types.go: Core type definitions and base validator
// - security_checks.go: Individual security check implementations
// - policy_engine.go: Policy framework and enhanced validation
// - compliance_frameworks.go: Compliance framework implementations

// NewSecurityValidator creates a new security validator with default checks
// This is the main entry point that external packages should use
func NewSecurityValidatorWithDefaults(logger zerolog.Logger, trustedRegistries []string) *SecurityValidator {
	// Create the base validator (implemented in security_types.go)
	validator := NewSecurityValidator(logger, trustedRegistries)

	// Set up default security checks provider (implemented in security_checks.go)
	validator.SetChecksProvider(NewDefaultSecurityChecks(logger))

	logger.Info().Msg("Security validator initialized with default checks")

	return validator
}

// NewEnhancedSecurityValidatorWithDefaults creates an enhanced security validator with all features
func NewEnhancedSecurityValidatorWithDefaults(logger zerolog.Logger, trustedRegistries []string) *EnhancedSecurityValidator {
	// Create the enhanced validator (implemented in policy_engine.go)
	validator := NewEnhancedSecurityValidator(logger, trustedRegistries)

	// Load default compliance frameworks (implemented in compliance_frameworks.go)
	LoadDefaultComplianceFrameworks(validator)

	logger.Info().Msg("Enhanced security validator initialized with default compliance frameworks")

	return validator
}

// Legacy compatibility functions - these delegate to the main implementations

// CreateSecurityValidator provides backward compatibility for older code
func CreateSecurityValidator(logger zerolog.Logger, trustedRegistries []string) *SecurityValidator {
	return NewSecurityValidatorWithDefaults(logger, trustedRegistries)
}

// ValidateDockerfileSecurity provides a simplified interface for basic security validation
func ValidateDockerfileSecurity(content string, logger zerolog.Logger, trustedRegistries []string) (*BuildValidationResult, error) {
	validator := NewSecurityValidatorWithDefaults(logger, trustedRegistries)
	return validator.Validate(content, ValidationOptions{CheckSecurity: true})
}

// ValidateDockerfileCompliance provides compliance validation for a specific framework
func ValidateDockerfileCompliance(content string, framework string, logger zerolog.Logger, trustedRegistries []string) *ComplianceResult {
	validator := NewSecurityValidatorWithDefaults(logger, trustedRegistries)
	return validator.ValidateCompliance(content, framework)
}

// ProcessDockerfileVulnerabilities processes vulnerability scan results
func ProcessDockerfileVulnerabilities(scanResult *VulnerabilityScanResult) *BuildValidationResult {
	return ProcessVulnerabilityScan(scanResult)
}
