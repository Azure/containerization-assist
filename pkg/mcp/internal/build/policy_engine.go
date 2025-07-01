package build

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
)

// EnhancedSecurityValidator extends SecurityValidator with policy support
type EnhancedSecurityValidator struct {
	*SecurityValidator
	policies         map[string]*SecurityPolicy
	activePolicy     *SecurityPolicy
	complianceEngine *ComplianceEngine
	vulnerabilityDB  *VulnerabilityDatabase
}

// DetailedSecurityValidationResult extends ValidationResult with security-specific information
type DetailedSecurityValidationResult struct {
	*ValidationResult
	PolicyName       string                 `json:"policy_name"`
	PolicyVersion    string                 `json:"policy_version"`
	ComplianceStatus map[string]bool        `json:"compliance_status"`
	PolicyViolations []PolicyViolation      `json:"policy_violations"`
	SecurityScore    int                    `json:"security_score"`
	RiskAssessment   SecurityRiskAssessment `json:"risk_assessment"`
}

// SecurityRiskAssessment contains risk analysis results for security validation
type SecurityRiskAssessment struct {
	OverallRisk   string   `json:"overall_risk"`
	RiskScore     int      `json:"risk_score"`
	CriticalRisks int      `json:"critical_risks"`
	HighRisks     int      `json:"high_risks"`
	MediumRisks   int      `json:"medium_risks"`
	LowRisks      int      `json:"low_risks"`
	RiskFactors   []string `json:"risk_factors"`
	Mitigations   []string `json:"mitigations"`
}

// ComplianceEngine handles compliance checking
type ComplianceEngine struct {
	logger     zerolog.Logger
	frameworks map[string]*ComplianceFramework
}

// VulnerabilityDatabase handles vulnerability checking
type VulnerabilityDatabase struct {
	logger               zerolog.Logger
	knownVulnerabilities map[string][]VulnerabilityInfo
}

// VulnerabilityInfo represents information about a known vulnerability
type VulnerabilityInfo struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	Severity    string  `json:"severity"`
	CVSS        float64 `json:"cvss"`
	Remediation string  `json:"remediation"`
}

// NewEnhancedSecurityValidator creates a new enhanced security validator
func NewEnhancedSecurityValidator(logger zerolog.Logger, trustedRegistries []string) *EnhancedSecurityValidator {
	return &EnhancedSecurityValidator{
		SecurityValidator: NewSecurityValidator(logger, trustedRegistries),
		policies:          make(map[string]*SecurityPolicy),
		complianceEngine:  NewComplianceEngine(logger),
		vulnerabilityDB:   NewVulnerabilityDatabase(logger),
	}
}

// LoadPolicy loads a security policy
func (v *EnhancedSecurityValidator) LoadPolicy(policy *SecurityPolicy) error {
	if policy.Name == "" {
		return fmt.Errorf("policy name is required")
	}
	v.policies[policy.Name] = policy
	v.logger.Info().Str("policy", policy.Name).Msg("Loaded security policy")
	return nil
}

// SetActivePolicy sets the active security policy
func (v *EnhancedSecurityValidator) SetActivePolicy(policyName string) error {
	policy, exists := v.policies[policyName]
	if !exists {
		return fmt.Errorf("policy not found: %s", policyName)
	}
	v.activePolicy = policy
	v.logger.Info().Str("policy", policyName).Msg("Set active security policy")
	return nil
}

// GetActivePolicy returns the current active policy
func (v *EnhancedSecurityValidator) GetActivePolicy() *SecurityPolicy {
	return v.activePolicy
}

// ListPolicies returns all loaded policies
func (v *EnhancedSecurityValidator) ListPolicies() []*SecurityPolicy {
	policies := make([]*SecurityPolicy, 0, len(v.policies))
	for _, policy := range v.policies {
		policies = append(policies, policy)
	}
	return policies
}

