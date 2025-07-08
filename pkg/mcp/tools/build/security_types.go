package build

import (
	"time"
)

// NOTE: SecurityChecksProvider interface has been consolidated into SecurityService
// in unified_interface.go for better maintainability.
//
// Use SecurityService instead of SecurityChecksProvider for new implementations.

// The concrete implementations and utility functions remain in this file.

// SecurityPolicy represents a security policy configuration
type SecurityPolicy struct {
	Name                 string                `json:"name"`
	Version              string                `json:"version"`
	Description          string                `json:"description"`
	Rules                []SecurityRule        `json:"rules"`
	EnforcementLevel     string                `json:"enforcement_level"`
	TrustedRegistries    []string              `json:"trusted_registries"`
	ComplianceFrameworks []ComplianceFramework `json:"compliance_frameworks"`
	Metadata             map[string]string     `json:"metadata"`
	CreatedAt            time.Time             `json:"created_at"`
	UpdatedAt            time.Time             `json:"updated_at"`
}

// SecurityRule represents a security rule within a policy
type SecurityRule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	Enabled     bool     `json:"enabled"`
	Category    string   `json:"category"`
	Action      string   `json:"action"`
	Patterns    []string `json:"patterns"`
}

// PolicyViolation represents a violation of a security policy
type PolicyViolation struct {
	RuleID      string                 `json:"rule_id"`
	RuleName    string                 `json:"rule_name"`
	Severity    string                 `json:"severity"`
	Message     string                 `json:"message"`
	File        string                 `json:"file,omitempty"`
	Line        int                    `json:"line,omitempty"`
	Remediation string                 `json:"remediation,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ComplianceFramework represents a compliance framework configuration
type ComplianceFramework struct {
	Name         string                  `json:"name"`
	Version      string                  `json:"version"`
	Description  string                  `json:"description"`
	Requirements []ComplianceRequirement `json:"requirements"`
	Metadata     map[string]string       `json:"metadata"`
}

// ComplianceRequirement represents a specific compliance requirement
type ComplianceRequirement struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Mandatory   bool   `json:"mandatory"`
	Category    string `json:"category"`
	Check       string `json:"check,omitempty"`
}

// SecurityComplianceViolation represents a security compliance violation
type SecurityComplianceViolation struct {
	Requirement string `json:"requirement"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Line        int    `json:"line,omitempty"`
	Rule        string `json:"rule,omitempty"`
}
