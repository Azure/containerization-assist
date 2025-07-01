package core

import (
	"context"
)

// Validator is the main interface that all validators must implement
type Validator interface {
	// Validate performs validation on the given data
	Validate(ctx context.Context, data interface{}, options *ValidationOptions) *ValidationResult

	// GetName returns the name of the validator
	GetName() string

	// GetVersion returns the version of the validator
	GetVersion() string

	// GetSupportedTypes returns the types this validator can handle
	GetSupportedTypes() []string
}

// FieldValidator validates individual fields
type FieldValidator interface {
	// ValidateField validates a single field value
	ValidateField(fieldName string, value interface{}, options *ValidationOptions) *ValidationError

	// GetFieldName returns the field name this validator handles
	GetFieldName() string
}

// TypedValidator provides type-safe validation
type TypedValidator[T any] interface {
	// ValidateTyped performs type-safe validation
	ValidateTyped(ctx context.Context, data T, options *ValidationOptions) *ValidationResult

	// GetName returns the name of the validator
	GetName() string
}

// ChainableValidator can be combined with other validators
type ChainableValidator interface {
	Validator

	// Chain creates a new validator that runs this validator followed by the next
	Chain(next Validator) Validator
}

// ConditionalValidator can conditionally validate based on context
type ConditionalValidator interface {
	Validator

	// ShouldValidate determines if validation should be performed
	ShouldValidate(ctx context.Context, data interface{}, options *ValidationOptions) bool
}

// SecurityValidator performs security-specific validation
type SecurityValidator interface {
	Validator

	// ValidateSecurity performs security validation
	ValidateSecurity(ctx context.Context, data interface{}, options *ValidationOptions) *SecurityValidationResult
}

// SecurityValidationResult extends ValidationResult with security-specific information
type SecurityValidationResult struct {
	*ValidationResult
	SecurityScore      float64               `json:"security_score"`      // 0-100 security score
	RiskLevel          string                `json:"risk_level"`          // low, medium, high, critical
	VulnerabilityCount map[ErrorSeverity]int `json:"vulnerability_count"` // Count by severity
	ComplianceResults  []ComplianceResult    `json:"compliance_results,omitempty"`
	ThreatAssessment   *ThreatAssessment     `json:"threat_assessment,omitempty"`
}

// ComplianceResult represents compliance validation results
type ComplianceResult struct {
	Framework  string                `json:"framework"` // CIS, NIST, PCI-DSS, etc.
	Version    string                `json:"version"`
	Compliant  bool                  `json:"compliant"`
	Score      float64               `json:"score"` // 0-100 compliance score
	Violations []ComplianceViolation `json:"violations,omitempty"`
}

// ComplianceViolation represents a compliance violation
type ComplianceViolation struct {
	Requirement string        `json:"requirement"`
	Description string        `json:"description"`
	Severity    ErrorSeverity `json:"severity"`
	Line        int           `json:"line,omitempty"`
	Remediation string        `json:"remediation,omitempty"`
}

// ThreatAssessment provides threat analysis
type ThreatAssessment struct {
	OverallRisk     string                   `json:"overall_risk"`
	Threats         []IdentifiedThreat       `json:"threats,omitempty"`
	Mitigations     []string                 `json:"mitigations,omitempty"`
	Recommendations []SecurityRecommendation `json:"recommendations,omitempty"`
}

// IdentifiedThreat represents a security threat
type IdentifiedThreat struct {
	Type        string        `json:"type"`
	Severity    ErrorSeverity `json:"severity"`
	Description string        `json:"description"`
	Impact      string        `json:"impact"`
	Likelihood  string        `json:"likelihood"`
}

// SecurityRecommendation provides security recommendations
type SecurityRecommendation struct {
	Priority    string   `json:"priority"`
	Action      string   `json:"action"`
	Description string   `json:"description"`
	References  []string `json:"references,omitempty"`
}

