// Package scan contains business rules for security scanning operations
package scan

import (
	"fmt"
	"time"
)

// ValidationError represents a scan validation error
type ValidationError struct {
	Field   string
	Message string
	Code    string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("scan validation error: %s - %s", e.Field, e.Message)
}

// Validate performs domain-level validation on a scan request
func (sr *ScanRequest) Validate() []ValidationError {
	var errors []ValidationError

	// Session ID is required
	if sr.SessionID == "" {
		errors = append(errors, ValidationError{
			Field:   "session_id",
			Message: "session ID is required",
			Code:    "MISSING_SESSION_ID",
		})
	}

	// Target identifier is required
	if sr.Target.Identifier == "" {
		errors = append(errors, ValidationError{
			Field:   "target.identifier",
			Message: "target identifier is required",
			Code:    "MISSING_TARGET_IDENTIFIER",
		})
	}

	// Target type must be valid
	if !isValidTargetType(sr.Target.Type) {
		errors = append(errors, ValidationError{
			Field:   "target.type",
			Message: "invalid target type",
			Code:    "INVALID_TARGET_TYPE",
		})
	}

	// Scan type must be valid
	if !isValidScanType(sr.ScanType) {
		errors = append(errors, ValidationError{
			Field:   "scan_type",
			Message: "invalid scan type",
			Code:    "INVALID_SCAN_TYPE",
		})
	}

	// Validate timeout
	if sr.Options.Timeout < 0 {
		errors = append(errors, ValidationError{
			Field:   "options.timeout",
			Message: "timeout cannot be negative",
			Code:    "INVALID_TIMEOUT",
		})
	}

	return errors
}

// Business Rules for Scan Operations

// IsCompleted returns true if the scan has completed
func (sr *ScanResult) IsCompleted() bool {
	return sr.Status == ScanStatusCompleted ||
		sr.Status == ScanStatusFailed ||
		sr.Status == ScanStatusCancelled ||
		sr.Status == ScanStatusTimeout
}

// IsSuccessful returns true if the scan completed successfully
func (sr *ScanResult) IsSuccessful() bool {
	return sr.Status == ScanStatusCompleted
}

// HasCriticalIssues returns true if the scan found critical security issues
func (sr *ScanResult) HasCriticalIssues() bool {
	return sr.Summary.CriticalCount > 0
}

// PassesPolicy returns true if the scan results pass the given policy
func (sr *ScanResult) PassesPolicy(policy *ScanPolicy) bool {
	// Check severity limits
	for severity, limit := range policy.SeverityLimits {
		if count, exists := sr.Summary.BySeverity[severity]; exists && count > limit {
			return false
		}
	}

	// Check overall score threshold
	if sr.Summary.Score < policy.FailureThreshold {
		return false
	}

	return true
}

// GetCriticalVulnerabilities returns vulnerabilities with critical severity
func (sr *ScanResult) GetCriticalVulnerabilities() []Vulnerability {
	var critical []Vulnerability
	for _, vuln := range sr.Vulnerabilities {
		if vuln.Severity == SeverityCritical {
			critical = append(critical, vuln)
		}
	}
	return critical
}

// GetFixableVulnerabilities returns vulnerabilities that can be fixed
func (sr *ScanResult) GetFixableVulnerabilities() []Vulnerability {
	var fixable []Vulnerability
	for _, vuln := range sr.Vulnerabilities {
		if vuln.IsFixable {
			fixable = append(fixable, vuln)
		}
	}
	return fixable
}

// GetSecretsBySeverity returns secrets filtered by minimum severity
func (sr *ScanResult) GetSecretsBySeverity(minSeverity SeverityLevel) []Secret {
	severityOrder := map[SeverityLevel]int{
		SeverityCritical: 5,
		SeverityHigh:     4,
		SeverityMedium:   3,
		SeverityLow:      2,
		SeverityInfo:     1,
		SeverityUnknown:  0,
	}

	threshold := severityOrder[minSeverity]
	var filtered []Secret
	for _, secret := range sr.Secrets {
		if severityOrder[secret.Severity] >= threshold {
			filtered = append(filtered, secret)
		}
	}
	return filtered
}

// CalculateSecurityGrade calculates an overall security grade
func (sr *ScanResult) CalculateSecurityGrade() SecurityGrade {
	score := sr.Summary.Score

	if score >= 90 {
		return GradeA
	} else if score >= 80 {
		return GradeB
	} else if score >= 70 {
		return GradeC
	} else if score >= 60 {
		return GradeD
	}
	return GradeF
}

// ShouldBlockDeployment determines if deployment should be blocked based on scan results
func (sr *ScanResult) ShouldBlockDeployment() bool {
	// Block if scan failed
	if !sr.IsSuccessful() {
		return true
	}

	// Block if critical vulnerabilities found
	if sr.HasCriticalIssues() {
		return true
	}

	// Block if grade is F
	if sr.CalculateSecurityGrade() == GradeF {
		return true
	}

	// Block if active secrets detected
	for _, secret := range sr.Secrets {
		if secret.IsActive && (secret.Severity == SeverityCritical || secret.Severity == SeverityHigh) {
			return true
		}
	}

	return false
}

// Business Rules for Scanner Selection

