package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/errors"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"

	coresecurity "github.com/Azure/container-kit/pkg/core/security"
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
		return errors.Validationf("build_validator", "Dockerfile not found at %s", dockerfilePath)
	}
	// Check if build context exists
	if _, err := os.Stat(buildContext); os.IsNotExist(err) {
		return errors.Validationf("build_validator", "Build context directory not found at %s", buildContext)
	}
	// Check if Docker is available
	cmd := exec.Command("docker", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker is not available. Please ensure Docker is installed and running")
	}
	return nil
}

// Helper method to check if Trivy is installed
func (bv *BuildValidatorImpl) isTrivyInstalled() bool {
	cmd := exec.Command("trivy", "--version")
	return cmd.Run() == nil
}

// Helper method to execute Trivy scan
func (bv *BuildValidatorImpl) executeTrivyScan(ctx context.Context, fullImageRef string) ([]byte, error) {
	scanCmd := exec.CommandContext(ctx, "trivy", "image", "--format", "json", "--quiet", fullImageRef)
	output, err := scanCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			bv.logger.Warn().Str("stderr", string(exitErr.Stderr)).Msg("Trivy scan failed")
		}
		return nil, err
	}
	return output, nil
}

// Helper method to create scan error
func (bv *BuildValidatorImpl) createScanError(fullImageRef string) error {
	return fmt.Errorf("security scan failed for image %s", fullImageRef)
}

// Helper method to initialize scan result
func (bv *BuildValidatorImpl) initializeScanResult(fullImageRef string, startTime time.Time) *coredocker.ScanResult {
	return &coredocker.ScanResult{
		Success:  true,
		ImageRef: fullImageRef,
		ScanTime: time.Now(),
		Duration: time.Since(startTime),
		Summary: coresecurity.VulnerabilitySummary{
			Total:    0,
			Critical: 0,
			High:     0,
			Medium:   0,
			Low:      0,
			Unknown:  0,
			Fixable:  0,
		},
		Vulnerabilities: []coresecurity.Vulnerability{},
		Remediation:     []coresecurity.RemediationStep{},
		Context:         make(map[string]interface{}),
	}
}

// Helper method to count vulnerabilities from string output
func (bv *BuildValidatorImpl) countVulnerabilitiesFromString(outputStr string, scanResult *coredocker.ScanResult) {
	severityLevels := []struct {
		level string
		field *int
	}{
		{"CRITICAL", &scanResult.Summary.Critical},
		{"HIGH", &scanResult.Summary.High},
		{"MEDIUM", &scanResult.Summary.Medium},
		{"LOW", &scanResult.Summary.Low},
		{"UNKNOWN", &scanResult.Summary.Unknown},
	}
	for _, severity := range severityLevels {
		count := strings.Count(outputStr, severity.level)
		if count > 0 {
			*severity.field = count
			scanResult.Summary.Total += count
		}
	}
}

// Helper method to process JSON results
func (bv *BuildValidatorImpl) processJSONResults(trivyResult *coredocker.TrivyResult, scanResult *coredocker.ScanResult) {
	for _, result := range trivyResult.Results {
		for _, vuln := range result.Vulnerabilities {
			// Create vulnerability object
			vulnerability := coresecurity.Vulnerability{
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
}

// Helper method to add remediation steps
func (bv *BuildValidatorImpl) addRemediationSteps(scanResult *coredocker.ScanResult) {
	if scanResult.Summary.Critical == 0 && scanResult.Summary.High == 0 {
		return
	}
	scanResult.Remediation = append(scanResult.Remediation, coresecurity.RemediationStep{
		Priority:    1,
		Action:      "update_base_image",
		Description: "Update base image to latest version to fix known vulnerabilities",
		Command:     "docker pull <base-image>:latest",
	})
	if scanResult.Summary.Fixable > 0 {
		scanResult.Remediation = append(scanResult.Remediation, coresecurity.RemediationStep{
			Priority:    2,
			Action:      "update_packages",
			Description: fmt.Sprintf("Update %d packages with available fixes", scanResult.Summary.Fixable),
			Command:     "Update package versions in Dockerfile or run package manager update commands",
		})
	}
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
		return fmt.Errorf("image name is required")
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
			return errors.Validationf("build_validator", "invalid platform %s, must be one of: %v", args.Platform, validPlatforms)
		}
	}
	// Validate registry URL if push is requested
	if args.PushAfterBuild && args.RegistryURL == "" {
		return fmt.Errorf("registry URL is required when push_after_build is true")
	}
	return nil
}
