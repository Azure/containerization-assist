package config

import (
	"time"
)

// ScanConfig represents typed configuration for security scanning operations
type ScanConfig struct {
	// Basic scan configuration
	Enabled    bool   `json:"enabled,omitempty"`
	ScanType   string `json:"scan_type" validate:"required"`   // vulnerability, secrets, compliance, sbom
	TargetType string `json:"target_type" validate:"required"` // image, filesystem, repository
	Target     string `json:"target" validate:"required"`

	// Scanner configuration
	Scanner ScannerConfig `json:"scanner,omitempty"`

	// Vulnerability scanning configuration
	Vulnerability VulnScanConfig `json:"vulnerability,omitempty"`

	// Secret scanning configuration
	Secrets SecretScanConfig `json:"secrets,omitempty"`

	// Compliance scanning configuration
	Compliance ComplianceScanConfig `json:"compliance,omitempty"`

	// SBOM generation configuration
	SBOM SBOMConfig `json:"sbom,omitempty"`

	// Output configuration
	Output OutputConfig `json:"output,omitempty"`

	// Filtering and severity configuration
	Severity SeverityConfig `json:"severity,omitempty"`

	// Timeout and retry configuration
	Timeout time.Duration `json:"timeout" validate:"required,min=1s"`
	Retries int           `json:"retries" validate:"min=0,max=10"`

	// Parallel processing
	Parallel bool `json:"parallel,omitempty"`
	Workers  int  `json:"workers,omitempty"`

	// Cache configuration
	Cache CacheConfig `json:"cache,omitempty"`

	// Database configuration
	Database DatabaseConfig `json:"database,omitempty"`

	// Policy configuration
	Policy PolicyConfig `json:"policy,omitempty"`

	// Notification configuration
	Notifications NotificationConfig `json:"notifications,omitempty"`

	// Metadata
	ScanID    string            `json:"scan_id,omitempty"`
	CreatedBy string            `json:"created_by,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// ScannerConfig represents scanner-specific configuration
type ScannerConfig struct {
	Name      string            `json:"name" validate:"required"` // trivy, grype, clair, snyk
	Version   string            `json:"version,omitempty"`
	Config    map[string]string `json:"config,omitempty"`
	Binary    string            `json:"binary,omitempty"`
	Arguments []string          `json:"arguments,omitempty"`
}

// VulnScanConfig represents vulnerability scanning configuration
type VulnScanConfig struct {
	Enabled         bool     `json:"enabled,omitempty"`
	DatabaseUpdate  bool     `json:"database_update,omitempty"`
	OfflineMode     bool     `json:"offline_mode,omitempty"`
	IgnoreUnfixed   bool     `json:"ignore_unfixed,omitempty"`
	VendorFirst     bool     `json:"vendor_first,omitempty"`
	ScanLayers      bool     `json:"scan_layers,omitempty"`
	SkipFiles       []string `json:"skip_files,omitempty"`
	SkipDirectories []string `json:"skip_directories,omitempty"`
	IgnorePolicy    string   `json:"ignore_policy,omitempty"`
	CVEWhitelist    []string `json:"cve_whitelist,omitempty"`
}

// SecretScanConfig represents secret scanning configuration
type SecretScanConfig struct {
	Enabled        bool            `json:"enabled,omitempty"`
	Patterns       []string        `json:"patterns,omitempty"`
	IgnorePatterns []string        `json:"ignore_patterns,omitempty"`
	MaxFileSize    int64           `json:"max_file_size,omitempty"`
	FileExtensions []string        `json:"file_extensions,omitempty"`
	ExcludeFiles   []string        `json:"exclude_files,omitempty"`
	ExcludePaths   []string        `json:"exclude_paths,omitempty"`
	DetectionRules []DetectionRule `json:"detection_rules,omitempty"`
}

// DetectionRule represents a secret detection rule
type DetectionRule struct {
	ID          string   `json:"id" validate:"required"`
	Name        string   `json:"name" validate:"required"`
	Pattern     string   `json:"pattern" validate:"required"`
	Description string   `json:"description,omitempty"`
	Severity    string   `json:"severity,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	Entropy     float64  `json:"entropy,omitempty"`
}

// ComplianceScanConfig represents compliance scanning configuration
type ComplianceScanConfig struct {
	Enabled       bool          `json:"enabled,omitempty"`
	Standards     []string      `json:"standards,omitempty"` // CIS, NIST, PCI-DSS, SOC2
	Benchmarks    []string      `json:"benchmarks,omitempty"`
	Controls      []string      `json:"controls,omitempty"`
	CustomChecks  []CustomCheck `json:"custom_checks,omitempty"`
	FailThreshold float64       `json:"fail_threshold,omitempty"`
}

