package domaintypes

import (
	"fmt"
	"time"
)

// SecurityScanParams defines strongly-typed parameters for security scanning
type SecurityScanParams struct {
	Target   string `json:"target" validate:"required"`
	ScanType string `json:"scan_type" validate:"required,oneof=image container filesystem"`

	Scanner  string   `json:"scanner,omitempty" validate:"omitempty,oneof=trivy grype"`
	Format   string   `json:"format,omitempty" validate:"omitempty,oneof=json yaml table"`
	Severity []string `json:"severity,omitempty" validate:"omitempty,dive,oneof=UNKNOWN LOW MEDIUM HIGH CRITICAL"`

	IgnoreUnfixed bool     `json:"ignore_unfixed,omitempty"`
	IgnoreFiles   []string `json:"ignore_files,omitempty"`
	PolicyPath    string   `json:"policy_path,omitempty" validate:"omitempty,file"`

	OutputPath string `json:"output_path,omitempty"`
	ExitCode   bool   `json:"exit_code,omitempty"`

	SessionID string `json:"session_id,omitempty"`

	Registry struct {
		Username string `json:"username,omitempty"`
		Password string `json:"password,omitempty"`
		Token    string `json:"token,omitempty"`
	} `json:"registry,omitempty"`
}

// Validate implements tools.ToolParams
func (p SecurityScanParams) Validate() error {
	if p.Target == "" {
		return fmt.Errorf("security-scan: target is required")
	}
	if p.ScanType == "" {
		return fmt.Errorf("security-scan: scan_type is required")
	}
	validScanTypes := map[string]bool{
		"image":      true,
		"container":  true,
		"filesystem": true,
	}
	if !validScanTypes[p.ScanType] {
		return fmt.Errorf("security-scan: scan_type must be one of: image, container, filesystem")
	}
	return nil
}

// GetSessionID implements tools.ToolParams
func (p SecurityScanParams) GetSessionID() string {
	return p.SessionID
}

// SecurityScanResult defines strongly-typed results for security scanning
type SecurityScanResult struct {
	Success bool `json:"success"`

	Target   string        `json:"target"`
	ScanType string        `json:"scan_type"`
	Scanner  string        `json:"scanner"`
	Duration time.Duration `json:"duration"`

	TotalVulnerabilities      int            `json:"total_vulnerabilities"`
	VulnerabilitiesBySeverity map[string]int `json:"vulnerabilities_by_severity"`

	Vulnerabilities []SecurityVulnerability `json:"vulnerabilities,omitempty"`

	ComplianceResults []ComplianceResult `json:"compliance_results,omitempty"`

	Secrets []DetectedSecret `json:"secrets,omitempty"`

	Licenses []LicenseInfo `json:"licenses,omitempty"`

	SessionID string `json:"session_id,omitempty"`

	RiskScore float64 `json:"risk_score,omitempty"`
	RiskLevel string  `json:"risk_level,omitempty"`

	Recommendations []string `json:"recommendations,omitempty"`
}

// IsSuccess implements tools.ToolResult
func (r SecurityScanResult) IsSuccess() bool {
	return r.Success
}

// GetDuration implements tools.ToolResult
func (r SecurityScanResult) GetDuration() time.Duration {
	return r.Duration
}

// SecurityVulnerability represents a detected security vulnerability
type SecurityVulnerability struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Severity    string  `json:"severity"`
	CVSS        float64 `json:"cvss,omitempty"`

	Package struct {
		Name           string `json:"name"`
		Version        string `json:"version"`
		FixedVersion   string `json:"fixed_version,omitempty"`
		PackageManager string `json:"package_manager,omitempty"`
	} `json:"package"`

	References []string `json:"references,omitempty"`

	Fixed bool   `json:"fixed"`
	Fix   string `json:"fix,omitempty"`
}

// ComplianceResult represents compliance check results
type ComplianceResult struct {
	Standard    string `json:"standard"`
	Control     string `json:"control"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Remediation string `json:"remediation,omitempty"`
}

// DetectedSecret represents a detected secret or sensitive information
type DetectedSecret struct {
	Type        string `json:"type"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

// LicenseInfo represents license information for dependencies
type LicenseInfo struct {
	Package string `json:"package"`
	License string `json:"license"`
	Type    string `json:"type"`
	Risk    string `json:"risk,omitempty"`
}
