package core

// Validation and scanning-related types
// These are primarily used by validation and security scanning operations

// ValidateResult represents the result of a validation operation
// TODO: Migrate to pkg/mcp/validation/core.ValidationResult
type ValidateResult struct {
	BaseToolResponse
	Valid       bool     `json:"valid"`
	Score       float64  `json:"score"`
	Suggestions []string `json:"suggestions"`
}

// ScanParams represents parameters for security scan operations
type ScanParams struct {
	SessionID      string `json:"session_id"`
	ImageRef       string `json:"image_ref"`
	ScanType       string `json:"scan_type"`
	OutputFile     string `json:"output_file,omitempty"`
	SeverityFilter string `json:"severity_filter,omitempty"`
}

// SecretFinding represents a detected secret
type SecretFinding struct {
	Type        string `json:"type"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Description string `json:"description"`
	Confidence  string `json:"confidence"`
	RuleID      string `json:"rule_id"`
}