// SelectOptimalScanner determines the best scanner for the scan type
func SelectOptimalScanner(scanType ScanType) Scanner {
	switch scanType {
	case ScanTypeVulnerability:
		return ScannerTrivy // Trivy is excellent for vulnerability scanning
	case ScanTypeSecret:
		return ScannerTrivy // Trivy also handles secrets well
	case ScanTypeMalware:
		return ScannerClair // Clair has good malware detection
	case ScanTypeCompliance:
		return ScannerAquaSec // Commercial solutions often better for compliance
	case ScanTypeConfiguration:
		return ScannerTrivy // Trivy supports config scanning
	case ScanTypeLicense:
		return ScannerSnyk // Snyk has good license scanning
	case ScanTypeComprehensive:
		return ScannerTrivy // Trivy supports multiple scan types
	default:
		return ScannerTrivy // Default to Trivy
	}
}

// EstimateScanTime estimates scan duration based on target and scan type
func EstimateScanTime(req *ScanRequest) time.Duration {
	baseTime := 2 * time.Minute // Default base time

	// Adjust for scan type
	switch req.ScanType {
	case ScanTypeVulnerability:
		baseTime = 3 * time.Minute
	case ScanTypeSecret:
		baseTime = 1 * time.Minute
	case ScanTypeMalware:
		baseTime = 5 * time.Minute
	case ScanTypeCompliance:
		baseTime = 4 * time.Minute
	case ScanTypeComprehensive:
		baseTime = 10 * time.Minute
	}

	// Adjust for target type
	switch req.Target.Type {
	case TargetTypeImage:
		// Image scans are typically faster
		baseTime = time.Duration(float64(baseTime) * 0.8)
	case TargetTypeRepository:
		// Repository scans take longer
		baseTime = time.Duration(float64(baseTime) * 1.5)
	case TargetTypeFilesystem:
		// Filesystem scans can be very slow
		baseTime = time.Duration(float64(baseTime) * 2.0)
	}

	return baseTime
}

// Validation helper functions

// isValidTargetType validates target type
func isValidTargetType(targetType TargetType) bool {
	validTypes := []TargetType{
		TargetTypeImage,
		TargetTypeRepository,
		TargetTypeManifest,
		TargetTypeFilesystem,
		TargetTypeContainer,
	}

	for _, validType := range validTypes {
		if targetType == validType {
			return true
		}
	}
	return false
}

// isValidScanType validates scan type
func isValidScanType(scanType ScanType) bool {
	validTypes := []ScanType{
		ScanTypeVulnerability,
		ScanTypeSecret,
		ScanTypeMalware,
		ScanTypeCompliance,
		ScanTypeConfiguration,
		ScanTypeLicense,
		ScanTypeComprehensive,
	}

	for _, validType := range validTypes {
		if scanType == validType {
			return true
		}
	}
	return false
}

// Business Rules for Risk Assessment

// AssessRiskLevel calculates overall risk level based on scan results
func (sr *ScanResult) AssessRiskLevel() RiskLevel {
	// Critical vulnerabilities or active secrets = High risk
	if sr.HasCriticalIssues() {
		return RiskLevelHigh
	}

	// Check for high severity issues
	highCount := sr.Summary.BySeverity[SeverityHigh]
	if highCount > 5 {
		return RiskLevelHigh
	} else if highCount > 0 {
		return RiskLevelMedium
	}

	// Check for secrets
	for _, secret := range sr.Secrets {
		if secret.IsActive && secret.Severity == SeverityHigh {
			return RiskLevelMedium
		}
	}

	// Check grade
	grade := sr.CalculateSecurityGrade()
	switch grade {
	case GradeA, GradeB:
		return RiskLevelLow
	case GradeC:
		return RiskLevelMedium
	case GradeD, GradeF:
		return RiskLevelHigh
	}

	return RiskLevelLow
}

// RiskLevel represents the overall risk level
type RiskLevel string

const (
	RiskLevelLow    RiskLevel = "low"
	RiskLevelMedium RiskLevel = "medium"
	RiskLevelHigh   RiskLevel = "high"
)

// GetRecommendations returns security recommendations based on scan results
func (sr *ScanResult) GetRecommendations() []SecurityRecommendation {
	var recommendations []SecurityRecommendation

	// Vulnerability recommendations
	if len(sr.GetFixableVulnerabilities()) > 0 {
		recommendations = append(recommendations, SecurityRecommendation{
			Type:        "vulnerability",
			Priority:    "high",
			Description: fmt.Sprintf("Update packages to fix %d fixable vulnerabilities", len(sr.GetFixableVulnerabilities())),
		})
	}

	// Secret recommendations
	activeSecrets := sr.GetSecretsBySeverity(SeverityHigh)
	if len(activeSecrets) > 0 {
		recommendations = append(recommendations, SecurityRecommendation{
			Type:        "secret",
			Priority:    "critical",
			Description: "Remove or rotate detected secrets and credentials",
		})
	}

	// Base image recommendations
	if sr.HasCriticalIssues() {
		recommendations = append(recommendations, SecurityRecommendation{
			Type:        "base_image",
			Priority:    "high",
			Description: "Consider using a more secure base image",
		})
	}

	// Compliance recommendations
	for _, compliance := range sr.Compliance {
		if !compliance.Passed {
			recommendations = append(recommendations, SecurityRecommendation{
				Type:        "compliance",
				Priority:    "medium",
				Description: fmt.Sprintf("Address %s compliance failures", compliance.Standard),
			})
		}
	}

	return recommendations
}

// SecurityRecommendation represents a security recommendation
type SecurityRecommendation struct {
	Type        string `json:"type"`
	Priority    string `json:"priority"`
	Description string `json:"description"`
}