// ValidateWithPolicy performs validation with the active security policy
func (v *EnhancedSecurityValidator) ValidateWithPolicy(content string, options ValidationOptions) (*DetailedSecurityValidationResult, error) {
	if v.activePolicy == nil {
		return nil, fmt.Errorf("no active security policy set")
	}

	// Perform base validation
	baseResult, err := v.Validate(content, options)
	if err != nil {
		return nil, err
	}

	// Create enhanced result
	result := &DetailedSecurityValidationResult{
		ValidationResult: baseResult,
		PolicyName:       v.activePolicy.Name,
		PolicyVersion:    v.activePolicy.Version,
		ComplianceStatus: make(map[string]bool),
		PolicyViolations: []PolicyViolation{},
		SecurityScore:    100, // Start with perfect score
		RiskAssessment:   SecurityRiskAssessment{},
	}

	lines := strings.Split(content, "\n")

	// Apply policy rules
	v.applyPolicyRules(lines, result)

	// Check compliance frameworks
	v.checkCompliance(lines, result)

	// Assess overall risk
	v.assessRisk(result)

	// Calculate security score
	v.calculateSecurityScore(result)

	v.logger.Info().
		Str("policy", result.PolicyName).
		Int("security_score", result.SecurityScore).
		Str("overall_risk", result.RiskAssessment.OverallRisk).
		Msg("Policy validation completed")

	return result, nil
}

// applyPolicyRules applies security policy rules
func (v *EnhancedSecurityValidator) applyPolicyRules(lines []string, result *DetailedSecurityValidationResult) {
	for _, rule := range v.activePolicy.Rules {
		if !rule.Enabled {
			continue
		}

		violations := v.checkRule(lines, rule)
		for _, violation := range violations {
			result.PolicyViolations = append(result.PolicyViolations, violation)

			// Add to errors or warnings based on action
			switch rule.Action {
			case "block":
				result.Errors = append(result.Errors, ValidationError{
					Line:    violation.Line,
					Column:  0,
					Message: violation.Message,
					Rule:    rule.ID,
				})
				result.Valid = false
			case "warn":
				result.Warnings = append(result.Warnings, ValidationWarning{
					Line:    violation.Line,
					Column:  0,
					Message: violation.Message,
					Rule:    rule.ID,
				})
			}
		}
	}
}

// checkRule checks a specific security rule against lines
func (v *EnhancedSecurityValidator) checkRule(lines []string, rule SecurityRule) []PolicyViolation {
	violations := []PolicyViolation{}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check patterns
		for _, pattern := range rule.Patterns {
			if matched, err := regexp.MatchString(pattern, trimmed); err == nil && matched {
				violation := PolicyViolation{
					RuleID:      rule.ID,
					RuleName:    rule.Name,
					Severity:    rule.Severity,
					Line:        i + 1,
					Message:     fmt.Sprintf("%s: %s", rule.Name, rule.Description),
					Remediation: v.getRemediation(rule.ID),
				}
				violations = append(violations, violation)
			}
		}

		// Apply built-in rule checks
		v.applyBuiltInRuleChecks(rule, trimmed, i+1, &violations)
	}

	return violations
}

// applyBuiltInRuleChecks applies built-in security rule checks
func (v *EnhancedSecurityValidator) applyBuiltInRuleChecks(rule SecurityRule, line string, lineNum int, violations *[]PolicyViolation) {
	switch rule.ID {
	case "no-root-user":
		if strings.HasPrefix(strings.ToUpper(line), "USER") && (strings.Contains(line, "root") || strings.Contains(line, " 0")) {
			violation := PolicyViolation{
				RuleID:      rule.ID,
				RuleName:    rule.Name,
				Severity:    rule.Severity,
				Line:        lineNum,
				Message:     "Container should not run as root user",
				Remediation: v.getRemediation(rule.ID),
			}
			*violations = append(*violations, violation)
		}

	case "pin-versions":
		if (strings.Contains(line, "apt-get install") || strings.Contains(line, "pip install")) &&
			!strings.Contains(line, "=") && !strings.Contains(line, "==") {
			violation := PolicyViolation{
				RuleID:      rule.ID,
				RuleName:    rule.Name,
				Severity:    rule.Severity,
				Line:        lineNum,
				Message:     "Package versions should be pinned",
				Remediation: v.getRemediation(rule.ID),
			}
			*violations = append(*violations, violation)
		}

	case "no-latest-tag":
		if strings.HasPrefix(strings.ToUpper(line), "FROM") &&
			(strings.Contains(line, ":latest") || !strings.Contains(line, ":")) {
			violation := PolicyViolation{
				RuleID:      rule.ID,
				RuleName:    rule.Name,
				Severity:    rule.Severity,
				Line:        lineNum,
				Message:     "Base image should not use 'latest' tag",
				Remediation: v.getRemediation(rule.ID),
			}
			*violations = append(*violations, violation)
		}
	}
}

