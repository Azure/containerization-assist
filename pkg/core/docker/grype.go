package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	mcperrors "github.com/Azure/container-kit/pkg/common/errors"
	coresecurity "github.com/Azure/container-kit/pkg/core/security"
	"github.com/rs/zerolog"
)

// GrypeScanner provides container image security scanning using Grype
type GrypeScanner struct {
	logger    zerolog.Logger
	grypePath string
}

// NewGrypeScanner creates a new Grype scanner
func NewGrypeScanner(logger zerolog.Logger) *GrypeScanner {
	return &GrypeScanner{
		logger: logger.With().Str("component", "grype_scanner").Logger(),
	}
}

// GrypeResult represents the raw JSON output from Grype
type GrypeResult struct {
	Matches []struct {
		Vulnerability struct {
			ID          string   `json:"id"`
			DataSource  string   `json:"dataSource"`
			Severity    string   `json:"severity"`
			URLs        []string `json:"urls"`
			Description string   `json:"description"`
			Fix         struct {
				Versions []string `json:"versions"`
				State    string   `json:"state"`
			} `json:"fix"`
		} `json:"vulnerability"`
		RelatedVulnerabilities []struct {
			ID          string   `json:"id"`
			DataSource  string   `json:"dataSource"`
			Severity    string   `json:"severity"`
			URLs        []string `json:"urls"`
			Description string   `json:"description"`
		} `json:"relatedVulnerabilities"`
		MatchDetails []struct {
			Type       string `json:"type"`
			Matcher    string `json:"matcher"`
			SearchedBy struct {
				Package struct {
					Name    string `json:"name"`
					Version string `json:"version"`
				} `json:"package"`
			} `json:"searchedBy"`
			Found struct {
				VersionConstraint string `json:"versionConstraint"`
			} `json:"found"`
		} `json:"matchDetails"`
		Artifact struct {
			Name      string `json:"name"`
			Version   string `json:"version"`
			Type      string `json:"type"`
			Locations []struct {
				Path        string `json:"path"`
				LayerID     string `json:"layerID"`
				LayerDiffID string `json:"layerDiffID"`
			} `json:"locations"`
			CPEs     []string    `json:"cpes"`
			PURL     string      `json:"purl"`
			Metadata interface{} `json:"metadata"`
		} `json:"artifact"`
	} `json:"matches"`
	Source struct {
		Type   string `json:"type"`
		Target struct {
			UserInput      string   `json:"userInput"`
			ImageID        string   `json:"imageID"`
			ManifestDigest string   `json:"manifestDigest"`
			MediaType      string   `json:"mediaType"`
			Tags           []string `json:"tags"`
			ImageSize      int64    `json:"imageSize"`
			Layers         []struct {
				MediaType string `json:"mediaType"`
				Digest    string `json:"digest"`
				Size      int64  `json:"size"`
			} `json:"layers"`
			RepoDigests []string `json:"repoDigests"`
		} `json:"target"`
	} `json:"source"`
	Descriptor struct {
		Name          string `json:"name"`
		Version       string `json:"version"`
		Configuration struct {
			ConfigPath string `json:"configPath"`
			DB         struct {
				Built         string      `json:"built"`
				SchemaVersion int         `json:"schemaVersion"`
				Location      string      `json:"location"`
				Checksum      string      `json:"checksum"`
				Error         interface{} `json:"error"`
			} `json:"db"`
			ExternalSources struct {
				Enable bool `json:"enable"`
				Maven  struct {
					SearchUpstreamBySha1 bool   `json:"searchUpstreamBySha1"`
					BaseURL              string `json:"baseURL"`
				} `json:"maven"`
			} `json:"externalSources"`
			Match struct {
				Java struct {
					UseCPEs bool `json:"useCPEs"`
				} `json:"java"`
				Python struct {
					UseCPEs bool `json:"useCPEs"`
				} `json:"python"`
				Javascript struct {
					UseCPEs bool `json:"useCPEs"`
				} `json:"javascript"`
				Ruby struct {
					UseCPEs bool `json:"useCPEs"`
				} `json:"ruby"`
				Rust struct {
					UseCPEs bool `json:"useCPEs"`
				} `json:"rust"`
				Golang struct {
					UseCPEs bool `json:"useCPEs"`
				} `json:"golang"`
			} `json:"match"`
		} `json:"configuration"`
	} `json:"descriptor"`
}

