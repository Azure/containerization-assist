// Package security provides security policy enforcement capabilities
package security

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// PolicyEngine enforces security policies on scan results
type PolicyEngine struct {
	logger   zerolog.Logger
	policies []SecurityPolicy
}

// NewPolicyEngine creates a new policy enforcement engine
func NewPolicyEngine(logger zerolog.Logger) *PolicyEngine {
	return &PolicyEngine{
		logger:   logger.With().Str("component", "policy_engine").Logger(),
		policies: make([]SecurityPolicy, 0),
	}
}

// SecurityPolicy defines a security policy rule
type SecurityPolicy struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Enabled     bool              `json:"enabled"`
	Severity    PolicySeverity    `json:"severity"`
	Category    PolicyCategory    `json:"category"`
	Rules       []PolicyRule      `json:"rules"`
	Actions     []PolicyAction    `json:"actions"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// PolicySeverity defines the severity levels for policies
type PolicySeverity string

const (
	PolicySeverityLow      PolicySeverity = "low"
	PolicySeverityMedium   PolicySeverity = "medium"
	PolicySeverityHigh     PolicySeverity = "high"
	PolicySeverityCritical PolicySeverity = "critical"
)

// PolicyCategory defines categories of security policies
type PolicyCategory string

const (
	PolicyCategoryVulnerability PolicyCategory = "vulnerability"
	PolicyCategorySecret        PolicyCategory = "secret"
	PolicyCategoryCompliance    PolicyCategory = "compliance"
	PolicyCategoryImage         PolicyCategory = "image"
	PolicyCategoryConfiguration PolicyCategory = "configuration"
)

// PolicyRule defines a single rule within a policy
type PolicyRule struct {
	ID          string            `json:"id"`
	Type        RuleType          `json:"type"`
	Field       string            `json:"field"`
	Operator    RuleOperator      `json:"operator"`
	Value       interface{}       `json:"value"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// RuleType defines the type of policy rule
type RuleType string

const (
	RuleTypeVulnerabilityCount    RuleType = "vulnerability_count"
	RuleTypeVulnerabilitySeverity RuleType = "vulnerability_severity"
	RuleTypeCVSSScore             RuleType = "cvss_score"
	RuleTypeSecretPresence        RuleType = "secret_presence"
	RuleTypePackageVersion        RuleType = "package_version"
	RuleTypeImageAge              RuleType = "image_age"
	RuleTypeImageSize             RuleType = "image_size"
	RuleTypeLicense               RuleType = "license"
	RuleTypeCompliance            RuleType = "compliance"
)

// RuleOperator defines comparison operators for rules
type RuleOperator string

const (
	OperatorEquals             RuleOperator = "equals"
	OperatorNotEquals          RuleOperator = "not_equals"
	OperatorGreaterThan        RuleOperator = "greater_than"
	OperatorGreaterThanOrEqual RuleOperator = "greater_than_or_equal"
	OperatorLessThan           RuleOperator = "less_than"
	OperatorLessThanOrEqual    RuleOperator = "less_than_or_equal"
	OperatorContains           RuleOperator = "contains"
	OperatorNotContains        RuleOperator = "not_contains"
	OperatorMatches            RuleOperator = "matches"
	OperatorNotMatches         RuleOperator = "not_matches"
	OperatorIn                 RuleOperator = "in"
	OperatorNotIn              RuleOperator = "not_in"
)

// PolicyAction defines actions to take when a policy is violated
type PolicyAction struct {
	Type        ActionType        `json:"type"`
	Parameters  map[string]string `json:"parameters,omitempty"`
	Description string            `json:"description"`
}

// ActionType defines the type of action to take
type ActionType string

const (
	ActionTypeBlock      ActionType = "block"
	ActionTypeWarn       ActionType = "warn"
	ActionTypeLog        ActionType = "log"
	ActionTypeNotify     ActionType = "notify"
	ActionTypeQuarantine ActionType = "quarantine"
	ActionTypeAutoFix    ActionType = "auto_fix"
)