// CustomCheck represents a custom compliance check
type CustomCheck struct {
	ID          string   `json:"id" validate:"required"`
	Name        string   `json:"name" validate:"required"`
	Description string   `json:"description,omitempty"`
	Command     []string `json:"command,omitempty"`
	Expected    string   `json:"expected,omitempty"`
	Severity    string   `json:"severity,omitempty"`
}

// SBOMConfig represents SBOM generation configuration
type SBOMConfig struct {
	Enabled         bool              `json:"enabled,omitempty"`
	Format          []string          `json:"format,omitempty"` // cyclonedx, spdx, syft
	IncludeDev      bool              `json:"include_dev,omitempty"`
	Licenses        bool              `json:"licenses,omitempty"`
	Vulnerabilities bool              `json:"vulnerabilities,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// OutputConfig represents output configuration for scan results
type OutputConfig struct {
	Format     []string `json:"format,omitempty"` // json, table, sarif, cyclonedx, spdx
	File       string   `json:"file,omitempty"`
	Template   string   `json:"template,omitempty"`
	Quiet      bool     `json:"quiet,omitempty"`
	Verbose    bool     `json:"verbose,omitempty"`
	Debug      bool     `json:"debug,omitempty"`
	NoProgress bool     `json:"no_progress,omitempty"`

	// Report configuration
	Report ReportConfig `json:"report,omitempty"`
}

// ReportConfig represents detailed report configuration
type ReportConfig struct {
	Title       string            `json:"title,omitempty"`
	Summary     bool              `json:"summary,omitempty"`
	Details     bool              `json:"details,omitempty"`
	Remediation bool              `json:"remediation,omitempty"`
	Charts      bool              `json:"charts,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// SeverityConfig represents severity filtering configuration
type SeverityConfig struct {
	Minimum    string             `json:"minimum,omitempty"` // UNKNOWN, LOW, MEDIUM, HIGH, CRITICAL
	Maximum    string             `json:"maximum,omitempty"`
	Include    []string           `json:"include,omitempty"`
	Exclude    []string           `json:"exclude,omitempty"`
	Thresholds SeverityThresholds `json:"thresholds,omitempty"`
}

// SeverityThresholds represents severity count thresholds
type SeverityThresholds struct {
	Critical int `json:"critical,omitempty"`
	High     int `json:"high,omitempty"`
	Medium   int `json:"medium,omitempty"`
	Low      int `json:"low,omitempty"`
	Unknown  int `json:"unknown,omitempty"`
}

// CacheConfig represents cache configuration for scan operations
type CacheConfig struct {
	Enabled     bool          `json:"enabled,omitempty"`
	Directory   string        `json:"directory,omitempty"`
	TTL         time.Duration `json:"ttl,omitempty"`
	MaxSize     int64         `json:"max_size,omitempty"`
	ClearBefore bool          `json:"clear_before,omitempty"`
	ClearAfter  bool          `json:"clear_after,omitempty"`
}

// DatabaseConfig represents vulnerability database configuration
type DatabaseConfig struct {
	AutoUpdate     bool          `json:"auto_update,omitempty"`
	UpdateInterval time.Duration `json:"update_interval,omitempty"`
	Source         []string      `json:"source,omitempty"`
	Mirror         string        `json:"mirror,omitempty"`
	OfflineMode    bool          `json:"offline_mode,omitempty"`
	CacheDirectory string        `json:"cache_directory,omitempty"`
	SkipUpdate     bool          `json:"skip_update,omitempty"`
}

// PolicyConfig represents policy enforcement configuration
type PolicyConfig struct {
	Enabled         bool         `json:"enabled,omitempty"`
	File            string       `json:"file,omitempty"`
	Rules           []PolicyRule `json:"rules,omitempty"`
	FailOnViolation bool         `json:"fail_on_violation,omitempty"`
	WarnOnViolation bool         `json:"warn_on_violation,omitempty"`
}

// PolicyRule represents a policy enforcement rule
type PolicyRule struct {
	ID        string `json:"id" validate:"required"`
	Name      string `json:"name" validate:"required"`
	Condition string `json:"condition" validate:"required"`
	Action    string `json:"action,omitempty"` // fail, warn, ignore
	Message   string `json:"message,omitempty"`
	Severity  string `json:"severity,omitempty"`
}

// NotificationConfig represents notification configuration
type NotificationConfig struct {
	Enabled   bool            `json:"enabled,omitempty"`
	Webhooks  []WebhookConfig `json:"webhooks,omitempty"`
	Email     EmailConfig     `json:"email,omitempty"`
	Slack     SlackConfig     `json:"slack,omitempty"`
	OnSuccess bool            `json:"on_success,omitempty"`
	OnFailure bool            `json:"on_failure,omitempty"`
	OnError   bool            `json:"on_error,omitempty"`
}

// WebhookConfig represents webhook notification configuration
type WebhookConfig struct {
	URL      string            `json:"url" validate:"required"`
	Method   string            `json:"method,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Template string            `json:"template,omitempty"`
	Timeout  time.Duration     `json:"timeout,omitempty"`
}

// EmailConfig represents email notification configuration
type EmailConfig struct {
	SMTP     SMTPConfig `json:"smtp,omitempty"`
	From     string     `json:"from" validate:"required"`
	To       []string   `json:"to" validate:"required"`
	Subject  string     `json:"subject,omitempty"`
	Template string     `json:"template,omitempty"`
}

// SMTPConfig represents SMTP server configuration
type SMTPConfig struct {
	Host     string `json:"host" validate:"required"`
	Port     int    `json:"port" validate:"required"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	TLS      bool   `json:"tls,omitempty"`
	StartTLS bool   `json:"start_tls,omitempty"`
}

// SlackConfig represents Slack notification configuration
type SlackConfig struct {
	WebhookURL string `json:"webhook_url" validate:"required"`
	Channel    string `json:"channel,omitempty"`
	Username   string `json:"username,omitempty"`
	IconURL    string `json:"icon_url,omitempty"`
	Template   string `json:"template,omitempty"`
}

// Validate validates the scan configuration
func (sc *ScanConfig) Validate() error {
	if sc.ScanType == "" {
		return NewValidationError("scan_type", "required field cannot be empty")
	}

	if sc.TargetType == "" {
		return NewValidationError("target_type", "required field cannot be empty")
	}

	if sc.Target == "" {
		return NewValidationError("target", "required field cannot be empty")
	}

	if sc.Scanner.Name == "" {
		return NewValidationError("scanner.name", "required field cannot be empty")
	}

	if sc.Timeout < time.Second {
		return NewValidationError("timeout", "must be at least 1 second")
	}

	if sc.Retries < 0 || sc.Retries > 10 {
		return NewValidationError("retries", "must be between 0 and 10")
	}

	if sc.Workers < 0 || sc.Workers > 50 {
		return NewValidationError("workers", "must be between 0 and 50")
	}

	return nil
}

// SetDefaults sets default values for scan configuration
func (sc *ScanConfig) SetDefaults() {
	if sc.Timeout == 0 {
		sc.Timeout = 10 * time.Minute
	}

	if sc.Retries == 0 {
		sc.Retries = 3
	}

	if sc.Workers == 0 {
		sc.Workers = 4
	}

	if sc.Scanner.Name == "" {
		sc.Scanner.Name = "trivy"
	}

	if sc.Severity.Minimum == "" {
		sc.Severity.Minimum = "MEDIUM"
	}

	if len(sc.Output.Format) == 0 {
		sc.Output.Format = []string{"json"}
	}

	// Set default cache configuration
	if sc.Cache.TTL == 0 {
		sc.Cache.TTL = 24 * time.Hour
	}

	if sc.Cache.MaxSize == 0 {
		sc.Cache.MaxSize = 1024 * 1024 * 1024 // 1GB
	}

	// Set default database update interval
	if sc.Database.UpdateInterval == 0 {
		sc.Database.UpdateInterval = 12 * time.Hour
	}
}

// IsValid checks if the configuration is valid
func (sc *ScanConfig) IsValid() bool {
	return sc.Validate() == nil
}

// IsVulnerabilityScanEnabled checks if vulnerability scanning is enabled
func (sc *ScanConfig) IsVulnerabilityScanEnabled() bool {
	return sc.Vulnerability.Enabled || sc.ScanType == "vulnerability"
}

// IsSecretScanEnabled checks if secret scanning is enabled
func (sc *ScanConfig) IsSecretScanEnabled() bool {
	return sc.Secrets.Enabled || sc.ScanType == "secrets"
}

// IsComplianceScanEnabled checks if compliance scanning is enabled
func (sc *ScanConfig) IsComplianceScanEnabled() bool {
	return sc.Compliance.Enabled || sc.ScanType == "compliance"
}

// IsSBOMGenerationEnabled checks if SBOM generation is enabled
func (sc *ScanConfig) IsSBOMGenerationEnabled() bool {
	return sc.SBOM.Enabled || sc.ScanType == "sbom"
}

// GetScannerBinary returns the scanner binary path or default
func (sc *ScanConfig) GetScannerBinary() string {
	if sc.Scanner.Binary != "" {
		return sc.Scanner.Binary
	}
	return sc.Scanner.Name // Use scanner name as binary name
}

// HasNotifications checks if notifications are enabled
func (sc *ScanConfig) HasNotifications() bool {
	return sc.Notifications.Enabled
}

// ShouldFailOnPolicy checks if scan should fail on policy violations
func (sc *ScanConfig) ShouldFailOnPolicy() bool {
	return sc.Policy.Enabled && sc.Policy.FailOnViolation
}