// ScanImage scans a Docker image for vulnerabilities using Grype
func (gs *GrypeScanner) ScanImage(ctx context.Context, imageRef string, severityThreshold string) (*ScanResult, error) {
	// Check if Grype is installed
	grypePath, err := gs.findGrype()
	if err != nil {
		gs.logger.Warn().Err(err).Msg("Grype not found")
		return nil, mcperrors.New(mcperrors.CodeInternalError, "core", "grype not available", err)
	}
	gs.grypePath = grypePath

	startTime := time.Now()
	result := &ScanResult{
		ImageRef:        imageRef,
		ScanTime:        startTime,
		Vulnerabilities: make([]coresecurity.Vulnerability, 0),
		Context:         make(map[string]interface{}),
		Remediation:     make([]coresecurity.RemediationStep, 0),
	}

	gs.logger.Info().
		Str("image", imageRef).
		Str("severity_threshold", severityThreshold).
		Msg("Starting Grype security scan")

	// Update vulnerability database first
	if err := gs.updateDB(ctx); err != nil {
		gs.logger.Warn().Err(err).Msg("Failed to update Grype DB, using existing database")
	}

	// Run Grype scan with JSON output
	args := []string{
		imageRef,
		"-o", "json",
		"--quiet",
	}

	// Add severity filter if specified
	if severityThreshold != "" {
		args = append(args, "--fail-on", severityThreshold)
	}

	// nolint:gosec // grype path is validated and args are controlled
	cmd := exec.CommandContext(ctx, gs.grypePath, args...)
	output, err := cmd.Output()

	result.Duration = time.Since(startTime)
	result.Context["grype_version"] = gs.getGrypeVersion()
	result.Context["scan_duration_ms"] = result.Duration.Milliseconds()
	result.Context["scanner"] = "grype"

	if err != nil {
		// Check if it's just because vulnerabilities were found
		if exitErr, ok := err.(*exec.ExitError); ok && len(output) > 0 {
			// Grype returns non-zero exit code when vulnerabilities matching threshold are found
			gs.logger.Debug().Int("exit_code", exitErr.ExitCode()).Msg("Grype found vulnerabilities")
		} else {
			return result, mcperrors.New(mcperrors.CodeOperationFailed, "docker", "grype scan failed", err)
		}
	}

	var grypeResult GrypeResult
	if err := json.Unmarshal(output, &grypeResult); err != nil {
		return result, mcperrors.New(mcperrors.CodeInternalError, "docker", "failed to parse grype output", err)
	}

	if grypeResult.Source.Target.ImageID != "" {
		result.Context["image_id"] = grypeResult.Source.Target.ImageID
	}
	if grypeResult.Source.Target.ImageSize > 0 {
		result.Context["image_size_mb"] = grypeResult.Source.Target.ImageSize / (1024 * 1024)
	}
	if grypeResult.Descriptor.Configuration.DB.Built != "" {
		result.Context["db_built"] = grypeResult.Descriptor.Configuration.DB.Built
	}

	// Convert Grype results to our format
	gs.processResults(&grypeResult, result)

	// Generate remediation steps
	gs.generateRemediationSteps(result)

	result.Success = result.Summary.Critical == 0 && result.Summary.High == 0

	gs.logger.Info().
		Bool("success", result.Success).
		Int("total_vulnerabilities", result.Summary.Total).
		Int("critical", result.Summary.Critical).
		Int("high", result.Summary.High).
		Dur("duration", result.Duration).
		Msg("Grype scan completed")

	return result, nil
}

// findGrype locates the Grype executable
func (gs *GrypeScanner) findGrype() (string, error) {
	// Check common locations
	paths := []string{
		"grype",
		"/usr/local/bin/grype",
		"/usr/bin/grype",
		"/opt/grype/grype",
	}

	for _, path := range paths {
		if p, err := exec.LookPath(path); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("grype executable not found in PATH")
}

// updateDB updates the Grype vulnerability database
func (gs *GrypeScanner) updateDB(ctx context.Context) error {
	gs.logger.Debug().Msg("Updating Grype vulnerability database")

	// nolint:gosec // grype path is validated
	cmd := exec.CommandContext(ctx, gs.grypePath, "db", "update")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update grype db: %w (output: %s)", err, string(output))
	}

	gs.logger.Debug().Msg("Grype vulnerability database updated successfully")
	return nil
}

// getGrypeVersion gets the Grype version
func (gs *GrypeScanner) getGrypeVersion() string {
	if gs.grypePath == "" {
		return "unknown"
	}

	// nolint:gosec // grype path is validated
	cmd := exec.Command(gs.grypePath, "version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	// Parse version from output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Version:") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "Version:" && i+1 < len(parts) {
					return parts[i+1]
				}
			}
		}
	}

	return "unknown"
}

