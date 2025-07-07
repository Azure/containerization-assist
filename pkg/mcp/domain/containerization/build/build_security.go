package build

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
)

// BuildSecurityScanner handles security scanning of built images
type BuildSecurityScanner struct {
	logger *slog.Logger
}

// NewBuildSecurityScanner creates a new build security scanner
func NewBuildSecurityScanner(logger *slog.Logger) *BuildSecurityScanner {
	return &BuildSecurityScanner{
		logger: logger.With("component", "build_security_scanner"),
	}
}

// RunSecurityScan performs security scanning on the built image
func (s *BuildSecurityScanner) RunSecurityScan(ctx context.Context, session *core.SessionState, result *AtomicBuildImageResult) error {
	scanStartTime := time.Now()

	s.logger.Info("Starting security scan for built image",
		"image_ref", result.FullImageRef,
		"session_id", session.SessionID)

	// Initialize BuildContext_Info if needed
	if result.BuildContext_Info == nil {
		result.BuildContext_Info = &BuildContextInfo{}
	}

	// Create security scanner with slog support
	scanner := s.createSecurityScanner()

	// Attempt to scan the built image
	scanResult, err := scanner.ScanBuiltImage(ctx, result.FullImageRef)
	if err != nil {
		s.logger.Warn("Security scan failed, continuing with basic recommendations",
			"error", err,
			"image_ref", result.FullImageRef)

		// Add basic security recommendations even if scan fails
		s.addBasicSecurityRecommendations(result)
		result.ScanDuration = time.Since(scanStartTime)
		return nil // Don't fail the build for scan failures
	}

	result.ScanDuration = time.Since(scanStartTime)

	// Process scan results and add recommendations
	s.processScanResults(result, scanResult)

	s.logger.Info("Security scan completed",
		"image_ref", result.FullImageRef,
		"vulnerabilities_found", len(scanResult.Vulnerabilities),
		"scan_duration", result.ScanDuration)

	return nil
}

// BuildImageScanner interface for scanning built images
type BuildImageScanner interface {
	ScanBuiltImage(ctx context.Context, imageRef string) (*BuildScanResult, error)
}

// BuildScanResult represents the result of scanning a built image
type BuildScanResult struct {
	Success         bool                         `json:"success"`
	ImageRef        string                       `json:"image_ref"`
	Vulnerabilities []BuildSecurityVulnerability `json:"vulnerabilities"`
	Summary         BuildSecuritySummary         `json:"summary"`
	Recommendations []string                     `json:"recommendations"`
	ScanTime        time.Time                    `json:"scan_time"`
	Duration        time.Duration                `json:"duration"`
}

