package build

import (
	"regexp"
	"strings"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/rs/zerolog"
)

// ComplianceResult represents the result of a compliance check
type ComplianceResult struct {
	Framework  string                        `json:"framework"`
	Compliant  bool                          `json:"compliant"`
	Score      float64                       `json:"score"`
	Violations []SecurityComplianceViolation `json:"violations"`
}

// SecurityComplianceViolation represents a specific compliance violation
type SecurityComplianceViolation struct {
	Requirement string `json:"requirement"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Line        int    `json:"line,omitempty"`
}

// SecurityValidator handles Dockerfile security validation
type SecurityValidator struct {
	logger            zerolog.Logger
	secretPatterns    []*regexp.Regexp
	trustedRegistries []string
	policies          []SecurityPolicy
	checksProvider    SecurityChecksProvider
}

// SecurityChecksProvider interface for providing security check implementations
type SecurityChecksProvider interface {
	CheckForRootUser(lines []string, result *BuildValidationResult)
	CheckForSecrets(lines []string, result *BuildValidationResult, patterns []*regexp.Regexp)
	CheckForSensitivePorts(lines []string, result *BuildValidationResult)
	CheckPackagePinning(lines []string, result *BuildValidationResult)
	CheckForSUIDBindaries(lines []string, result *BuildValidationResult)
	CheckBaseImageSecurity(lines []string, result *BuildValidationResult, trustedRegistries []string)
	CheckForInsecureDownloads(lines []string, result *BuildValidationResult)
}

// SecurityPolicy represents a security policy that can be applied
type SecurityPolicy struct {
	Name                 string                `json:"name"`
	Description          string                `json:"description"`
	Version              string                `json:"version"`
	EnforcementLevel     string                `json:"enforcement_level"`
	Rules                []SecurityRule        `json:"rules"`
	Enabled              bool                  `json:"enabled"`
	TrustedRegistries    []string              `json:"trusted_registries"`
	ComplianceFrameworks []ComplianceFramework `json:"compliance_frameworks"`
}

// SecurityRule represents an individual security rule
type SecurityRule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	Category    string   `json:"category"`
	Check       string   `json:"check"`
	Remediation string   `json:"remediation"`
	Enabled     bool     `json:"enabled"`
	Action      string   `json:"action"`
	Patterns    []string `json:"patterns"`
}

// DetailedSecurityResult contains comprehensive security validation results
type DetailedSecurityResult struct {
	*BuildValidationResult
	ComplianceResults []ComplianceResult `json:"compliance_results,omitempty"`
	SecurityScore     float64            `json:"security_score"`
	RiskLevel         string             `json:"risk_level"`
	PolicyViolations  []PolicyViolation  `json:"policy_violations,omitempty"`
	Recommendations   []string           `json:"recommendations,omitempty"`
}

// PolicyViolation represents a violation of a security policy
type PolicyViolation struct {
	RuleID      string `json:"rule_id"`
	RuleName    string `json:"rule_name"`
	Policy      string `json:"policy"`
	Rule        string `json:"rule"`
	Severity    string `json:"severity"`
	Line        int    `json:"line,omitempty"`
	Message     string `json:"message"`
	Description string `json:"description"`
	Remediation string `json:"remediation"`
}

// ComplianceFramework defines compliance requirements (moved from policy_engine.go)
type ComplianceFramework struct {
	Name         string                  `json:"name"` // CIS, NIST, PCI-DSS, etc.
	Version      string                  `json:"version"`
	Requirements []ComplianceRequirement `json:"requirements"`
}

// ComplianceRequirement defines a specific compliance requirement
type ComplianceRequirement struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Check       string `json:"check"` // Function name or rule to check
}

// NewSecurityValidator creates a new security validator with default checks
func NewSecurityValidator(logger zerolog.Logger, trustedRegistries []string) *SecurityValidator {
	validator := &SecurityValidator{
		logger:            logger.With().Str("component", "security_validator").Logger(),
		trustedRegistries: trustedRegistries,
		secretPatterns:    compileSecretPatterns(),
		policies:          []SecurityPolicy{},
	}

	// Set default checks provider (will be implemented in security_checks.go)
	validator.checksProvider = NewDefaultSecurityChecks(logger)

	return validator
}

// SetChecksProvider allows injecting a custom security checks provider
func (v *SecurityValidator) SetChecksProvider(provider SecurityChecksProvider) {
	v.checksProvider = provider
}

// compileSecretPatterns compiles common patterns for detecting secrets
func compileSecretPatterns() []*regexp.Regexp {
	patterns := []string{
		// API keys and tokens
		`[Aa][Pp][Ii]_?[Kk][Ee][Yy]\s*[:=]\s*['"][^'"]{20,}['"]`,
		`[Tt][Oo][Kk][Ee][Nn]\s*[:=]\s*['"][^'"]{20,}['"]`,
		// Private keys
		`-----BEGIN\s+(?:RSA\s+)?PRIVATE\s+KEY-----`,
		// AWS credentials
		`AKIA[0-9A-Z]{16}`,
		`aws_access_key_id\s*[:=]\s*['"][^'"]+['"]`,
		`aws_secret_access_key\s*[:=]\s*['"][^'"]+['"]`,
		// Database URLs with credentials
		`[a-zA-Z]+://[^:]+:[^@]+@[^/]+`,
		// Generic passwords
		`[Pp][Aa][Ss][Ss][Ww][Oo][Rr][Dd]\s*[:=]\s*['"][^'"]{8,}['"]`,
		// JWT tokens
		`eyJ[A-Za-z0-9-_=]+\.eyJ[A-Za-z0-9-_=]+\.?[A-Za-z0-9-_.+/=]*`,
	}

	var compiled []*regexp.Regexp
	for _, pattern := range patterns {
		if re, err := regexp.Compile(pattern); err == nil {
			compiled = append(compiled, re)
		}
	}

	return compiled
}

