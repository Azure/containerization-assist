// Package scan contains pure business entities and rules for security scanning operations.
// This package has no external dependencies and represents the core security scanning domain.
package scan

import (
	"time"
)

// ScanRequest represents a request to perform security scanning
type ScanRequest struct {
	ID        string      `json:"id"`
	SessionID string      `json:"session_id"`
	Target    ScanTarget  `json:"target"`
	ScanType  ScanType    `json:"scan_type"`
	Scope     ScanScope   `json:"scope"`
	Options   ScanOptions `json:"options"`
	CreatedAt time.Time   `json:"created_at"`
}

// ScanTarget represents what is being scanned
type ScanTarget struct {
	Type       TargetType `json:"type"`
	Identifier string     `json:"identifier"`
	Repository string     `json:"repository,omitempty"`
	Tag        string     `json:"tag,omitempty"`
	Path       string     `json:"path,omitempty"`
	Manifest   string     `json:"manifest,omitempty"`
}

// TargetType represents the type of scan target
type TargetType string

const (
	TargetTypeImage      TargetType = "image"
	TargetTypeRepository TargetType = "repository"
	TargetTypeManifest   TargetType = "manifest"
	TargetTypeFilesystem TargetType = "filesystem"
	TargetTypeContainer  TargetType = "container"
)

// ScanType represents the type of security scan
type ScanType string

const (
	ScanTypeVulnerability ScanType = "vulnerability"
	ScanTypeSecret        ScanType = "secret"
	ScanTypeMalware       ScanType = "malware"
	ScanTypeCompliance    ScanType = "compliance"
	ScanTypeConfiguration ScanType = "configuration"
	ScanTypeLicense       ScanType = "license"
	ScanTypeComprehensive ScanType = "comprehensive"
)

// ScanScope defines the scope of the scan
type ScanScope struct {
	IncludeDependencies bool     `json:"include_dependencies,omitempty"`
	IncludeBaseImage    bool     `json:"include_base_image,omitempty"`
	IncludeSecrets      bool     `json:"include_secrets,omitempty"`
	IncludeCompliance   bool     `json:"include_compliance,omitempty"`
	Layers              []string `json:"layers,omitempty"`
	Paths               []string `json:"paths,omitempty"`
	Exclusions          []string `json:"exclusions,omitempty"`
}

// ScanOptions contains scanning configuration options
type ScanOptions struct {
	Scanner           Scanner           `json:"scanner,omitempty"`
	SeverityThreshold SeverityLevel     `json:"severity_threshold,omitempty"`
	Timeout           time.Duration     `json:"timeout,omitempty"`
	FailOnSeverity    SeverityLevel     `json:"fail_on_severity,omitempty"`
	OutputFormat      OutputFormat      `json:"output_format,omitempty"`
	IncludeFixed      bool              `json:"include_fixed,omitempty"`
	IncludeUnfixed    bool              `json:"include_unfixed,omitempty"`
	CustomRules       []CustomRule      `json:"custom_rules,omitempty"`
	SkipDBUpdate      bool              `json:"skip_db_update,omitempty"`
}

// Scanner represents different security scanners
type Scanner string

const (
	ScannerTrivy    Scanner = "trivy"
	ScannerGrype    Scanner = "grype"
	ScannerClair    Scanner = "clair"
	ScannerAnchore  Scanner = "anchore"
	ScannerSnyk     Scanner = "snyk"
	ScannerAquaSec  Scanner = "aquasec"
	ScannerTwistlock Scanner = "twistlock"
)

// SeverityLevel represents the severity level of issues
type SeverityLevel string

const (
	SeverityCritical SeverityLevel = "critical"
	SeverityHigh     SeverityLevel = "high"
	SeverityMedium   SeverityLevel = "medium"
	SeverityLow      SeverityLevel = "low"
	SeverityInfo     SeverityLevel = "info"
	SeverityUnknown  SeverityLevel = "unknown"
)

// OutputFormat represents the output format for scan results
type OutputFormat string

const (
	OutputFormatJSON   OutputFormat = "json"
	OutputFormatXML    OutputFormat = "xml"
	OutputFormatSARIF  OutputFormat = "sarif"
	OutputFormatTable  OutputFormat = "table"
	OutputFormatCSV    OutputFormat = "csv"
)

