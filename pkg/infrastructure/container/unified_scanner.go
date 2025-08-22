package container

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	mcperrors "github.com/Azure/containerization-assist/pkg/domain/errors"
	coresecurity "github.com/Azure/containerization-assist/pkg/infrastructure/security"
	"github.com/rs/zerolog"
)

// UnifiedSecurityScanner combines multiple vulnerability scanners for comprehensive security analysis
type UnifiedSecurityScanner struct {
	trivyScanner *TrivyScanner
	grypeScanner *GrypeScanner
	enableTrivy  bool
	enableGrype  bool
}

// NewUnifiedSecurityScanner creates a new unified scanner with both Trivy and Grype
func NewUnifiedSecurityScanner(logger zerolog.Logger) *UnifiedSecurityScanner {
	scanner := &UnifiedSecurityScanner{
		trivyScanner: NewTrivyScanner(logger),
		grypeScanner: NewGrypeScanner(logger),
	}

	// Check which scanners are available
	scanner.enableTrivy = scanner.trivyScanner.CheckTrivyInstalled()
	scanner.enableGrype = scanner.grypeScanner.CheckGrypeInstalled()

	return scanner
}

// UnifiedScanResult combines results from multiple scanners
type UnifiedScanResult struct {
	Success           bool                              `json:"success"`
	ImageRef          string                            `json:"image_ref"`
	ScanTime          time.Time                         `json:"scan_time"`
	Duration          time.Duration                     `json:"duration"`
	TrivyResult       *ScanResult                       `json:"trivy_result,omitempty"`
	GrypeResult       *ScanResult                       `json:"grype_result,omitempty"`
	CombinedSummary   coresecurity.VulnerabilitySummary `json:"combined_summary"`
	UniqueVulns       []coresecurity.Vulnerability      `json:"unique_vulnerabilities"`
	Remediation       []coresecurity.RemediationStep    `json:"remediation"`
	ComparisonMetrics ComparisonMetrics                 `json:"comparison_metrics"`
	Context           map[string]interface{}            `json:"context"`
}

// ComparisonMetrics provides insights into scanner differences
type ComparisonMetrics struct {
	TrivyOnly        int     `json:"trivy_only_count"`
	GrypeOnly        int     `json:"grype_only_count"`
	BothScanners     int     `json:"both_scanners_count"`
	AgreementRate    float64 `json:"agreement_rate"`
	SeverityMismatch int     `json:"severity_mismatch_count"`
}

// ScanImage performs a comprehensive security scan using all available scanners
func (us *UnifiedSecurityScanner) ScanImage(ctx context.Context, imageRef string, severityThreshold string) (*UnifiedScanResult, error) {
	if !us.enableTrivy && !us.enableGrype {
		return nil, mcperrors.New(mcperrors.CodeInternalError, "core", "no vulnerability scanners available. Install Trivy or Grype", nil)
	}

	startTime := time.Now()
	result := &UnifiedScanResult{
		ImageRef:    imageRef,
		ScanTime:    startTime,
		Context:     make(map[string]interface{}),
		Remediation: make([]coresecurity.RemediationStep, 0),
	}

	// Run scanners in parallel
	var wg sync.WaitGroup
	var trivyErr, grypeErr error

	if us.enableTrivy {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result.TrivyResult, trivyErr = us.trivyScanner.ScanImage(ctx, imageRef, severityThreshold)
		}()
	}

	if us.enableGrype {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result.GrypeResult, grypeErr = us.grypeScanner.ScanImage(ctx, imageRef, severityThreshold)
		}()
	}

	wg.Wait()
	result.Duration = time.Since(startTime)

	// Handle errors
	if trivyErr != nil && grypeErr != nil {
		return result, mcperrors.New(mcperrors.CodeOperationFailed, "docker", fmt.Sprintf("all scanners failed: trivy: %v, grype: %v", trivyErr, grypeErr), nil)
	}

	if trivyErr != nil {
		result.Context["trivy_error"] = trivyErr.Error()
	}
	if grypeErr != nil {
		result.Context["grype_error"] = grypeErr.Error()
	}

	// Combine and analyze results
	us.combineResults(result)
	us.generateUnifiedRemediation(result)

	// Determine overall success
	result.Success = result.CombinedSummary.Critical == 0 && result.CombinedSummary.High == 0

	return result, nil
}