// Validate performs comprehensive security validation on Dockerfile
func (v *SecurityValidator) Validate(content string, options ValidationOptions) (*BuildValidationResult, error) {
	if !options.CheckSecurity {
		v.logger.Debug().Msg("Security validation disabled")
		return &BuildValidationResult{Valid: true}, nil
	}

	v.logger.Info().Msg("Starting Dockerfile security validation")
	result := core.NewBuildResult("dockerfile-security-validator", "1.0.0")

	lines := strings.Split(content, "\n")

	// Perform various security checks using the provider
	if v.checksProvider != nil {
		v.checksProvider.CheckForRootUser(lines, result)
		v.checksProvider.CheckForSecrets(lines, result, v.secretPatterns)
		v.checksProvider.CheckForSensitivePorts(lines, result)
		v.checksProvider.CheckPackagePinning(lines, result)
		v.checksProvider.CheckForSUIDBindaries(lines, result)
		v.checksProvider.CheckBaseImageSecurity(lines, result, v.trustedRegistries)
		v.checksProvider.CheckForInsecureDownloads(lines, result)
	}

	// Update validation state
	if len(result.Errors) > 0 {
		result.Valid = false
	}

	v.logger.Info().
		Bool("valid", result.Valid).
		Int("github.com/Azure/container-kit/pkg/mcp/domain/errors", len(result.Errors)).
		Int("warnings", len(result.Warnings)).
		Msg("Security validation completed")

	return result, nil
}

// ValidateCompliance validates a Dockerfile against a specific compliance framework
func (v *SecurityValidator) ValidateCompliance(dockerfile string, framework string) *ComplianceResult {
	result := &ComplianceResult{
		Framework:  framework,
		Compliant:  true,
		Score:      100.0,
		Violations: make([]SecurityComplianceViolation, 0),
	}

	// This will be implemented by the compliance frameworks module
	v.logger.Info().
		Str("framework", framework).
		Msg("Compliance validation requested - implementation in compliance_frameworks.go")

	return result
}

