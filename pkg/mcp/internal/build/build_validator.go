package build

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	coredocker "github.com/Azure/container-copilot/pkg/core/docker"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// BuildValidatorImpl handles build validation and security scanning
type BuildValidatorImpl struct {
	logger zerolog.Logger
}

// NewBuildValidator creates a new build validator
func NewBuildValidator(logger zerolog.Logger) *BuildValidatorImpl {
	return &BuildValidatorImpl{
		logger: logger,
	}
}

// ValidateBuildPrerequisites validates that all prerequisites for building are met
func (bv *BuildValidatorImpl) ValidateBuildPrerequisites(dockerfilePath string, buildContext string) error {
	// Check if Dockerfile exists
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		return types.NewErrorBuilder("invalid_arguments",
			fmt.Sprintf("Dockerfile not found at %s", dockerfilePath), "validation").
			WithSeverity("high").
			WithOperation("ValidateBuildPrerequisites").
			WithField("dockerfilePath", dockerfilePath).
			Build()
	}

	// Check if build context exists
	if _, err := os.Stat(buildContext); os.IsNotExist(err) {
		return types.NewErrorBuilder("invalid_arguments",
			fmt.Sprintf("Build context directory not found at %s", buildContext), "validation").
			WithSeverity("high").
			WithOperation("ValidateBuildPrerequisites").
			WithField("buildContext", buildContext).
			Build()
	}

	// Check if Docker is available
	cmd := exec.Command("docker", "version")
	if err := cmd.Run(); err != nil {
		return types.NewErrorBuilder("internal_server_error",
			"Docker is not available. Please ensure Docker is installed and running", "execution").
			WithSeverity("critical").
			WithOperation("ValidateBuildPrerequisites").
			WithRootCause("Docker daemon not running").
			Build()
	}

	return nil
}

// RunSecurityScan runs a security scan on the built image using Trivy
func (bv *BuildValidatorImpl) RunSecurityScan(ctx context.Context, imageName string, imageTag string) (*coredocker.ScanResult, time.Duration, error) {
	startTime := time.Now()

	// Check if Trivy is installed
	cmd := exec.Command("trivy", "--version")
	if err := cmd.Run(); err != nil {
		bv.logger.Warn().Msg("Trivy not found, skipping security scan")
		return nil, 0, nil
	}

	fullImageRef := fmt.Sprintf("%s:%s", imageName, imageTag)
	bv.logger.Info().Str("image", fullImageRef).Msg("Running security scan with Trivy")

	// Run Trivy scan
	scanCmd := exec.CommandContext(ctx, "trivy", "image", "--format", "json", "--quiet", fullImageRef)
	output, err := scanCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			bv.logger.Warn().Str("stderr", string(exitErr.Stderr)).Msg("Trivy scan failed")
		}
		return nil, time.Since(startTime), types.NewErrorBuilder("internal_server_error",
			"Security scan failed", "execution").
			WithSeverity("medium").
			WithOperation("RunSecurityScan").
			WithField("image", fullImageRef).
			Build()
	}

	// Parse the scan results (simplified for now)
	scanResult := &coredocker.ScanResult{
		Success:  true,
		ImageRef: fullImageRef,
		ScanTime: time.Now(),
		Duration: time.Since(startTime),
		Summary: coredocker.VulnerabilitySummary{
			Total:    0,
			Critical: 0,
			High:     0,
			Medium:   0,
			Low:      0,
			Unknown:  0,
			Fixable:  0,
		},
		Vulnerabilities: []coredocker.Vulnerability{},
		Remediation:     []coredocker.RemediationStep{},
		Context:         make(map[string]interface{}),
	}

	// Parse Trivy JSON output
	var trivyResult coredocker.TrivyResult
	if err := json.Unmarshal(output, &trivyResult); err != nil {
		bv.logger.Warn().
			Err(err).
			Str("output", string(output)).
			Msg("Failed to parse Trivy JSON output, falling back to string matching")

		// Fallback to string matching if JSON parsing fails
		outputStr := string(output)
		if strings.Contains(outputStr, "CRITICAL") {
			scanResult.Summary.Critical = 1
			scanResult.Summary.Total = 1
		}
		if strings.Contains(outputStr, "HIGH") {
			scanResult.Summary.High = 1
			scanResult.Summary.Total++
		}
	} else {
		// Process properly parsed JSON results
		for _, result := range trivyResult.Results {
			for _, vuln := range result.Vulnerabilities {
				// Add to vulnerabilities list
				vulnerability := coredocker.Vulnerability{
					VulnerabilityID:  vuln.VulnerabilityID,
					PkgName:          vuln.PkgName,
					InstalledVersion: vuln.InstalledVersion,
					FixedVersion:     vuln.FixedVersion,
					Severity:         vuln.Severity,
					Title:            vuln.Title,
					Description:      vuln.Description,
					References:       vuln.References,
				}

				if vuln.Layer.DiffID != "" {
					vulnerability.Layer = vuln.Layer.DiffID
				}

				scanResult.Vulnerabilities = append(scanResult.Vulnerabilities, vulnerability)

				// Update summary counts
				switch strings.ToUpper(vuln.Severity) {
				case "CRITICAL":
					scanResult.Summary.Critical++
				case "HIGH":
					scanResult.Summary.High++
				case "MEDIUM":
					scanResult.Summary.Medium++
				case "LOW":
					scanResult.Summary.Low++
				default:
					scanResult.Summary.Unknown++
				}
				scanResult.Summary.Total++

				// Count fixable vulnerabilities
				if vuln.FixedVersion != "" {
					scanResult.Summary.Fixable++
				}
			}
		}

		// Add remediation recommendations for critical and high vulnerabilities
		if scanResult.Summary.Critical > 0 || scanResult.Summary.High > 0 {
			scanResult.Remediation = append(scanResult.Remediation, coredocker.RemediationStep{
				Priority:    1,
				Action:      "update_base_image",
				Description: "Update base image to latest version to fix known vulnerabilities",
				Command:     "docker pull <base-image>:latest",
			})

			if scanResult.Summary.Fixable > 0 {
				scanResult.Remediation = append(scanResult.Remediation, coredocker.RemediationStep{
					Priority:    2,
					Action:      "update_packages",
					Description: fmt.Sprintf("Update %d packages with available fixes", scanResult.Summary.Fixable),
					Command:     "Update package versions in Dockerfile or run package manager update commands",
				})
			}
		}
	}

	duration := time.Since(startTime)
	bv.logger.Info().
		Dur("duration", duration).
		Interface("summary", scanResult.Summary).
		Msg("Security scan completed")

	return scanResult, duration, nil
}

