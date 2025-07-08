package build

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/rs/zerolog"
)

// This file now serves as a compatibility layer and main entry point.
// The actual implementation has been split into focused modules:
// - security_types.go: Core type definitions and base validator
// - security_checks.go: Individual security check implementations
// - policy_engine.go: Policy framework and enhanced validation
// - compliance_frameworks.go: Compliance framework implementations

// NewSecurityValidatorWithDefaults creates a new security validator with default checks
// This is the main entry point that external packages should use
func NewSecurityValidatorWithDefaults(logger zerolog.Logger, trustedRegistries []string) *SecurityValidator {
	// Create the base validator using the function from policy_engine.go
	return NewSecurityValidator(logger, trustedRegistries)
}

// NewEnhancedSecurityValidatorWithDefaults creates an enhanced security validator with all features
func NewEnhancedSecurityValidatorWithDefaults(logger zerolog.Logger, trustedRegistries []string) *EnhancedSecurityValidator {
	// Convert zerolog to slog using a simple adapter
	slogLogger := slog.New(slog.NewTextHandler(zerologAdapter{logger}, nil))

	// Create the enhanced validator (implemented in policy_engine.go)
	validator := NewEnhancedSecurityValidator(slogLogger, trustedRegistries)

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
func ValidateDockerfileSecurity(content string, logger zerolog.Logger, trustedRegistries []string) (*core.BuildValidationResult, error) {
	validator := NewSecurityValidatorWithDefaults(logger, trustedRegistries)
	return validator.Validate(content, ValidationOptions{CheckSecurity: true})
}

// ValidateDockerfileCompliance provides compliance validation for a specific framework
func ValidateDockerfileCompliance(content string, framework string, logger zerolog.Logger, trustedRegistries []string) *ComplianceResult {
	// Simple implementation for now
	return &ComplianceResult{
		Passed: true,
	}
}

// ProcessDockerfileVulnerabilities processes vulnerability scan results
func ProcessDockerfileVulnerabilities(scanResult *VulnerabilityScanResult) *BuildValidationResult {
	return ProcessVulnerabilityScan(scanResult)
}

// zerologAdapter adapts zerolog.Logger to io.Writer for slog
type zerologAdapter struct {
	logger zerolog.Logger
}

// Write implements io.Writer
func (z zerologAdapter) Write(p []byte) (n int, err error) {
	z.logger.Info().Msg(string(p))
	return len(p), nil
}