// checkCompliance checks compliance with configured frameworks
func (v *EnhancedSecurityValidator) checkCompliance(lines []string, result *DetailedSecurityValidationResult) {
	for _, framework := range v.activePolicy.ComplianceFrameworks {
		compliant := v.complianceEngine.CheckFrameworkCompliance(lines, framework)
		result.ComplianceStatus[framework.Name] = compliant

		if !compliant {
			v.logger.Warn().
				Str("framework", framework.Name).
				Str("version", framework.Version).
				Msg("Failed compliance check")
		}
	}
}

// assessRisk performs comprehensive risk assessment
func (v *EnhancedSecurityValidator) assessRisk(result *DetailedSecurityValidationResult) {
	assessment := &result.RiskAssessment

	// Count risks by severity
	for _, violation := range result.PolicyViolations {
		switch violation.Severity {
		case "critical":
			assessment.CriticalRisks++
		case "high":
			assessment.HighRisks++
		case "medium":
			assessment.MediumRisks++
		case "low":
			assessment.LowRisks++
		}
	}

	// Calculate risk score
	assessment.RiskScore = assessment.CriticalRisks*20 + assessment.HighRisks*10 + assessment.MediumRisks*5 + assessment.LowRisks*1

	// Determine overall risk level
	if assessment.CriticalRisks > 0 {
		assessment.OverallRisk = "CRITICAL"
	} else if assessment.HighRisks > 2 {
		assessment.OverallRisk = "HIGH"
	} else if assessment.HighRisks > 0 || assessment.MediumRisks > 3 {
		assessment.OverallRisk = "MEDIUM"
	} else if assessment.MediumRisks > 0 || assessment.LowRisks > 5 {
		assessment.OverallRisk = "LOW"
	} else {
		assessment.OverallRisk = "MINIMAL"
	}

	// Generate risk factors and mitigations
	assessment.RiskFactors = v.generateRiskFactors(result)
	assessment.Mitigations = v.generateMitigations(result)
}

// calculateSecurityScore calculates the overall security score
func (v *EnhancedSecurityValidator) calculateSecurityScore(result *DetailedSecurityValidationResult) {
	score := 100

	// Deduct points for policy violations
	for _, violation := range result.PolicyViolations {
		switch violation.Severity {
		case "critical":
			score -= 20
		case "high":
			score -= 10
		case "medium":
			score -= 5
		case "low":
			score -= 2
		}
	}

	// Deduct for non-compliance
	for framework, compliant := range result.ComplianceStatus {
		if !compliant {
			score -= 15
			v.logger.Warn().Str("framework", framework).Msg("Non-compliant with framework")
		}
	}

	// Ensure score doesn't go below 0
	if score < 0 {
		score = 0
	}

	result.SecurityScore = score
}

// Helper methods

// getRemediation returns remediation guidance for a specific rule
func (v *EnhancedSecurityValidator) getRemediation(ruleID string) string {
	remediations := map[string]string{
		"no-root-user":  "Add 'USER <non-root-user>' instruction to run container as non-root",
		"pin-versions":  "Pin all package versions for reproducible builds (e.g., package=1.2.3)",
		"no-secrets":    "Use secrets management solution instead of hardcoding secrets",
		"no-latest-tag": "Use specific version tags for base images (e.g., ubuntu:20.04)",
		"minimal-base":  "Use minimal base images like alpine or distroless",
		"no-privileged": "Avoid running containers in privileged mode",
	}

	if remediation, exists := remediations[ruleID]; exists {
		return remediation
	}
	return "Review and fix the security issue according to policy guidelines"
}

// getComplianceRemediation returns compliance-specific remediation
func (v *EnhancedSecurityValidator) getComplianceRemediation(framework, reqID string) string {
	return fmt.Sprintf("Implement %s requirement %s", framework, reqID)
}