// CustomRule represents a custom scanning rule
type CustomRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Pattern     string            `json:"pattern"`
	Severity    SeverityLevel     `json:"severity"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ScanResult represents the result of a security scan
type ScanResult struct {
	ScanID          string              `json:"scan_id"`
	RequestID       string              `json:"request_id"`
	SessionID       string              `json:"session_id"`
	Target          ScanTarget          `json:"target"`
	ScanType        ScanType            `json:"scan_type"`
	Status          ScanStatus          `json:"status"`
	Summary         ScanSummary         `json:"summary"`
	Vulnerabilities []Vulnerability     `json:"vulnerabilities,omitempty"`
	Secrets         []Secret            `json:"secrets,omitempty"`
	Malware         []MalwareDetection  `json:"malware,omitempty"`
	Compliance      []ComplianceResult  `json:"compliance,omitempty"`
	Licenses        []LicenseIssue      `json:"licenses,omitempty"`
	Configurations  []ConfigurationIssue `json:"configurations,omitempty"`
	Error           string              `json:"error,omitempty"`
	Duration        time.Duration       `json:"duration"`
	CreatedAt       time.Time           `json:"created_at"`
	CompletedAt     *time.Time          `json:"completed_at,omitempty"`
	Metadata        ScanMetadata        `json:"metadata"`
}

// ScanStatus represents the status of a scan operation
type ScanStatus string

const (
	ScanStatusPending   ScanStatus = "pending"
	ScanStatusRunning   ScanStatus = "running"
	ScanStatusCompleted ScanStatus = "completed"
	ScanStatusFailed    ScanStatus = "failed"
	ScanStatusCancelled ScanStatus = "cancelled"
	ScanStatusTimeout   ScanStatus = "timeout"
)

// ScanSummary provides a high-level summary of scan results
type ScanSummary struct {
	TotalIssues       int                        `json:"total_issues"`
	BySeverity        map[SeverityLevel]int      `json:"by_severity"`
	ByCategory        map[string]int             `json:"by_category"`
	CriticalCount     int                        `json:"critical_count"`
	HighCount         int                        `json:"high_count"`
	MediumCount       int                        `json:"medium_count"`
	LowCount          int                        `json:"low_count"`
	FixableCount      int                        `json:"fixable_count"`
	UnfixableCount    int                        `json:"unfixable_count"`
	Score             float64                    `json:"score"`
	Grade             SecurityGrade              `json:"grade"`
	Passed            bool                       `json:"passed"`
}

// SecurityGrade represents an overall security grade
type SecurityGrade string

const (
	GradeA SecurityGrade = "A"
	GradeB SecurityGrade = "B"
	GradeC SecurityGrade = "C"
	GradeD SecurityGrade = "D"
	GradeF SecurityGrade = "F"
)

// Vulnerability represents a security vulnerability
type Vulnerability struct {
	ID              string        `json:"id"`
	CVE             string        `json:"cve,omitempty"`
	Title           string        `json:"title"`
	Description     string        `json:"description"`
	Severity        SeverityLevel `json:"severity"`
	Score           float64       `json:"score,omitempty"`
	Package         string        `json:"package"`
	Version         string        `json:"version"`
	FixedVersion    string        `json:"fixed_version,omitempty"`
	Layer           string        `json:"layer,omitempty"`
	Path            string        `json:"path,omitempty"`
	References      []string      `json:"references,omitempty"`
	Categories      []string      `json:"categories,omitempty"`
	PublishedDate   *time.Time    `json:"published_date,omitempty"`
	LastModified    *time.Time    `json:"last_modified,omitempty"`
	IsFixable       bool          `json:"is_fixable"`
	ExploitMaturity ExploitLevel  `json:"exploit_maturity,omitempty"`
}

// ExploitLevel represents the maturity level of exploits
type ExploitLevel string

const (
	ExploitLevelNone         ExploitLevel = "none"
	ExploitLevelProofOfConcept ExploitLevel = "proof_of_concept"
	ExploitLevelFunctional   ExploitLevel = "functional"
	ExploitLevelHigh         ExploitLevel = "high"
	ExploitLevelNotDefined   ExploitLevel = "not_defined"
)

// Secret represents a detected secret or credential
type Secret struct {
	ID          string        `json:"id"`
	Type        SecretType    `json:"type"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Severity    SeverityLevel `json:"severity"`
	Match       string        `json:"match"`
	File        string        `json:"file"`
	Line        int           `json:"line,omitempty"`
	Column      int           `json:"column,omitempty"`
	Rule        string        `json:"rule"`
	Tags        []string      `json:"tags,omitempty"`
	Entropy     float64       `json:"entropy,omitempty"`
	IsActive    bool          `json:"is_active,omitempty"`
}