// combineResults merges results from multiple scanners
func (us *UnifiedSecurityScanner) combineResults(result *UnifiedScanResult) {
	vulnMap := make(map[string]*coresecurity.Vulnerability)
	trivyVulns := make(map[string]bool)
	grypeVulns := make(map[string]bool)

	// Process Trivy results
	if result.TrivyResult != nil {
		for _, vuln := range result.TrivyResult.Vulnerabilities {
			key := fmt.Sprintf("%s-%s", vuln.VulnerabilityID, vuln.PkgName)
			vulnCopy := vuln
			vulnMap[key] = &vulnCopy
			trivyVulns[key] = true
		}
	}

	// Process Grype results
	if result.GrypeResult != nil {
		for _, vuln := range result.GrypeResult.Vulnerabilities {
			key := fmt.Sprintf("%s-%s", vuln.VulnerabilityID, vuln.PkgName)
			grypeVulns[key] = true

			if existing, ok := vulnMap[key]; ok {
				// Merge information from both scanners
				vulnCopy := vuln
				us.mergeVulnerability(existing, &vulnCopy)
			} else {
				vulnCopy := vuln
				vulnMap[key] = &vulnCopy
			}
		}
	}

	// Calculate comparison metrics
	metrics := &result.ComparisonMetrics
	for key := range vulnMap {
		inTrivy := trivyVulns[key]
		inGrype := grypeVulns[key]

		if inTrivy && inGrype {
			metrics.BothScanners++
		} else if inTrivy {
			metrics.TrivyOnly++
		} else {
			metrics.GrypeOnly++
		}
	}

	total := metrics.TrivyOnly + metrics.GrypeOnly + metrics.BothScanners
	if total > 0 {
		metrics.AgreementRate = float64(metrics.BothScanners) / float64(total) * 100
	}

	// Build unique vulnerability list and combined summary
	summary := &result.CombinedSummary
	for _, vuln := range vulnMap {
		result.UniqueVulns = append(result.UniqueVulns, *vuln)

		summary.Total++
		if vuln.FixedVersion != "" {
			summary.Fixable++
		}

		switch strings.ToUpper(vuln.Severity) {
		case "CRITICAL":
			summary.Critical++
		case "HIGH":
			summary.High++
		case "MEDIUM":
			summary.Medium++
		case "LOW":
			summary.Low++
		default:
			summary.Unknown++
		}
	}

	// Store scanner availability in context
	result.Context["scanners_used"] = us.getScannersUsed()
	result.Context["scan_duration_ms"] = result.Duration.Milliseconds()
}

// mergeVulnerability combines information from multiple scanners
func (us *UnifiedSecurityScanner) mergeVulnerability(existing, newVuln *coresecurity.Vulnerability) {
	// Prefer non-empty values
	if existing.Description == "" && newVuln.Description != "" {
		existing.Description = newVuln.Description
	}
	if existing.Title == "" && newVuln.Title != "" {
		existing.Title = newVuln.Title
	}
	if existing.FixedVersion == "" && newVuln.FixedVersion != "" {
		existing.FixedVersion = newVuln.FixedVersion
	}

	// Merge references (unique URLs)
	refMap := make(map[string]bool)
	for _, ref := range existing.References {
		refMap[ref] = true
	}
	for _, ref := range newVuln.References {
		if !refMap[ref] {
			existing.References = append(existing.References, ref)
		}
	}
}

// generateUnifiedRemediation creates comprehensive remediation steps
func (us *UnifiedSecurityScanner) generateUnifiedRemediation(result *UnifiedScanResult) {
	if result.CombinedSummary.Total == 0 {
		result.Remediation = append(result.Remediation, coresecurity.RemediationStep{
			Priority:    1,
			Action:      "No action required",
			Description: "No vulnerabilities found by any scanner",
		})
		return
	}

	priority := 1

	// Critical and High vulnerabilities
	if result.CombinedSummary.Critical > 0 || result.CombinedSummary.High > 0 {
		result.Remediation = append(result.Remediation, coresecurity.RemediationStep{
			Priority: priority,
			Action:   "Address critical security issues",
			Description: fmt.Sprintf("Found %d CRITICAL and %d HIGH severity vulnerabilities across all scanners",
				result.CombinedSummary.Critical, result.CombinedSummary.High),
		})
		priority++
	}

	// Scanner discrepancies
	if result.ComparisonMetrics.AgreementRate < 80 {
		scannerSpecific := result.ComparisonMetrics.TrivyOnly + result.ComparisonMetrics.GrypeOnly
		result.Remediation = append(result.Remediation, coresecurity.RemediationStep{
			Priority: priority,
			Action:   "Review scanner-specific findings",
			Description: fmt.Sprintf("%d vulnerabilities were found by only one scanner (%.1f%% agreement rate). Manual review recommended",
				scannerSpecific, result.ComparisonMetrics.AgreementRate),
		})
		priority++
	}

	// Fixable vulnerabilities
	if result.CombinedSummary.Fixable > 0 {
		fixRate := float64(result.CombinedSummary.Fixable) / float64(result.CombinedSummary.Total) * 100
		result.Remediation = append(result.Remediation, coresecurity.RemediationStep{
			Priority: priority,
			Action:   "Apply available patches",
			Description: fmt.Sprintf("%d of %d vulnerabilities (%.1f%%) have fixes available",
				result.CombinedSummary.Fixable, result.CombinedSummary.Total, fixRate),
			Command: "docker build --no-cache -t <image>:<tag> .",
		})
		priority++
	}

	// Add scanner-specific remediation if available
	if result.TrivyResult != nil && len(result.TrivyResult.Remediation) > 0 {
		for _, step := range result.TrivyResult.Remediation {
			if step.Action != "No action required" {
				step.Priority = priority
				step.Description = "[Trivy] " + step.Description
				result.Remediation = append(result.Remediation, step)
				priority++
			}
		}
	}

	// General best practices
	result.Remediation = append(result.Remediation, coresecurity.RemediationStep{
		Priority:    priority,
		Action:      "Implement continuous scanning",
		Description: "Set up automated scanning in CI/CD pipeline to catch vulnerabilities early",
	})
}

// getScannersUsed returns a list of scanners that were used
func (us *UnifiedSecurityScanner) getScannersUsed() []string {
	scanners := make([]string, 0, 2)
	if us.enableTrivy {
		scanners = append(scanners, "trivy")
	}
	if us.enableGrype {
		scanners = append(scanners, "grype")
	}
	return scanners
}
