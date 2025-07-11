package docker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	coresecurity "github.com/Azure/container-kit/pkg/core/security"
	mcperrors "github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/rs/zerolog"
)

// UnifiedSecurityScanner combines multiple vulnerability scanners for comprehensive security analysis
type UnifiedSecurityScanner struct {
	logger       zerolog.Logger
	trivyScanner *TrivyScanner
	grypeScanner *GrypeScanner
	enableTrivy  bool
	enableGrype  bool
}

// NewUnifiedSecurityScanner creates a new unified scanner with both Trivy and Grype
func NewUnifiedSecurityScanner(logger zerolog.Logger) *UnifiedSecurityScanner {
	scanner := &UnifiedSecurityScanner{
		logger:       logger.With().Str("component", "unified_scanner").Logger(),
		trivyScanner: NewTrivyScanner(logger),
		grypeScanner: NewGrypeScanner(logger),
	}

	// Check which scanners are available
	scanner.enableTrivy = scanner.trivyScanner.CheckTrivyInstalled()
	scanner.enableGrype = scanner.grypeScanner.CheckGrypeInstalled()

	scanner.logger.Info().
		Bool("trivy_enabled", scanner.enableTrivy).
		Bool("grype_enabled", scanner.enableGrype).
		Msg("Initialized unified security scanner")

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

	us.logger.Info().
		Str("image", imageRef).
		Str("severity_threshold", severityThreshold).
		Msg("Starting unified security scan")

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
		us.logger.Warn().Err(trivyErr).Msg("Trivy scan failed, using Grype results only")
		result.Context["trivy_error"] = trivyErr.Error()
	}
	if grypeErr != nil {
		us.logger.Warn().Err(grypeErr).Msg("Grype scan failed, using Trivy results only")
		result.Context["grype_error"] = grypeErr.Error()
	}

	// Combine and analyze results
	us.combineResults(result)
	us.generateUnifiedRemediation(result)

	// Determine overall success
	result.Success = result.CombinedSummary.Critical == 0 && result.CombinedSummary.High == 0

	us.logger.Info().
		Bool("success", result.Success).
		Int("total_unique_vulnerabilities", len(result.UniqueVulns)).
		Int("critical", result.CombinedSummary.Critical).
		Int("high", result.CombinedSummary.High).
		Float64("scanner_agreement_rate", result.ComparisonMetrics.AgreementRate).
		Dur("duration", result.Duration).
		Msg("Unified scan completed")

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

// GetAvailableScanners returns information about available scanners
func (us *UnifiedSecurityScanner) GetAvailableScanners() map[string]bool {
	return map[string]bool{
		"trivy": us.enableTrivy,
		"grype": us.enableGrype,
	}
}

// FormatUnifiedScanSummary formats unified scan results for display
func (us *UnifiedSecurityScanner) FormatUnifiedScanSummary(result *UnifiedScanResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Unified Security Scan Results for %s:\n", result.ImageRef))
	sb.WriteString(fmt.Sprintf("Scan completed in %v using %v\n\n",
		result.Duration.Round(time.Millisecond), result.Context["scanners_used"]))

	// Combined summary
	sb.WriteString("Combined Vulnerability Summary:\n")
	sb.WriteString(fmt.Sprintf("  CRITICAL: %d\n", result.CombinedSummary.Critical))
	sb.WriteString(fmt.Sprintf("  HIGH:     %d\n", result.CombinedSummary.High))
	sb.WriteString(fmt.Sprintf("  MEDIUM:   %d\n", result.CombinedSummary.Medium))
	sb.WriteString(fmt.Sprintf("  LOW:      %d\n", result.CombinedSummary.Low))
	sb.WriteString(fmt.Sprintf("  TOTAL:    %d (Fixable: %d)\n",
		result.CombinedSummary.Total, result.CombinedSummary.Fixable))

	// Scanner comparison
	sb.WriteString("\nScanner Comparison:\n")
	sb.WriteString(fmt.Sprintf("  Agreement Rate: %.1f%%\n", result.ComparisonMetrics.AgreementRate))
	sb.WriteString(fmt.Sprintf("  Found by both:  %d\n", result.ComparisonMetrics.BothScanners))
	sb.WriteString(fmt.Sprintf("  Trivy only:     %d\n", result.ComparisonMetrics.TrivyOnly))
	sb.WriteString(fmt.Sprintf("  Grype only:     %d\n", result.ComparisonMetrics.GrypeOnly))

	// Status
	if result.Success {
		sb.WriteString("\n✅ Image passed security requirements\n")
	} else {
		sb.WriteString(fmt.Sprintf("\n❌ Image has %d CRITICAL and %d HIGH severity vulnerabilities\n",
			result.CombinedSummary.Critical, result.CombinedSummary.High))
	}

	// Remediation steps
	if len(result.Remediation) > 0 {
		sb.WriteString("\nRemediation Steps:\n")
		for _, step := range result.Remediation {
			sb.WriteString(fmt.Sprintf("%d. %s\n", step.Priority, step.Action))
			sb.WriteString(fmt.Sprintf("   %s\n", step.Description))
			if step.Command != "" {
				sb.WriteString(fmt.Sprintf("   Command: %s\n", step.Command))
			}
		}
	}

	return sb.String()
}
