package security

import (
	"time"
)

// VulnerabilityInfo contains information about known vulnerabilities
type VulnerabilityInfo struct {
	CVE         string    `json:"cve"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	Component   string    `json:"component"`
	Version     string    `json:"version"`
	Patched     bool      `json:"patched"`
	DetectedAt  time.Time `json:"detected_at"`
}

// ThreatModel defines the threat assessment model
type ThreatModel struct {
	Threats    map[string]ThreatInfo   `json:"threats"`
	Controls   map[string]ControlInfo  `json:"controls"`
	RiskMatrix map[string][]RiskFactor `json:"risk_matrix"`
}

// ThreatInfo describes a specific threat
type ThreatInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Impact      string   `json:"impact"`      // HIGH, MEDIUM, LOW
	Probability string   `json:"probability"` // HIGH, MEDIUM, LOW
	Category    string   `json:"category"`    // CONTAINER_ESCAPE, CODE_INJECTION, etc.
	Mitigations []string `json:"mitigations"`
}

// ControlInfo describes a security control
type ControlInfo struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Type          string   `json:"type"`          // PREVENTIVE, DETECTIVE, CORRECTIVE
	Effectiveness string   `json:"effectiveness"` // HIGH, MEDIUM, LOW
	Threats       []string `json:"threats"`       // List of threat IDs this control addresses
}

// RiskFactor defines a risk calculation factor
type RiskFactor struct {
	Factor      string  `json:"factor"`
	Weight      float64 `json:"weight"`
	Impact      string  `json:"impact"`
	Description string  `json:"description"`
}

// SecurityPolicy defines a security policy
//
//revive:disable-next-line:exported
type SecurityPolicy struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Rules       []SecurityRule         `json:"rules"`
	Actions     []SecurityAction       `json:"actions"`
	Severity    string                 `json:"severity"`
	Enabled     bool                   `json:"enabled"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// SecurityRule defines a security rule within a policy
//
//revive:disable-next-line:exported
type SecurityRule struct {
	ID          string      `json:"id"`
	Description string      `json:"description"`
	Pattern     string      `json:"pattern"`
	RuleType    string      `json:"rule_type"` // REGEX, PATH, COMMAND, etc.
	Action      string      `json:"action"`    // BLOCK, WARN, LOG
	Exceptions  []string    `json:"exceptions"`
	Metadata    interface{} `json:"metadata"`
}

// SecurityAction defines an action to take when a rule is violated
//
//revive:disable-next-line:exported
type SecurityAction struct {
	Type        string                 `json:"type"` // BLOCK, QUARANTINE, LOG, ALERT
	Parameters  map[string]interface{} `json:"parameters"`
	Description string                 `json:"description"`
}

// SecretScannerResult represents the result of a secret scan
type SecretScannerResult struct {
	Found       bool                    `json:"found"`
	Secrets     []DetectedSecret        `json:"secrets"`
	ScanSummary SecretScanSummary       `json:"scan_summary"`
	Files       map[string][]FileSecret `json:"files"`
	Metadata    map[string]interface{}  `json:"metadata"`
}

// DetectedSecret represents a detected secret
type DetectedSecret struct {
	Type        string `json:"type"`
	Value       string `json:"value,omitempty"` // Sanitized/masked value
	File        string `json:"file"`
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	Confidence  string `json:"confidence"` // HIGH, MEDIUM, LOW
	Severity    string `json:"severity"`   // CRITICAL, HIGH, MEDIUM, LOW
	Description string `json:"description"`
	Redacted    bool   `json:"redacted"`
}

// SecretScanSummary summarizes the results of a secret scan
type SecretScanSummary struct {
	TotalFiles     int                    `json:"total_files"`
	ScannedFiles   int                    `json:"scanned_files"`
	FilesScanned   int                    `json:"files_scanned"`
	SecretsFound   int                    `json:"secrets_found"`
	ByType         map[string]int         `json:"by_type"`
	BySeverity     map[string]int         `json:"by_severity"`
	PatternMatches map[string]int         `json:"pattern_matches"`
	FileTypes      map[string]int         `json:"file_types"`
	HighSeverity   int                    `json:"high_severity"`
	MediumSeverity int                    `json:"medium_severity"`
	LowSeverity    int                    `json:"low_severity"`
	ScanDuration   time.Duration          `json:"scan_duration"`
	Errors         []string               `json:"errors,omitempty"`
	Metadata       map[string]interface{} `json:"metadata"`
}

// FileSecret represents a secret found in a specific file
type FileSecret struct {
	Path        string           `json:"path"`
	Line        int              `json:"line"`
	Column      int              `json:"column"`
	Type        string           `json:"type"`
	Value       string           `json:"value"`      // Masked value
	Confidence  string           `json:"confidence"` // HIGH, MEDIUM, LOW
	Severity    string           `json:"severity"`   // CRITICAL, HIGH, MEDIUM, LOW
	Description string           `json:"description"`
	Context     string           `json:"context"` // Surrounding code context
	Secrets     []DetectedSecret `json:"secrets,omitempty"`
}

// SensitiveEnvVar represents a detected sensitive environment variable
type SensitiveEnvVar struct {
	Name          string `json:"name"`
	Value         string `json:"value"` // Masked value
	Type          string `json:"type"`
	Sensitivity   string `json:"sensitivity"`
	Pattern       string `json:"pattern"`
	Redacted      string `json:"redacted"` // Changed from bool to string to match usage
	SuggestedName string `json:"suggested_name"`
}

// SecretExternalizationPlan describes how to externalize secrets
type SecretExternalizationPlan struct {
	Manager          string                            `json:"manager"`
	Secrets          []SecretMapping                   `json:"secrets"`
	Configuration    map[string]interface{}            `json:"configuration"`
	Implementation   SecretManagerImplementationDetail `json:"implementation"`
	DetectedSecrets  []SensitiveEnvVar                 `json:"detected_secrets"`
	PreferredManager string                            `json:"preferred_manager"`
	SecretReferences map[string]SecretReference        `json:"secret_references"`
	ConfigMapEntries map[string]string                 `json:"config_map_entries"`
}

// SecretMapping maps a secret to its external reference
type SecretMapping struct {
	Original   string `json:"original"`
	External   string `json:"external"`
	Type       string `json:"type"`
	SecretPath string `json:"secret_path"`
}

// SecretManagerImplementationDetail provides implementation details for secret managers
type SecretManagerImplementationDetail struct {
	Provider     string                 `json:"provider"`
	Setup        []string               `json:"setup"`
	Dependencies []string               `json:"dependencies"`
	Examples     map[string]interface{} `json:"examples"`
}

// SecretManager describes available secret management systems
type SecretManager struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Provider     string   `json:"provider"`
	Features     []string `json:"features"`
	Supported    bool     `json:"supported"`
	RequiresAuth bool     `json:"requires_auth"`
	CloudNative  bool     `json:"cloud_native"`
	Description  string   `json:"description"`
	Example      string   `json:"example"`
}

// SanitizationPattern defines patterns for error message sanitization
type SanitizationPattern struct {
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement"`
	Type        string `json:"type"`
}

// SecretReference represents a reference to an external secret
type SecretReference struct {
	SecretName string `json:"secret_name"`
	SecretKey  string `json:"secret_key"`
	EnvVarName string `json:"env_var_name"`
}