// BuildSecurityVulnerability represents a security vulnerability in a built image
type BuildSecurityVulnerability struct {
	ID          string   `json:"id"`
	Severity    string   `json:"severity"`
	Package     string   `json:"package"`
	Version     string   `json:"version"`
	FixedIn     string   `json:"fixed_in"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Layer       string   `json:"layer"`
	References  []string `json:"references"`
}

// BuildSecuritySummary provides summary statistics for build security scan
type BuildSecuritySummary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

// BuildImageScannerImpl implements BuildImageScanner with slog support
type BuildImageScannerImpl struct {
	logger *slog.Logger
}

// createSecurityScanner creates a new build image scanner
func (s *BuildSecurityScanner) createSecurityScanner() BuildImageScanner {
	return &BuildImageScannerImpl{
		logger: s.logger.With("component", "build_image_scanner"),
	}
}

// ScanBuiltImage performs security scanning on a built image
func (bis *BuildImageScannerImpl) ScanBuiltImage(ctx context.Context, imageRef string) (*BuildScanResult, error) {
	startTime := time.Now()

	bis.logger.Info("Scanning built image", "image_ref", imageRef)

	// Check if external scanners are available
	if bis.isTrivyAvailable() {
		return bis.scanWithTrivy(ctx, imageRef, startTime)
	}

	if bis.isDockerScanAvailable() {
		return bis.scanWithDockerScan(ctx, imageRef, startTime)
	}

	// Fall back to basic security assessment
	return bis.performBasicBuildScan(ctx, imageRef, startTime), nil
}

// isTrivyAvailable checks if trivy is available for scanning
func (bis *BuildImageScannerImpl) isTrivyAvailable() bool {
	// TODO: Check for actual trivy executable
	return false
}

// isDockerScanAvailable checks if docker scan is available
func (bis *BuildImageScannerImpl) isDockerScanAvailable() bool {
	// TODO: Check for docker scan capability
	return false
}

// scanWithTrivy performs scanning using trivy
func (bis *BuildImageScannerImpl) scanWithTrivy(ctx context.Context, imageRef string, startTime time.Time) (*BuildScanResult, error) {
	// TODO: Implement actual trivy integration
	return nil, fmt.Errorf("trivy scanner not available")
}

// scanWithDockerScan performs scanning using docker scan
func (bis *BuildImageScannerImpl) scanWithDockerScan(ctx context.Context, imageRef string, startTime time.Time) (*BuildScanResult, error) {
	// TODO: Implement docker scan integration
	return nil, fmt.Errorf("docker scan not available")
}

// performBasicBuildScan performs basic security assessment for built images
func (bis *BuildImageScannerImpl) performBasicBuildScan(ctx context.Context, imageRef string, startTime time.Time) *BuildScanResult {
	bis.logger.Info("Performing basic build security assessment", "image_ref", imageRef)

	// Generate basic security findings for built images
	vulns := []BuildSecurityVulnerability{
		{
			ID:          "BUILD-SEC-001",
			Severity:    "MEDIUM",
			Package:     "image-config",
			Version:     "unknown",
			FixedIn:     "secure-config",
			Title:       "Image configuration review",
			Description: "Built image should be reviewed for security configuration",
			Layer:       "config",
			References:  []string{"https://docs.docker.com/develop/security-best-practices/"},
		},
		{
			ID:          "BUILD-SEC-002",
			Severity:    "LOW",
			Package:     "build-context",
			Version:     "current",
			FixedIn:     "optimized",
			Title:       "Build context optimization",
			Description: "Build context may contain unnecessary files that increase attack surface",
			Layer:       "build",
			References:  []string{"https://docs.docker.com/develop/dev-best-practices/"},
		},
	}

	return &BuildScanResult{
		Success:         true,
		ImageRef:        imageRef,
		Vulnerabilities: vulns,
		Summary: BuildSecuritySummary{
			Total:    len(vulns),
			Critical: 0,
			High:     0,
			Medium:   1,
			Low:      1,
		},
		Recommendations: []string{
			"Install a security scanner (Trivy recommended) for comprehensive vulnerability detection",
			"Use multi-stage builds to reduce final image size",
			"Use minimal base images (alpine, distroless)",
			"Don't include unnecessary files in build context",
			"Run containers as non-root user",
			"Regularly update base images and dependencies",
		},
		ScanTime: startTime,
		Duration: time.Since(startTime),
	}
}

// addBasicSecurityRecommendations adds basic security recommendations when scanning fails
func (s *BuildSecurityScanner) addBasicSecurityRecommendations(result *AtomicBuildImageResult) {
	recommendations := []string{
		"Install Trivy for comprehensive security scanning: curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh",
		"Enable Docker scan if using Docker Desktop",
		"Use minimal base images (alpine, distroless)",
		"Implement multi-stage builds to reduce attack surface",
		"Run containers as non-root user when possible",
		"Regularly update base images and dependencies",
		"Remove unnecessary packages and files from final image",
		"Review and minimize exposed ports",
	}

	result.BuildContext_Info.SecurityRecommendations = append(
		result.BuildContext_Info.SecurityRecommendations,
		recommendations...)
}

// processScanResults processes security scan results and updates build result
func (s *BuildSecurityScanner) processScanResults(result *AtomicBuildImageResult, scanResult *BuildScanResult) {
	// Add security summary
	securitySummary := fmt.Sprintf("Security scan found %d vulnerabilities (%d critical, %d high, %d medium, %d low)",
		scanResult.Summary.Total,
		scanResult.Summary.Critical,
		scanResult.Summary.High,
		scanResult.Summary.Medium,
		scanResult.Summary.Low)

	result.BuildContext_Info.SecurityRecommendations = append(
		result.BuildContext_Info.SecurityRecommendations,
		securitySummary)

	// Add specific recommendations from scan
	result.BuildContext_Info.SecurityRecommendations = append(
		result.BuildContext_Info.SecurityRecommendations,
		scanResult.Recommendations...)

	// Add vulnerability details if any critical or high severity issues found
	if scanResult.Summary.Critical > 0 || scanResult.Summary.High > 0 {
		criticalHighVulns := []string{}
		for _, vuln := range scanResult.Vulnerabilities {
			if vuln.Severity == "CRITICAL" || vuln.Severity == "HIGH" {
				vulnDetail := fmt.Sprintf("%s: %s in %s %s", vuln.Severity, vuln.Title, vuln.Package, vuln.Version)
				if vuln.FixedIn != "" {
					vulnDetail += fmt.Sprintf(" (fix available in %s)", vuln.FixedIn)
				}
				criticalHighVulns = append(criticalHighVulns, vulnDetail)
			}
		}

		if len(criticalHighVulns) > 0 {
			result.BuildContext_Info.SecurityRecommendations = append(
				result.BuildContext_Info.SecurityRecommendations,
				"Critical/High vulnerabilities found:")
			result.BuildContext_Info.SecurityRecommendations = append(
				result.BuildContext_Info.SecurityRecommendations,
				criticalHighVulns...)
		}
	}
}