// SecretType represents the type of secret detected
type SecretType string

const (
	SecretTypeAPIKey         SecretType = "api_key"
	SecretTypePassword       SecretType = "password"
	SecretTypeToken          SecretType = "token"
	SecretTypePrivateKey     SecretType = "private_key"
	SecretTypeCertificate    SecretType = "certificate"
	SecretTypeConnectionString SecretType = "connection_string"
	SecretTypeCredential     SecretType = "credential"
	SecretTypeGeneric        SecretType = "generic"
)

// MalwareDetection represents detected malware
type MalwareDetection struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Type        MalwareType   `json:"type"`
	Severity    SeverityLevel `json:"severity"`
	Description string        `json:"description"`
	File        string        `json:"file"`
	Hash        string        `json:"hash,omitempty"`
	Size        int64         `json:"size,omitempty"`
	Scanner     string        `json:"scanner"`
	Signature   string        `json:"signature,omitempty"`
}

// MalwareType represents the type of malware
type MalwareType string

const (
	MalwareTypeVirus    MalwareType = "virus"
	MalwareTypeTrojan   MalwareType = "trojan"
	MalwareTypeWorm     MalwareType = "worm"
	MalwareTypeRootkit  MalwareType = "rootkit"
	MalwareTypeBackdoor MalwareType = "backdoor"
	MalwareTypeSpyware  MalwareType = "spyware"
	MalwareTypeAdware   MalwareType = "adware"
	MalwareTypeGeneric  MalwareType = "generic"
)

// ComplianceResult represents compliance check results
type ComplianceResult struct {
	Standard     string             `json:"standard"`
	Version      string             `json:"version,omitempty"`
	Passed       bool               `json:"passed"`
	Score        float64            `json:"score"`
	TotalChecks  int                `json:"total_checks"`
	PassedChecks int                `json:"passed_checks"`
	FailedChecks int                `json:"failed_checks"`
	Checks       []ComplianceCheck  `json:"checks"`
}

// ComplianceCheck represents a single compliance check
type ComplianceCheck struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Category    string        `json:"category"`
	Severity    SeverityLevel `json:"severity"`
	Passed      bool          `json:"passed"`
	Required    bool          `json:"required"`
	Message     string        `json:"message,omitempty"`
	Remediation string        `json:"remediation,omitempty"`
}

// LicenseIssue represents a license-related issue
type LicenseIssue struct {
	ID          string        `json:"id"`
	Package     string        `json:"package"`
	Version     string        `json:"version"`
	License     string        `json:"license"`
	Type        LicenseType   `json:"type"`
	Severity    SeverityLevel `json:"severity"`
	Description string        `json:"description"`
	Risk        LicenseRisk   `json:"risk"`
	Category    string        `json:"category,omitempty"`
}

// LicenseType represents the type of license issue
type LicenseType string

const (
	LicenseTypeProhibited    LicenseType = "prohibited"
	LicenseTypeRestricted    LicenseType = "restricted"
	LicenseTypeUnknown       LicenseType = "unknown"
	LicenseTypeIncompatible  LicenseType = "incompatible"
	LicenseTypeConflicting   LicenseType = "conflicting"
)

// LicenseRisk represents the risk level of a license
type LicenseRisk string

const (
	LicenseRiskHigh   LicenseRisk = "high"
	LicenseRiskMedium LicenseRisk = "medium"
	LicenseRiskLow    LicenseRisk = "low"
	LicenseRiskNone   LicenseRisk = "none"
)

