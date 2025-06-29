package build

import (
	"context"
	"fmt"
	"time"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/core"
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
func (s *BuildSecurityScanner) RunSecurityScan(ctx context.Context, session *core.SessionState, result *AtomicBuildImageResult) error {
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
		return fmt.Errorf("security scan failed: %w", err)
	}

	// Update scan duration (Note: ScanDuration field needs to be added to BuildContextInfo)
	// result.BuildContext_Info.ScanDuration = time.Since(scanStartTime)
	// For now, we'll track this in the result's main scan duration field
	result.ScanDuration = time.Since(scanStartTime)

	// Process scan results
	if scanResult != nil {
		result.BuildContext_Info.SecurityRecommendations = append(
			result.BuildContext_Info.SecurityRecommendations,
			"Security scan completed successfully",
		)
	}

	return nil
}