// PolicyEvaluationResult represents the result of policy evaluation
type PolicyEvaluationResult struct {
	PolicyID    string                 `json:"policy_id"`
	PolicyName  string                 `json:"policy_name"`
	Passed      bool                   `json:"passed"`
	Violations  []PolicyViolation      `json:"violations"`
	Actions     []PolicyAction         `json:"actions"`
	EvaluatedAt time.Time              `json:"evaluated_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// PolicyViolation represents a specific policy violation
type PolicyViolation struct {
	RuleID        string                 `json:"rule_id"`
	Description   string                 `json:"description"`
	Severity      PolicySeverity         `json:"severity"`
	Field         string                 `json:"field"`
	ActualValue   interface{}            `json:"actual_value"`
	ExpectedValue interface{}            `json:"expected_value"`
	Context       map[string]interface{} `json:"context,omitempty"`
}

// SecurityScanContext provides context for policy evaluation
type SecurityScanContext struct {
	ImageRef        string                 `json:"image_ref"`
	ScanTime        time.Time              `json:"scan_time"`
	Vulnerabilities []Vulnerability        `json:"vulnerabilities"`
	VulnSummary     VulnerabilitySummary   `json:"vulnerability_summary"`
	SecretFindings  []SecretFinding        `json:"secret_findings,omitempty"`
	SecretSummary   *DiscoverySummary      `json:"secret_summary,omitempty"`
	ImageMetadata   map[string]interface{} `json:"image_metadata,omitempty"`
	Compliance      map[string]interface{} `json:"compliance,omitempty"`
	Packages        []PackageInfo          `json:"packages,omitempty"`
}

// PackageInfo represents information about a package in the image
type PackageInfo struct {
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	Type            string   `json:"type"`
	Licenses        []string `json:"licenses,omitempty"`
	Vulnerabilities int      `json:"vulnerabilities"`
}

// LoadPolicies loads security policies from configuration
func (pe *PolicyEngine) LoadPolicies(policies []SecurityPolicy) error {
	pe.logger.Info().Int("count", len(policies)).Msg("Loading security policies")

	// Validate policies
	for _, policy := range policies {
		if err := pe.validatePolicy(policy); err != nil {
			return fmt.Errorf("invalid policy %s: %w", policy.ID, err)
		}
	}

	pe.policies = policies
	pe.logger.Info().Int("loaded", len(pe.policies)).Msg("Security policies loaded successfully")
	return nil
}

// LoadDefaultPolicies loads a set of default security policies
func (pe *PolicyEngine) LoadDefaultPolicies() error {
	defaultPolicies := []SecurityPolicy{
		{
			ID:          "critical-vulns-block",
			Name:        "Block Critical Vulnerabilities",
			Description: "Block images with critical vulnerabilities",
			Enabled:     true,
			Severity:    PolicySeverityCritical,
			Category:    PolicyCategoryVulnerability,
			Rules: []PolicyRule{
				{
					ID:          "critical-count",
					Type:        RuleTypeVulnerabilityCount,
					Field:       "critical",
					Operator:    OperatorGreaterThan,
					Value:       float64(0),
					Description: "No critical vulnerabilities allowed",
				},
			},
			Actions: []PolicyAction{
				{
					Type:        ActionTypeBlock,
					Description: "Block deployment due to critical vulnerabilities",
				},
				{
					Type:        ActionTypeNotify,
					Parameters:  map[string]string{"channel": "security"},
					Description: "Notify security team",
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "high-vulns-warn",
			Name:        "Warn on High Vulnerabilities",
			Description: "Warn when images have more than 5 high severity vulnerabilities",
			Enabled:     true,
			Severity:    PolicySeverityHigh,
			Category:    PolicyCategoryVulnerability,
			Rules: []PolicyRule{
				{
					ID:          "high-count-threshold",
					Type:        RuleTypeVulnerabilityCount,
					Field:       "high",
					Operator:    OperatorGreaterThan,
					Value:       float64(5),
					Description: "Warn when more than 5 high vulnerabilities found",
				},
			},
			Actions: []PolicyAction{
				{
					Type:        ActionTypeWarn,
					Description: "Warn about high vulnerability count",
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "cvss-threshold",
			Name:        "CVSS Score Threshold",
			Description: "Block images with CVSS scores above 7.0",
			Enabled:     true,
			Severity:    PolicySeverityHigh,
			Category:    PolicyCategoryVulnerability,
			Rules: []PolicyRule{
				{
					ID:          "cvss-limit",
					Type:        RuleTypeCVSSScore,
					Field:       "max_cvss_score",
					Operator:    OperatorGreaterThan,
					Value:       7.0,
					Description: "Block images with CVSS score > 7.0",
				},
			},
			Actions: []PolicyAction{
				{
					Type:        ActionTypeBlock,
					Description: "Block due to high CVSS score",
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "secrets-block",
			Name:        "Block Exposed Secrets",
			Description: "Block images containing exposed secrets",
			Enabled:     true,
			Severity:    PolicySeverityCritical,
			Category:    PolicyCategorySecret,
			Rules: []PolicyRule{
				{
					ID:          "secret-presence",
					Type:        RuleTypeSecretPresence,
					Field:       "secrets_found",
					Operator:    OperatorGreaterThan,
					Value:       float64(0),
					Description: "No secrets should be present in images",
				},
			},
			Actions: []PolicyAction{
				{
					Type:        ActionTypeBlock,
					Description: "Block deployment due to exposed secrets",
				},
				{
					Type:        ActionTypeNotify,
					Parameters:  map[string]string{"channel": "security", "priority": "urgent"},
					Description: "Urgent notification for exposed secrets",
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "outdated-packages",
			Name:        "Outdated Package Warning",
			Description: "Warn about packages that are significantly outdated",
			Enabled:     true,
			Severity:    PolicySeverityMedium,
			Category:    PolicyCategoryCompliance,
			Rules: []PolicyRule{
				{
					ID:          "package-age",
					Type:        RuleTypePackageVersion,
					Field:       "outdated_packages",
					Operator:    OperatorGreaterThan,
					Value:       float64(10),
					Description: "Warn when more than 10 packages are outdated",
				},
			},
			Actions: []PolicyAction{
				{
					Type:        ActionTypeWarn,
					Description: "Warn about outdated packages",
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	return pe.LoadPolicies(defaultPolicies)
}

// EvaluatePolicies evaluates all enabled policies against the scan context
func (pe *PolicyEngine) EvaluatePolicies(ctx context.Context, scanCtx *SecurityScanContext) ([]PolicyEvaluationResult, error) {
	pe.logger.Info().
		Str("image", scanCtx.ImageRef).
		Int("policies", len(pe.policies)).
		Msg("Evaluating security policies")

	results := make([]PolicyEvaluationResult, 0, len(pe.policies))

	for _, policy := range pe.policies {
		if !policy.Enabled {
			pe.logger.Debug().Str("policy", policy.ID).Msg("Skipping disabled policy")
			continue
		}

		result, err := pe.evaluatePolicy(ctx, policy, scanCtx)
		if err != nil {
			pe.logger.Error().
				Err(err).
				Str("policy", policy.ID).
				Msg("Failed to evaluate policy")
			continue
		}

		results = append(results, *result)
	}

	pe.logger.Info().
		Str("image", scanCtx.ImageRef).
		Int("evaluated", len(results)).
		Msg("Policy evaluation completed")

	return results, nil
}

// evaluatePolicy evaluates a single policy against the scan context
func (pe *PolicyEngine) evaluatePolicy(_ context.Context, policy SecurityPolicy, scanCtx *SecurityScanContext) (*PolicyEvaluationResult, error) {
	result := &PolicyEvaluationResult{
		PolicyID:    policy.ID,
		PolicyName:  policy.Name,
		Passed:      true,
		Violations:  make([]PolicyViolation, 0),
		Actions:     make([]PolicyAction, 0),
		EvaluatedAt: time.Now(),
		Metadata:    make(map[string]interface{}),
	}

	// Evaluate each rule in the policy
	for _, rule := range policy.Rules {
		violation, err := pe.evaluateRule(rule, scanCtx)
		if err != nil {
			pe.logger.Error().
				Err(err).
				Str("policy", policy.ID).
				Str("rule", rule.ID).
				Msg("Failed to evaluate rule")
			continue
		}

		if violation != nil {
			result.Passed = false
			violation.Severity = policy.Severity
			result.Violations = append(result.Violations, *violation)
		}
	}

	// If any violations found, add policy actions
	if !result.Passed {
		result.Actions = append(result.Actions, policy.Actions...)

		pe.logger.Warn().
			Str("policy", policy.ID).
			Int("violations", len(result.Violations)).
			Msg("Policy violations found")
	}

	return result, nil
}

// evaluateRule evaluates a single rule against the scan context
func (pe *PolicyEngine) evaluateRule(rule PolicyRule, scanCtx *SecurityScanContext) (*PolicyViolation, error) {
	switch rule.Type {
	case RuleTypeVulnerabilityCount:
		return pe.evaluateVulnerabilityCountRule(rule, scanCtx)
	case RuleTypeVulnerabilitySeverity:
		return pe.evaluateVulnerabilitySeverityRule(rule, scanCtx)
	case RuleTypeCVSSScore:
		return pe.evaluateCVSSScoreRule(rule, scanCtx)
	case RuleTypeSecretPresence:
		return pe.evaluateSecretPresenceRule(rule, scanCtx)
	case RuleTypePackageVersion:
		return pe.evaluatePackageVersionRule(rule, scanCtx)
	case RuleTypeImageAge:
		return pe.evaluateImageAgeRule(rule, scanCtx)
	case RuleTypeImageSize:
		return pe.evaluateImageSizeRule(rule, scanCtx)
	case RuleTypeLicense:
		return pe.evaluateLicenseRule(rule, scanCtx)
	default:
		return nil, fmt.Errorf("unsupported rule type: %s", rule.Type)
	}
}

// evaluateVulnerabilityCountRule evaluates vulnerability count rules
func (pe *PolicyEngine) evaluateVulnerabilityCountRule(rule PolicyRule, scanCtx *SecurityScanContext) (*PolicyViolation, error) {
	var actualValue int

	switch rule.Field {
	case "total":
		actualValue = scanCtx.VulnSummary.Total
	case "critical":
		actualValue = scanCtx.VulnSummary.Critical
	case "high":
		actualValue = scanCtx.VulnSummary.High
	case "medium":
		actualValue = scanCtx.VulnSummary.Medium
	case "low":
		actualValue = scanCtx.VulnSummary.Low
	case "fixable":
		actualValue = scanCtx.VulnSummary.Fixable
	default:
		return nil, fmt.Errorf("unsupported vulnerability count field: %s", rule.Field)
	}

	expectedValue, ok := rule.Value.(float64)
	if !ok {
		return nil, fmt.Errorf("invalid value type for vulnerability count rule")
	}

	if pe.compareValues(actualValue, rule.Operator, int(expectedValue)) {
		return &PolicyViolation{
			RuleID:        rule.ID,
			Description:   rule.Description,
			Field:         rule.Field,
			ActualValue:   actualValue,
			ExpectedValue: int(expectedValue),
			Context: map[string]interface{}{
				"vulnerability_summary": scanCtx.VulnSummary,
			},
		}, nil
	}

	return nil, nil
}

// evaluateVulnerabilitySeverityRule evaluates vulnerability severity rules
func (pe *PolicyEngine) evaluateVulnerabilitySeverityRule(rule PolicyRule, scanCtx *SecurityScanContext) (*PolicyViolation, error) {
	// Count vulnerabilities by severity
	severityCounts := map[string]int{
		"CRITICAL": scanCtx.VulnSummary.Critical,
		"HIGH":     scanCtx.VulnSummary.High,
		"MEDIUM":   scanCtx.VulnSummary.Medium,
		"LOW":      scanCtx.VulnSummary.Low,
	}

	actualValue, exists := severityCounts[strings.ToUpper(rule.Field)]
	if !exists {
		return nil, fmt.Errorf("unsupported severity field: %s", rule.Field)
	}

	expectedValue, ok := rule.Value.(float64)
	if !ok {
		return nil, fmt.Errorf("invalid value type for vulnerability severity rule")
	}

	if pe.compareValues(actualValue, rule.Operator, int(expectedValue)) {
		return &PolicyViolation{
			RuleID:        rule.ID,
			Description:   rule.Description,
			Field:         rule.Field,
			ActualValue:   actualValue,
			ExpectedValue: int(expectedValue),
			Context: map[string]interface{}{
				"severity_breakdown": severityCounts,
			},
		}, nil
	}

	return nil, nil
}

// evaluateCVSSScoreRule evaluates CVSS score rules
func (pe *PolicyEngine) evaluateCVSSScoreRule(rule PolicyRule, scanCtx *SecurityScanContext) (*PolicyViolation, error) {
	var maxScore float64
	var highScoreVulns []string

	for _, vuln := range scanCtx.Vulnerabilities {
		score := vuln.CVSS.Score
		if vuln.CVSSV3.Score > 0 {
			score = vuln.CVSSV3.Score
		}

		if score > maxScore {
			maxScore = score
		}

		if score >= 7.0 {
			highScoreVulns = append(highScoreVulns, fmt.Sprintf("%s (%.1f)", vuln.VulnerabilityID, score))
		}
	}

	expectedValue, ok := rule.Value.(float64)
	if !ok {
		return nil, fmt.Errorf("invalid value type for CVSS score rule")
	}

	if pe.compareValues(maxScore, rule.Operator, expectedValue) {
		return &PolicyViolation{
			RuleID:        rule.ID,
			Description:   rule.Description,
			Field:         rule.Field,
			ActualValue:   maxScore,
			ExpectedValue: expectedValue,
			Context: map[string]interface{}{
				"high_score_vulnerabilities": highScoreVulns,
				"vulnerability_count":        len(scanCtx.Vulnerabilities),
			},
		}, nil
	}

	return nil, nil
}

// evaluateSecretPresenceRule evaluates secret presence rules
func (pe *PolicyEngine) evaluateSecretPresenceRule(rule PolicyRule, scanCtx *SecurityScanContext) (*PolicyViolation, error) {
	secretCount := 0
	if scanCtx.SecretSummary != nil {
		secretCount = scanCtx.SecretSummary.TotalFindings - scanCtx.SecretSummary.FalsePositives
	}

	expectedValue, ok := rule.Value.(float64)
	if !ok {
		return nil, fmt.Errorf("invalid value type for secret presence rule")
	}

	if pe.compareValues(secretCount, rule.Operator, int(expectedValue)) {
		secretTypes := make(map[string]int)
		for _, finding := range scanCtx.SecretFindings {
			if !finding.FalsePositive {
				secretTypes[finding.SecretType]++
			}
		}

		return &PolicyViolation{
			RuleID:        rule.ID,
			Description:   rule.Description,
			Field:         rule.Field,
			ActualValue:   secretCount,
			ExpectedValue: int(expectedValue),
			Context: map[string]interface{}{
				"secret_types":    secretTypes,
				"total_findings":  len(scanCtx.SecretFindings),
				"false_positives": scanCtx.SecretSummary.FalsePositives,
			},
		}, nil
	}

	return nil, nil
}

// evaluatePackageVersionRule evaluates package version rules
func (pe *PolicyEngine) evaluatePackageVersionRule(rule PolicyRule, scanCtx *SecurityScanContext) (*PolicyViolation, error) {
	outdatedCount := 0
	vulnerablePackages := 0

	for _, pkg := range scanCtx.Packages {
		if pkg.Vulnerabilities > 0 {
			vulnerablePackages++
		}
		// Simple heuristic for outdated packages - those with vulnerabilities
		if pkg.Vulnerabilities > 2 {
			outdatedCount++
		}
	}

	expectedValue, ok := rule.Value.(float64)
	if !ok {
		return nil, fmt.Errorf("invalid value type for package version rule")
	}

	var actualValue int
	switch rule.Field {
	case "outdated_packages":
		actualValue = outdatedCount
	case "vulnerable_packages":
		actualValue = vulnerablePackages
	default:
		return nil, fmt.Errorf("unsupported package field: %s", rule.Field)
	}

	if pe.compareValues(actualValue, rule.Operator, int(expectedValue)) {
		return &PolicyViolation{
			RuleID:        rule.ID,
			Description:   rule.Description,
			Field:         rule.Field,
			ActualValue:   actualValue,
			ExpectedValue: int(expectedValue),
			Context: map[string]interface{}{
				"total_packages":      len(scanCtx.Packages),
				"outdated_packages":   outdatedCount,
				"vulnerable_packages": vulnerablePackages,
			},
		}, nil
	}

	return nil, nil
}

// evaluateImageAgeRule evaluates image age rules
func (pe *PolicyEngine) evaluateImageAgeRule(rule PolicyRule, scanCtx *SecurityScanContext) (*PolicyViolation, error) {
	// This would typically get image creation time from metadata
	// For now, use scan time as a placeholder
	imageAge := time.Since(scanCtx.ScanTime).Hours() / 24 // days

	expectedValue, ok := rule.Value.(float64)
	if !ok {
		return nil, fmt.Errorf("invalid value type for image age rule")
	}

	if pe.compareValues(imageAge, rule.Operator, expectedValue) {
		return &PolicyViolation{
			RuleID:        rule.ID,
			Description:   rule.Description,
			Field:         rule.Field,
			ActualValue:   imageAge,
			ExpectedValue: expectedValue,
			Context: map[string]interface{}{
				"scan_time": scanCtx.ScanTime,
			},
		}, nil
	}

	return nil, nil
}

// evaluateImageSizeRule evaluates image size rules
func (pe *PolicyEngine) evaluateImageSizeRule(rule PolicyRule, scanCtx *SecurityScanContext) (*PolicyViolation, error) {
	var imageSize float64
	if size, ok := scanCtx.ImageMetadata["image_size_mb"]; ok {
		if sizeFloat, ok := size.(float64); ok {
			imageSize = sizeFloat
		}
	}

	expectedValue, ok := rule.Value.(float64)
	if !ok {
		return nil, fmt.Errorf("invalid value type for image size rule")
	}

	if pe.compareValues(imageSize, rule.Operator, expectedValue) {
		return &PolicyViolation{
			RuleID:        rule.ID,
			Description:   rule.Description,
			Field:         rule.Field,
			ActualValue:   imageSize,
			ExpectedValue: expectedValue,
			Context: map[string]interface{}{
				"image_metadata": scanCtx.ImageMetadata,
			},
		}, nil
	}

	return nil, nil
}

// evaluateLicenseRule evaluates license rules
func (pe *PolicyEngine) evaluateLicenseRule(rule PolicyRule, scanCtx *SecurityScanContext) (*PolicyViolation, error) {
	prohibitedLicenses := make([]string, 0)
	if rule.Value != nil {
		if licenseList, ok := rule.Value.([]interface{}); ok {
			for _, license := range licenseList {
				if licenseStr, ok := license.(string); ok {
					prohibitedLicenses = append(prohibitedLicenses, licenseStr)
				}
			}
		}
	}

	foundProhibited := make([]string, 0)
	for _, pkg := range scanCtx.Packages {
		for _, license := range pkg.Licenses {
			for _, prohibited := range prohibitedLicenses {
				if strings.Contains(strings.ToLower(license), strings.ToLower(prohibited)) {
					foundProhibited = append(foundProhibited, fmt.Sprintf("%s (%s)", pkg.Name, license))
				}
			}
		}
	}

	if len(foundProhibited) > 0 {
		return &PolicyViolation{
			RuleID:        rule.ID,
			Description:   rule.Description,
			Field:         rule.Field,
			ActualValue:   foundProhibited,
			ExpectedValue: "no prohibited licenses",
			Context: map[string]interface{}{
				"prohibited_licenses": prohibitedLicenses,
				"packages_checked":    len(scanCtx.Packages),
			},
		}, nil
	}

	return nil, nil
}

// compareValues compares two values using the specified operator
func (pe *PolicyEngine) compareValues(actual interface{}, operator RuleOperator, expected interface{}) bool {
	switch operator {
	case OperatorEquals:
		return actual == expected
	case OperatorNotEquals:
		return actual != expected
	case OperatorGreaterThan:
		return pe.compareNumeric(actual, expected, func(a, b float64) bool { return a > b })
	case OperatorGreaterThanOrEqual:
		return pe.compareNumeric(actual, expected, func(a, b float64) bool { return a >= b })
	case OperatorLessThan:
		return pe.compareNumeric(actual, expected, func(a, b float64) bool { return a < b })
	case OperatorLessThanOrEqual:
		return pe.compareNumeric(actual, expected, func(a, b float64) bool { return a <= b })
	case OperatorContains:
		return pe.compareString(actual, expected, strings.Contains)
	case OperatorNotContains:
		return !pe.compareString(actual, expected, strings.Contains)
	case OperatorMatches:
		return pe.compareRegex(actual, expected)
	case OperatorNotMatches:
		return !pe.compareRegex(actual, expected)
	default:
		pe.logger.Error().Str("operator", string(operator)).Msg("Unsupported operator")
		return false
	}
}

// compareNumeric performs numeric comparison
func (pe *PolicyEngine) compareNumeric(actual, expected interface{}, compareFn func(float64, float64) bool) bool {
	actualFloat := pe.toFloat64(actual)
	expectedFloat := pe.toFloat64(expected)
	return compareFn(actualFloat, expectedFloat)
}

// compareString performs string comparison
func (pe *PolicyEngine) compareString(actual, expected interface{}, compareFn func(string, string) bool) bool {
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)
	return compareFn(actualStr, expectedStr)
}

// compareRegex performs regex comparison
func (pe *PolicyEngine) compareRegex(actual, expected interface{}) bool {
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)

	regex, err := regexp.Compile(expectedStr)
	if err != nil {
		pe.logger.Error().Err(err).Str("pattern", expectedStr).Msg("Invalid regex pattern")
		return false
	}

	return regex.MatchString(actualStr)
}

// toFloat64 converts various numeric types to float64
func (pe *PolicyEngine) toFloat64(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
}

// validatePolicy validates a security policy
func (pe *PolicyEngine) validatePolicy(policy SecurityPolicy) error {
	if policy.ID == "" {
		return fmt.Errorf("policy ID is required")
	}
	if policy.Name == "" {
		return fmt.Errorf("policy name is required")
	}
	if len(policy.Rules) == 0 {
		return fmt.Errorf("policy must have at least one rule")
	}
	if len(policy.Actions) == 0 {
		return fmt.Errorf("policy must have at least one action")
	}

	for _, rule := range policy.Rules {
		if err := pe.validateRule(rule); err != nil {
			return fmt.Errorf("invalid rule %s: %w", rule.ID, err)
		}
	}

	return nil
}

// validateRule validates a policy rule
func (pe *PolicyEngine) validateRule(rule PolicyRule) error {
	if rule.ID == "" {
		return fmt.Errorf("rule ID is required")
	}
	if rule.Type == "" {
		return fmt.Errorf("rule type is required")
	}
	if rule.Field == "" {
		return fmt.Errorf("rule field is required")
	}
	if rule.Operator == "" {
		return fmt.Errorf("rule operator is required")
	}
	if rule.Value == nil {
		return fmt.Errorf("rule value is required")
	}

	return nil
}

// GetPolicies returns all loaded policies
func (pe *PolicyEngine) GetPolicies() []SecurityPolicy {
	return pe.policies
}

// GetPolicyByID returns a policy by its ID
func (pe *PolicyEngine) GetPolicyByID(id string) (*SecurityPolicy, error) {
	for _, policy := range pe.policies {
		if policy.ID == id {
			return &policy, nil
		}
	}
	return nil, fmt.Errorf("policy not found: %s", id)
}

// AddPolicy adds a new policy to the engine
func (pe *PolicyEngine) AddPolicy(policy SecurityPolicy) error {
	if err := pe.validatePolicy(policy); err != nil {
		return err
	}

	// Check for duplicate ID
	for _, existing := range pe.policies {
		if existing.ID == policy.ID {
			return fmt.Errorf("policy with ID %s already exists", policy.ID)
		}
	}

	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	pe.policies = append(pe.policies, policy)

	pe.logger.Info().Str("policy", policy.ID).Msg("Policy added successfully")
	return nil
}

// UpdatePolicy updates an existing policy
func (pe *PolicyEngine) UpdatePolicy(policy SecurityPolicy) error {
	if err := pe.validatePolicy(policy); err != nil {
		return err
	}

	for i, existing := range pe.policies {
		if existing.ID == policy.ID {
			policy.CreatedAt = existing.CreatedAt
			policy.UpdatedAt = time.Now()
			pe.policies[i] = policy

			pe.logger.Info().Str("policy", policy.ID).Msg("Policy updated successfully")
			return nil
		}
	}

	return fmt.Errorf("policy not found: %s", policy.ID)
}

// RemovePolicy removes a policy by ID
func (pe *PolicyEngine) RemovePolicy(id string) error {
	for i, policy := range pe.policies {
		if policy.ID == id {
			pe.policies = append(pe.policies[:i], pe.policies[i+1:]...)
			pe.logger.Info().Str("policy", id).Msg("Policy removed successfully")
			return nil
		}
	}
	return fmt.Errorf("policy not found: %s", id)
}

// ShouldBlock determines if any policy evaluation results should block deployment
func (pe *PolicyEngine) ShouldBlock(results []PolicyEvaluationResult) bool {
	for _, result := range results {
		if !result.Passed {
			for _, action := range result.Actions {
				if action.Type == ActionTypeBlock {
					return true
				}
			}
		}
	}
	return false
}

// GetViolationsSummary returns a summary of all violations
func (pe *PolicyEngine) GetViolationsSummary(results []PolicyEvaluationResult) map[string]interface{} {
	summary := map[string]interface{}{
		"total_policies":    len(results),
		"passed_policies":   0,
		"failed_policies":   0,
		"total_violations":  0,
		"blocking_policies": 0,
		"severity_counts": map[string]int{
			"critical": 0,
			"high":     0,
			"medium":   0,
			"low":      0,
		},
		"action_counts": map[string]int{
			"block":  0,
			"warn":   0,
			"log":    0,
			"notify": 0,
		},
	}

	for _, result := range results {
		if result.Passed {
			summary["passed_policies"] = summary["passed_policies"].(int) + 1
		} else {
			summary["failed_policies"] = summary["failed_policies"].(int) + 1
			summary["total_violations"] = summary["total_violations"].(int) + len(result.Violations)

			// Count violations by severity
			for _, violation := range result.Violations {
				severityCounts := summary["severity_counts"].(map[string]int)
				severityCounts[string(violation.Severity)]++
			}

			// Count actions
			hasBlockingAction := false
			for _, action := range result.Actions {
				actionCounts := summary["action_counts"].(map[string]int)
				actionCounts[string(action.Type)]++

				if action.Type == ActionTypeBlock {
					hasBlockingAction = true
				}
			}

			if hasBlockingAction {
				summary["blocking_policies"] = summary["blocking_policies"].(int) + 1
			}
		}
	}

	return summary
}