// GetTrustedRegistries returns the list of trusted registries
func (v *SecurityValidator) GetTrustedRegistries() []string {
	return v.trustedRegistries
}

// AddTrustedRegistry adds a registry to the trusted list
func (v *SecurityValidator) AddTrustedRegistry(registry string) {
	v.trustedRegistries = append(v.trustedRegistries, registry)
}

// GetSecretPatterns returns the compiled secret detection patterns
func (v *SecurityValidator) GetSecretPatterns() []*regexp.Regexp {
	return v.secretPatterns
}

// AddSecretPattern adds a new secret detection pattern
func (v *SecurityValidator) AddSecretPattern(pattern string) error {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	v.secretPatterns = append(v.secretPatterns, compiled)
	return nil
}

// ValidateWithEnhancedResults performs validation and returns enhanced results
func (v *SecurityValidator) ValidateWithEnhancedResults(content string, options ValidationOptions) (*DetailedSecurityResult, error) {
	baseResult, err := v.Validate(content, options)
	if err != nil {
		return nil, err
	}

	enhancedResult := &DetailedSecurityResult{
		BuildValidationResult: baseResult,
		ComplianceResults:     []ComplianceResult{},
		SecurityScore:         v.calculateSecurityScore(baseResult),
		RiskLevel:             v.determineRiskLevel(baseResult),
		PolicyViolations:      []PolicyViolation{},
		Recommendations:       v.generateRecommendations(baseResult),
	}

	return enhancedResult, nil
}

// calculateSecurityScore calculates a security score based on validation results
func (v *SecurityValidator) calculateSecurityScore(result *BuildValidationResult) float64 {
	if result.Valid && len(result.Errors) == 0 && len(result.Warnings) == 0 {
		return 100.0
	}

	// Start with perfect score and deduct points
	score := 100.0

	// Deduct for errors (more severe)
	score -= float64(len(result.Errors)) * 15.0

	// Deduct for warnings (less severe)
	score -= float64(len(result.Warnings)) * 5.0

	// Ensure score doesn't go below 0
	if score < 0 {
		score = 0
	}

	return score
}

// determineRiskLevel determines risk level based on validation results
func (v *SecurityValidator) determineRiskLevel(result *BuildValidationResult) string {
	errorCount := len(result.Errors)
	warningCount := len(result.Warnings)

	if errorCount >= 5 {
		return "CRITICAL"
	} else if errorCount >= 3 {
		return "HIGH"
	} else if errorCount >= 1 || warningCount >= 5 {
		return "MEDIUM"
	} else if warningCount >= 1 {
		return "LOW"
	}

	return "MINIMAL"
}

// generateRecommendations generates security recommendations based on validation results
func (v *SecurityValidator) generateRecommendations(result *BuildValidationResult) []string {
	recommendations := []string{}

	if len(result.Errors) > 0 {
		recommendations = append(recommendations, "Address all security errors before deploying to production")
	}

	if len(result.Warnings) > 0 {
		recommendations = append(recommendations, "Review and address security warnings to improve security posture")
	}

	// Add specific recommendations based on common patterns
	for _, err := range result.Errors {
		switch err.Rule {
		case "root_user":
			recommendations = append(recommendations, "Create and use a non-root user in your container")
		case "exposed_secret":
			recommendations = append(recommendations, "Use secrets management solutions like Docker secrets or Kubernetes secrets")
		case "sensitive_port":
			recommendations = append(recommendations, "Avoid exposing commonly attacked ports or ensure proper security measures")
		}
	}

	return recommendations
}

// Helper methods

// isFromTrustedRegistry checks if an image is from a trusted registry
func isFromTrustedRegistry(image string, trustedRegistries []string) bool {
	if len(trustedRegistries) == 0 {
		return false // If no trusted registries defined, consider none as trusted
	}

	for _, registry := range trustedRegistries {
		if strings.HasPrefix(image, registry) {
			return true
		}
	}
	return false
}