// ConfigurationIssue represents a configuration security issue
type ConfigurationIssue struct {
	ID          string        `json:"id"`
	Type        ConfigType    `json:"type"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Severity    SeverityLevel `json:"severity"`
	File        string        `json:"file,omitempty"`
	Line        int           `json:"line,omitempty"`
	Rule        string        `json:"rule"`
	Category    string        `json:"category"`
	Remediation string        `json:"remediation,omitempty"`
	References  []string      `json:"references,omitempty"`
}

// ConfigType represents the type of configuration issue
type ConfigType string

const (
	ConfigTypeDockerfile    ConfigType = "dockerfile"
	ConfigTypeKubernetes    ConfigType = "kubernetes"
	ConfigTypeTerraform     ConfigType = "terraform"
	ConfigTypeAnsible       ConfigType = "ansible"
	ConfigTypeCloudFormation ConfigType = "cloudformation"
	ConfigTypeGeneric       ConfigType = "generic"
)

// ScanMetadata contains additional scan information
type ScanMetadata struct {
	Scanner         Scanner           `json:"scanner"`
	ScannerVersion  string            `json:"scanner_version"`
	DatabaseVersion string            `json:"database_version,omitempty"`
	DatabaseUpdated *time.Time        `json:"database_updated,omitempty"`
	Platform        string            `json:"platform,omitempty"`
	Architecture    string            `json:"architecture,omitempty"`
	OS              string            `json:"os,omitempty"`
	Layers          []LayerInfo       `json:"layers,omitempty"`
	ScanStats       ScanStatistics    `json:"scan_stats"`
	Environment     map[string]string `json:"environment,omitempty"`
}

// LayerInfo represents information about a container layer
type LayerInfo struct {
	Digest     string    `json:"digest"`
	Command    string    `json:"command,omitempty"`
	Size       int64     `json:"size"`
	CreatedBy  string    `json:"created_by,omitempty"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
	IssueCount int       `json:"issue_count,omitempty"`
}

// ScanStatistics represents statistics about the scan operation
type ScanStatistics struct {
	FilesScanned      int           `json:"files_scanned"`
	PackagesAnalyzed  int           `json:"packages_analyzed"`
	LayersAnalyzed    int           `json:"layers_analyzed"`
	RulesEvaluated    int           `json:"rules_evaluated"`
	DatabaseSize      int64         `json:"database_size,omitempty"`
	ScanDuration      time.Duration `json:"scan_duration"`
	PreparationTime   time.Duration `json:"preparation_time,omitempty"`
	AnalysisTime      time.Duration `json:"analysis_time,omitempty"`
	ReportGeneration  time.Duration `json:"report_generation,omitempty"`
}

// ScanProgress represents the progress of an ongoing scan
type ScanProgress struct {
	ScanID         string        `json:"scan_id"`
	Status         ScanStatus    `json:"status"`
	CurrentStage   string        `json:"current_stage"`
	StageNumber    int           `json:"stage_number"`
	TotalStages    int           `json:"total_stages"`
	Percentage     float64       `json:"percentage"`
	ElapsedTime    time.Duration `json:"elapsed_time"`
	EstimatedTime  *time.Duration `json:"estimated_time,omitempty"`
	LastUpdate     time.Time     `json:"last_update"`
	Message        string        `json:"message,omitempty"`
}

// ScanPolicy represents a security scanning policy
type ScanPolicy struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	Version         string                 `json:"version"`
	Enabled         bool                   `json:"enabled"`
	SeverityLimits  map[SeverityLevel]int  `json:"severity_limits"`
	FailureThreshold float64               `json:"failure_threshold"`
	RequiredScans    []ScanType            `json:"required_scans"`
	Exemptions       []Exemption           `json:"exemptions,omitempty"`
	Compliance       []string              `json:"compliance,omitempty"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

// Exemption represents an exemption from security policies
type Exemption struct {
	ID          string        `json:"id"`
	Type        ExemptionType `json:"type"`
	Pattern     string        `json:"pattern"`
	Reason      string        `json:"reason"`
	Expiry      *time.Time    `json:"expiry,omitempty"`
	Approved    bool          `json:"approved"`
	ApprovedBy  string        `json:"approved_by,omitempty"`
	ApprovedAt  *time.Time    `json:"approved_at,omitempty"`
}

// ExemptionType represents the type of exemption
type ExemptionType string

const (
	ExemptionTypeCVE         ExemptionType = "cve"
	ExemptionTypePackage     ExemptionType = "package"
	ExemptionTypeFile        ExemptionType = "file"
	ExemptionTypePath        ExemptionType = "path"
	ExemptionTypeRule        ExemptionType = "rule"
	ExemptionTypeLicense     ExemptionType = "license"
)