// generateRiskFactors identifies key risk factors
func (v *EnhancedSecurityValidator) generateRiskFactors(result *DetailedSecurityValidationResult) []string {
	factors := []string{}

	if result.RiskAssessment.CriticalRisks > 0 {
		factors = append(factors, "Critical security vulnerabilities present")
	}
	if result.RiskAssessment.HighRisks > 0 {
		factors = append(factors, "High-severity security issues identified")
	}

	for framework, compliant := range result.ComplianceStatus {
		if !compliant {
			factors = append(factors, fmt.Sprintf("Non-compliant with %s framework", framework))
		}
	}

	if result.SecurityScore < 70 {
		factors = append(factors, "Security score below acceptable threshold")
	}

	return factors
}

// generateMitigations generates mitigation recommendations
func (v *EnhancedSecurityValidator) generateMitigations(result *DetailedSecurityValidationResult) []string {
	mitigations := []string{}

	if result.RiskAssessment.CriticalRisks > 0 {
		mitigations = append(mitigations, "Address all critical security issues immediately")
	}
	if result.RiskAssessment.HighRisks > 0 {
		mitigations = append(mitigations, "Fix high-severity security issues before deployment")
	}
	if result.SecurityScore < 70 {
		mitigations = append(mitigations, "Improve security practices to achieve minimum security score of 70")
	}

	// Add specific mitigations based on policy violations
	violationTypes := make(map[string]bool)
	for _, violation := range result.PolicyViolations {
		violationTypes[violation.RuleID] = true
	}

	if violationTypes["no-root-user"] {
		mitigations = append(mitigations, "Create and use a non-root user in your container")
	}
	if violationTypes["pin-versions"] {
		mitigations = append(mitigations, "Pin all package versions to ensure reproducible builds")
	}
	if violationTypes["no-secrets"] {
		mitigations = append(mitigations, "Implement proper secrets management")
	}

	return mitigations
}

// ComplianceEngine methods

// NewComplianceEngine creates a new compliance engine
func NewComplianceEngine(logger zerolog.Logger) *ComplianceEngine {
	return &ComplianceEngine{
		logger:     logger.With().Str("component", "compliance_engine").Logger(),
		frameworks: make(map[string]*ComplianceFramework),
	}
}

// RegisterFramework registers a compliance framework
func (ce *ComplianceEngine) RegisterFramework(framework *ComplianceFramework) {
	ce.frameworks[framework.Name] = framework
	ce.logger.Info().
		Str("framework", framework.Name).
		Str("version", framework.Version).
		Msg("Registered compliance framework")
}

// CheckFrameworkCompliance checks compliance with a specific framework
func (ce *ComplianceEngine) CheckFrameworkCompliance(lines []string, framework ComplianceFramework) bool {
	// This would implement specific compliance checks
	// For now, return true as a placeholder
	ce.logger.Debug().
		Str("framework", framework.Name).
		Int("requirements", len(framework.Requirements)).
		Msg("Checking framework compliance")

	return true
}

// GetFramework returns a registered framework by name
func (ce *ComplianceEngine) GetFramework(name string) (*ComplianceFramework, bool) {
	framework, exists := ce.frameworks[name]
	return framework, exists
}

// VulnerabilityDatabase methods

// NewVulnerabilityDatabase creates a new vulnerability database
func NewVulnerabilityDatabase(logger zerolog.Logger) *VulnerabilityDatabase {
	return &VulnerabilityDatabase{
		logger:               logger.With().Str("component", "vulnerability_db").Logger(),
		knownVulnerabilities: make(map[string][]VulnerabilityInfo),
	}
}

// CheckImageVulnerabilities checks for known vulnerabilities in base images
func (vdb *VulnerabilityDatabase) CheckImageVulnerabilities(imageName string) []VulnerabilityInfo {
	if vulns, exists := vdb.knownVulnerabilities[imageName]; exists {
		return vulns
	}
	return []VulnerabilityInfo{}
}

// AddVulnerability adds a vulnerability to the database
func (vdb *VulnerabilityDatabase) AddVulnerability(imageName string, vuln VulnerabilityInfo) {
	vdb.knownVulnerabilities[imageName] = append(vdb.knownVulnerabilities[imageName], vuln)
}

// LoadVulnerabilityData loads vulnerability data from external sources
func (vdb *VulnerabilityDatabase) LoadVulnerabilityData(data map[string][]VulnerabilityInfo) {
	vdb.knownVulnerabilities = data
	vdb.logger.Info().
		Int("images", len(data)).
		Msg("Loaded vulnerability data")
}