// processResults converts Grype results to our format
func (gs *GrypeScanner) processResults(grypeResult *GrypeResult, scanResult *ScanResult) {
	summary := &scanResult.Summary

	// Track unique vulnerabilities to avoid duplicates
	seen := make(map[string]bool)

	for _, match := range grypeResult.Matches {
		vuln := match.Vulnerability
		artifact := match.Artifact

		// Create unique key for deduplication
		key := fmt.Sprintf("%s-%s-%s", vuln.ID, artifact.Name, artifact.Version)
		if seen[key] {
			continue
		}
		seen[key] = true

		// Extract fixed version if available
		fixedVersion := ""
		if vuln.Fix.State == "fixed" && len(vuln.Fix.Versions) > 0 {
			fixedVersion = vuln.Fix.Versions[0]
		}

		// Extract layer information
		layer := ""
		if len(artifact.Locations) > 0 && artifact.Locations[0].LayerDiffID != "" {
			layer = artifact.Locations[0].LayerDiffID
		}

		// Convert to our vulnerability format
		v := coresecurity.Vulnerability{
			VulnerabilityID:  vuln.ID,
			PkgName:          artifact.Name,
			InstalledVersion: artifact.Version,
			FixedVersion:     fixedVersion,
			Severity:         strings.ToUpper(vuln.Severity),
			Title:            fmt.Sprintf("%s vulnerability in %s", vuln.ID, artifact.Name),
			Description:      vuln.Description,
			References:       vuln.URLs,
			Layer:            layer,
		}

		scanResult.Vulnerabilities = append(scanResult.Vulnerabilities, v)

		// Update summary counts
		summary.Total++
		if fixedVersion != "" {
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
		case "NEGLIGIBLE":
			summary.Low++ // Map negligible to low
		default:
			summary.Unknown++
		}
	}
}

// generateRemediationSteps creates actionable remediation guidance
func (gs *GrypeScanner) generateRemediationSteps(result *ScanResult) {
	if result.Summary.Total == 0 {
		result.Remediation = append(result.Remediation, coresecurity.RemediationStep{
			Priority:    1,
			Action:      "No action required",
			Description: "No vulnerabilities found in the image by Grype",
		})
		return
	}

	priority := 1

	// Critical and High vulnerabilities
	if result.Summary.Critical > 0 || result.Summary.High > 0 {
		result.Remediation = append(result.Remediation, coresecurity.RemediationStep{
			Priority: priority,
			Action:   "Fix critical vulnerabilities",
			Description: fmt.Sprintf("Found %d CRITICAL and %d HIGH severity vulnerabilities that must be fixed",
				result.Summary.Critical, result.Summary.High),
		})
		priority++

		// Check for OS package vulnerabilities
		osPackageCount := 0
		appPackageCount := 0
		for _, vuln := range result.Vulnerabilities {
			if vuln.Severity == "CRITICAL" || vuln.Severity == "HIGH" {
				// Simple heuristic: OS packages often have certain patterns
				if strings.Contains(vuln.PkgName, "lib") ||
					strings.Contains(vuln.PkgName, "-base") ||
					strings.Contains(vuln.PkgName, "openssl") ||
					strings.Contains(vuln.PkgName, "glibc") {
					osPackageCount++
				} else {
					appPackageCount++
				}
			}
		}

		if osPackageCount > 0 {
			result.Remediation = append(result.Remediation, coresecurity.RemediationStep{
				Priority:    priority,
				Action:      "Update base image",
				Description: fmt.Sprintf("%d vulnerabilities appear to be in OS packages. Update your base image", osPackageCount),
				Command:     "docker pull <base-image>:latest",
			})
			priority++
		}

		if appPackageCount > 0 {
			result.Remediation = append(result.Remediation, coresecurity.RemediationStep{
				Priority:    priority,
				Action:      "Update application dependencies",
				Description: fmt.Sprintf("%d vulnerabilities are in application packages", appPackageCount),
			})
			priority++
		}
	}

	// Fixable vulnerabilities
	if result.Summary.Fixable > 0 {
		result.Remediation = append(result.Remediation, coresecurity.RemediationStep{
			Priority: priority,
			Action:   "Apply available fixes",
			Description: fmt.Sprintf("%d vulnerabilities have fixes available. Update packages in your Dockerfile",
				result.Summary.Fixable),
			Command: "RUN apt-get update && apt-get upgrade -y && rm -rf /var/lib/apt/lists/*",
		})
		priority++
	}

	// Grype-specific recommendations
	result.Remediation = append(result.Remediation, coresecurity.RemediationStep{
		Priority:    priority,
		Action:      "Cross-reference with multiple scanners",
		Description: "Use both Grype and Trivy for comprehensive vulnerability detection",
	})
}

// CheckGrypeInstalled checks if Grype is available
func (gs *GrypeScanner) CheckGrypeInstalled() bool {
	_, err := gs.findGrype()
	return err == nil
}

// InstallGrype provides installation instructions for Grype
func (gs *GrypeScanner) InstallGrype() string {
	return `To install Grype:

# Using curl:
curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin

# Using Homebrew (macOS/Linux):
brew install grype

# Using Go:
go install github.com/anchore/grype@latest

# Verify installation:
grype version`
}