// ValidatorRegistry manages validator registration and discovery
type ValidatorRegistry interface {
	// Register registers a validator
	Register(name string, validator Validator) error

	// Unregister removes a validator
	Unregister(name string) error

	// Get retrieves a validator by name
	Get(name string) (Validator, bool)

	// List returns all registered validators
	List() map[string]Validator

	// GetByType returns validators that support the given type
	GetByType(dataType string) []Validator

	// Clear removes all validators
	Clear()
}

// ValidatorFactory creates validators
type ValidatorFactory interface {
	// CreateValidator creates a validator by name
	CreateValidator(name string, config map[string]interface{}) (Validator, error)

	// GetSupportedValidators returns supported validator names
	GetSupportedValidators() []string
}

// ValidatorChain represents a chain of validators
type ValidatorChain interface {
	Validator

	// Add adds a validator to the chain
	Add(validator Validator) ValidatorChain

	// AddConditional adds a conditional validator to the chain
	AddConditional(validator ConditionalValidator) ValidatorChain

	// GetValidators returns all validators in the chain
	GetValidators() []Validator

	// Clear removes all validators from the chain
	Clear()
}

// ValidationRule represents a single validation rule
type ValidationRule interface {
	// Execute executes the validation rule
	Execute(ctx context.Context, data interface{}, options *ValidationOptions) *ValidationError

	// GetName returns the rule name
	GetName() string

	// GetDescription returns the rule description
	GetDescription() string
}

// RuleEngine executes validation rules
type RuleEngine interface {
	// AddRule adds a validation rule
	AddRule(rule ValidationRule) error

	// RemoveRule removes a validation rule
	RemoveRule(name string) error

	// GetRule gets a validation rule by name
	GetRule(name string) (ValidationRule, bool)

	// ExecuteRules executes all applicable rules
	ExecuteRules(ctx context.Context, data interface{}, options *ValidationOptions) *ValidationResult

	// ExecuteRule executes a specific rule
	ExecuteRule(ctx context.Context, ruleName string, data interface{}, options *ValidationOptions) *ValidationError
}

// ValidationContext provides context for validation operations
type ValidationContext struct {
	SessionID string                 `json:"session_id,omitempty"`
	UserID    string                 `json:"user_id,omitempty"`
	Tool      string                 `json:"tool,omitempty"`
	Operation string                 `json:"operation,omitempty"`
	Stage     string                 `json:"stage,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ContextAware validators can access validation context
type ContextAware interface {
	// SetContext sets the validation context
	SetContext(ctx *ValidationContext)

	// GetContext returns the validation context
	GetContext() *ValidationContext
}

// Configurable validators can be configured
type Configurable interface {
	// Configure configures the validator with the given options
	Configure(config map[string]interface{}) error

	// GetConfiguration returns the current configuration
	GetConfiguration() map[string]interface{}
}

// Cacheable validators can cache validation results
type Cacheable interface {
	// GetCacheKey returns a cache key for the given data and options
	GetCacheKey(data interface{}, options *ValidationOptions) string

	// EnableCache enables or disables caching
	EnableCache(enabled bool)

	// ClearCache clears the validation cache
	ClearCache()
}

// StatefulValidator maintains state between validations
type StatefulValidator interface {
	Validator

	// Reset resets the validator state
	Reset()

	// GetState returns the current validator state
	GetState() map[string]interface{}
}

// AsyncValidator performs asynchronous validation
type AsyncValidator interface {
	// ValidateAsync performs asynchronous validation
	ValidateAsync(ctx context.Context, data interface{}, options *ValidationOptions) <-chan *ValidationResult
}

// BatchValidator validates multiple items in a batch
type BatchValidator interface {
	// ValidateBatch validates multiple items
	ValidateBatch(ctx context.Context, items []interface{}, options *ValidationOptions) []*ValidationResult
}

// StreamValidator validates data streams
type StreamValidator interface {
	// ValidateStream validates a data stream
	ValidateStream(ctx context.Context, stream <-chan interface{}, options *ValidationOptions) <-chan *ValidationResult
}
