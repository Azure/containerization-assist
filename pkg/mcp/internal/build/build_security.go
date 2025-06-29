package build

import (
	"context"
	"fmt"
	"time"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp"
	mcptypes "github.com/Azure/container-kit/pkg/mcp"
	"github.com/rs/zerolog"
)

// BuildSecurityScanner handles security scanning of built images
type BuildSecurityScanner struct {
	logger zerolog.Logger
}

// NewBuildSecurityScanner creates a new build security scanner
func NewBuildSecurityScanner(logger zerolog.Logger) *BuildSecurityScanner {
	return &BuildSecurityScanner{
		logger: logger.With().Str("component", "build_security_scanner").Logger(),
	}
}

// RunSecurityScan performs security scanning on the built image
func (s *BuildSecurityScanner) RunSecurityScan(ctx context.Context, session *mcp.SessionState, result *AtomicBuildImageResult) error {
	// Create Trivy scanner
	scanner := coredocker.NewTrivyScanner(s.logger)
	// Check if Trivy is installed
	if !scanner.CheckTrivyInstalled() {
		s.logger.Info().Msg("Trivy not installed, skipping security scan")
		if result.BuildContext_Info == nil {
			result.BuildContext_Info = &BuildContextInfo{}
		}
		result.BuildContext_Info.SecurityRecommendations = append(
			result.BuildContext_Info.SecurityRecommendations,
			"Install Trivy for container security scanning: curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin",
		)
		return nil
	}
	scanStartTime := time.Now()
	// Run security scan with HIGH severity threshold
	scanResult, err := scanner.ScanImage(ctx, result.FullImageRef, "HIGH,CRITICAL")
	if err != nil {
		return mcp.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("security scan failed: %v", err), "scan_error")
	}
	result.ScanDuration = time.Since(scanStartTime)
	result.SecurityScan = scanResult
	// Log scan summary
	s.logger.Info().
		Str("image", result.FullImageRef).
		Int("total_vulnerabilities", scanResult.Summary.Total).
		Int("critical", scanResult.Summary.Critical).
		Int("high", scanResult.Summary.High).
		Dur("scan_duration", result.ScanDuration).
		Msg("Security scan completed")
	// Update session state with scan results
	session.SecurityScan = &mcptypes.SecurityScanResult{
		Success:   scanResult.Success,
		ScannedAt: scanResult.ScanTime,
		ImageRef:  result.FullImageRef,
		Vulnerabilities: mcptypes.VulnerabilityCount{
			Total:    scanResult.Summary.Total,
			Critical: scanResult.Summary.Critical,
			High:     scanResult.Summary.High,
			Medium:   scanResult.Summary.Medium,
			Low:      scanResult.Summary.Low,
			Unknown:  scanResult.Summary.Unknown,
		},
		Scanner: "trivy",
	}
	// Also store in metadata for backward compatibility
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}
	session.Metadata["security_scan"] = map[string]interface{}{
		"scanned_at":     scanResult.ScanTime,
		"total_vulns":    scanResult.Summary.Total,
		"critical_vulns": scanResult.Summary.Critical,
		"high_vulns":     scanResult.Summary.High,
		"scan_success":   scanResult.Success,
	}
	// Process scan results and add recommendations
	return s.processScanResults(scanResult, result)
}

// processScanResults processes scan results and adds appropriate recommendations
func (s *BuildSecurityScanner) processScanResults(scanResult *coredocker.ScanResult, result *AtomicBuildImageResult) error {
	if result.BuildContext_Info == nil {
		result.BuildContext_Info = &BuildContextInfo{}
	}
	// Add security recommendations based on scan results
	if scanResult.Summary.Critical > 0 || scanResult.Summary.High > 0 {
		result.BuildContext_Info.SecurityRecommendations = append(
			result.BuildContext_Info.SecurityRecommendations,
			fmt.Sprintf("⚠️ Found %d CRITICAL and %d HIGH severity vulnerabilities",
				scanResult.Summary.Critical, scanResult.Summary.High),
		)
		// Add remediation steps to build context
		for _, step := range scanResult.Remediation {
			result.BuildContext_Info.SecurityRecommendations = append(
				result.BuildContext_Info.SecurityRecommendations,
				fmt.Sprintf("%d. %s: %s", step.Priority, step.Action, step.Description),
			)
		}
		// Mark as failed if critical vulnerabilities found
		if scanResult.Summary.Critical > 0 {
			s.logger.Error().
				Int("critical_vulns", scanResult.Summary.Critical).
				Int("high_vulns", scanResult.Summary.High).
				Str("image_ref", result.FullImageRef).
				Msg("Critical security vulnerabilities found")
			result.Success = false
			return mcp.NewRichError("INTERNAL_SERVER_ERROR", "critical vulnerabilities found", "security_error")
		}
	}
	// Update next steps based on scan results
	if scanResult.Success {
		result.BuildContext_Info.NextStepSuggestions = append(
			result.BuildContext_Info.NextStepSuggestions,
			"✅ Security scan passed - image is safe to deploy",
		)
	} else {
		result.BuildContext_Info.NextStepSuggestions = append(
			result.BuildContext_Info.NextStepSuggestions,
			"⚠️ Security vulnerabilities found - review and fix before deployment",
		)
	}
	return nil
}