// AddPushTroubleshootingTips adds troubleshooting tips for push failures
func (bv *BuildValidatorImpl) AddPushTroubleshootingTips(err error, registryURL string) []string {
	tips := []string{}

	errorMsg := err.Error()

	if strings.Contains(errorMsg, "authentication required") ||
		strings.Contains(errorMsg, "unauthorized") {
		tips = append(tips,
			"Authentication failed. Run: docker login "+registryURL,
			"Check if your credentials are correct",
			"For private registries, ensure you have push permissions")
	}

	if strings.Contains(errorMsg, "connection refused") ||
		strings.Contains(errorMsg, "no such host") {
		tips = append(tips,
			"Cannot connect to registry. Check if the registry URL is correct",
			"Verify network connectivity to "+registryURL,
			"If using a private registry, ensure it's accessible from your network")
	}

	if strings.Contains(errorMsg, "denied") {
		tips = append(tips,
			"Access denied. Verify you have push permissions to this repository",
			"Check if the repository exists and you have write access",
			"For organization repositories, ensure your account is properly configured")
	}

	return tips
}

// AddTroubleshootingTips adds general troubleshooting tips based on the error
func (bv *BuildValidatorImpl) AddTroubleshootingTips(err error) []string {
	tips := []string{}

	if err == nil {
		return tips
	}

	errorMsg := err.Error()

	// Docker daemon issues
	if strings.Contains(errorMsg, "Cannot connect to the Docker daemon") {
		tips = append(tips,
			"Ensure Docker Desktop is running",
			"Try: sudo systemctl start docker (Linux)",
			"Check Docker daemon logs for errors")
	}

	// Dockerfile syntax errors
	if strings.Contains(errorMsg, "failed to parse Dockerfile") ||
		strings.Contains(errorMsg, "unknown instruction") {
		tips = append(tips,
			"Check Dockerfile syntax",
			"Ensure all instructions are valid",
			"Verify proper line endings (LF, not CRLF)")
	}

	// Build context issues
	if strings.Contains(errorMsg, "no such file or directory") {
		tips = append(tips,
			"Verify all files referenced in Dockerfile exist",
			"Check if build context includes all necessary files",
			"Ensure relative paths are correct from build context")
	}

	// Network issues
	if strings.Contains(errorMsg, "temporary failure resolving") ||
		strings.Contains(errorMsg, "network is unreachable") {
		tips = append(tips,
			"Check internet connectivity",
			"Verify DNS settings",
			"Try using a different DNS server (e.g., 8.8.8.8)")
	}

	// Space issues
	if strings.Contains(errorMsg, "no space left on device") {
		tips = append(tips,
			"Free up disk space",
			"Run: docker system prune -a",
			"Check available space with: df -h")
	}

	return tips
}

// ValidateArgs validates the atomic build image arguments
func (bv *BuildValidatorImpl) ValidateArgs(args *AtomicBuildImageArgs) error {
	// Validate image name
	if args.ImageName == "" {
		return types.NewErrorBuilder("invalid_arguments", "image_name is required", "validation").
			WithSeverity("high").
			WithOperation("ValidateArgs").
			Build()
	}

	// Validate platform if specified
	if args.Platform != "" {
		validPlatforms := []string{"linux/amd64", "linux/arm64", "linux/arm/v7"}
		valid := false
		for _, p := range validPlatforms {
			if args.Platform == p {
				valid = true
				break
			}
		}
		if !valid {
			return types.NewErrorBuilder("invalid_arguments",
				fmt.Sprintf("invalid platform %s, must be one of: %v", args.Platform, validPlatforms), "validation").
				WithSeverity("high").
				WithOperation("ValidateArgs").
				WithField("platform", args.Platform).
				Build()
		}
	}

	// Validate registry URL if push is requested
	if args.PushAfterBuild && args.RegistryURL == "" {
		return types.NewErrorBuilder("invalid_arguments",
			"registry_url is required when push_after_build is true", "validation").
			WithSeverity("high").
			WithOperation("ValidateArgs").
			WithField("push_after_build", args.PushAfterBuild).
			Build()
	}

	return nil
}
