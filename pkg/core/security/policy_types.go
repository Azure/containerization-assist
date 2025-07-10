// Package security provides security policy enforcement capabilities
package security

import (
	"time"
)

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

// Policy defines a security policy rule
type Policy struct {
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

// PolicyAction defines actions to take when a policy is violated
type PolicyAction struct {
	Type        ActionType        `json:"type"`
	Parameters  map[string]string `json:"parameters,omitempty"`
	Description string            `json:"description"`
}

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

// ScanContext provides context for policy evaluation
type ScanContext struct {
	ImageRef        string                  `json:"image_ref"`
	ScanTime        time.Time               `json:"scan_time"`
	Vulnerabilities []Vulnerability         `json:"vulnerabilities"`
	VulnSummary     VulnerabilitySummary    `json:"vulnerability_summary"`
	SecretFindings  []ExtendedSecretFinding `json:"secret_findings,omitempty"`
	SecretSummary   *DiscoverySummary       `json:"secret_summary,omitempty"`
	ImageMetadata   map[string]interface{}  `json:"image_metadata,omitempty"`
	Compliance      map[string]interface{}  `json:"compliance,omitempty"`
	Packages        []PackageInfo           `json:"packages,omitempty"`
}

// PackageInfo represents information about a package in the image
type PackageInfo struct {
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	Type            string   `json:"type"`
	Licenses        []string `json:"licenses,omitempty"`
	Vulnerabilities int      `json:"vulnerabilities"`
}
