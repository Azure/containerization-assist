package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// TrivyScanner provides container image security scanning using Trivy
type TrivyScanner struct {
	logger    zerolog.Logger
	trivyPath string
}

// NewTrivyScanner creates a new Trivy scanner
func NewTrivyScanner(logger zerolog.Logger) *TrivyScanner {
	return &TrivyScanner{
		logger: logger.With().Str("component", "trivy_scanner").Logger(),
	}
}

// ScanResult represents the result of a Trivy scan
type ScanResult struct {
	Success         bool                   `json:"success"`
	ImageRef        string                 `json:"image_ref"`
	ScanTime        time.Time              `json:"scan_time"`
	Duration        time.Duration          `json:"duration"`
	Vulnerabilities []Vulnerability        `json:"vulnerabilities"`
	Summary         VulnerabilitySummary   `json:"summary"`
	Remediation     []RemediationStep      `json:"remediation"`
	Context         map[string]interface{} `json:"context"`
}

// Vulnerability represents a single vulnerability finding
type Vulnerability struct {
	VulnerabilityID  string   `json:"vulnerability_id"`
	PkgName          string   `json:"pkg_name"`
	InstalledVersion string   `json:"installed_version"`
	FixedVersion     string   `json:"fixed_version"`
	Severity         string   `json:"severity"` // CRITICAL, HIGH, MEDIUM, LOW, UNKNOWN
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	References       []string `json:"references"`
	Layer            string   `json:"layer,omitempty"`
}

// VulnerabilitySummary provides a summary of scan findings
type VulnerabilitySummary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Unknown  int `json:"unknown"`
	Fixable  int `json:"fixable"`
}

// RemediationStep provides guidance on fixing vulnerabilities
type RemediationStep struct {
	Priority    int    `json:"priority"`
	Action      string `json:"action"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
}

// TrivyResult represents the raw JSON output from Trivy
type TrivyResult struct {
	Results []struct {
		Target          string `json:"Target"`
		Vulnerabilities []struct {
			VulnerabilityID  string   `json:"VulnerabilityID"`
			PkgName          string   `json:"PkgName"`
			InstalledVersion string   `json:"InstalledVersion"`
			FixedVersion     string   `json:"FixedVersion,omitempty"`
			Severity         string   `json:"Severity"`
			Title            string   `json:"Title"`
			Description      string   `json:"Description"`
			References       []string `json:"References,omitempty"`
			Layer            struct {
				DiffID string `json:"DiffID"`
			} `json:"Layer,omitempty"`
		} `json:"Vulnerabilities"`
	} `json:"Results"`
}

// ScanImage scans a Docker image for vulnerabilities using Trivy
func (ts *TrivyScanner) ScanImage(ctx context.Context, imageRef string, severityThreshold string) (*ScanResult, error) {
	// Check if Trivy is installed
	trivyPath, err := ts.findTrivy()
	if err != nil {
		ts.logger.Warn().Err(err).Msg("Trivy not found")
		return nil, fmt.Errorf("trivy not available: %w", err)
	}
	ts.trivyPath = trivyPath

	startTime := time.Now()
	result := &ScanResult{
		ImageRef:        imageRef,
		ScanTime:        startTime,
		Vulnerabilities: make([]Vulnerability, 0),
		Context:         make(map[string]interface{}),
		Remediation:     make([]RemediationStep, 0),
	}

	ts.logger.Info().
		Str("image", imageRef).
		Str("severity_threshold", severityThreshold).
		Msg("Starting Trivy security scan")

	// Run Trivy scan with JSON output
	args := []string{
		"image",
		"--format", "json",
		"--quiet",
		"--no-progress",
		imageRef,
	}

	// Add severity filter if specified
	if severityThreshold != "" {
		args = append(args, "--severity", severityThreshold)
	}

	cmd := exec.CommandContext(ctx, ts.trivyPath, args...)
	output, err := cmd.Output()

	result.Duration = time.Since(startTime)
	result.Context["trivy_version"] = ts.getTrivyVersion()
	result.Context["scan_duration_ms"] = result.Duration.Milliseconds()

	if err != nil {
		// Check if it's just because vulnerabilities were found (exit code 1)
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 && len(output) > 0 {
			// This is normal when vulnerabilities are found
			ts.logger.Debug().Msg("Trivy found vulnerabilities (exit code 1)")
		} else {
			return result, fmt.Errorf("trivy scan failed: %w", err)
		}
	}

	// Parse Trivy JSON output
	var trivyResult TrivyResult
	if err := json.Unmarshal(output, &trivyResult); err != nil {
		return result, fmt.Errorf("failed to parse trivy output: %w", err)
	}

	// Convert Trivy results to our format
	ts.processResults(&trivyResult, result)

	// Generate remediation steps
	ts.generateRemediationSteps(result)

	result.Success = result.Summary.Critical == 0 && result.Summary.High == 0

	ts.logger.Info().
		Bool("success", result.Success).
		Int("total_vulnerabilities", result.Summary.Total).
		Int("critical", result.Summary.Critical).
		Int("high", result.Summary.High).
		Dur("duration", result.Duration).
		Msg("Trivy scan completed")

	return result, nil
}

// findTrivy locates the Trivy executable
func (ts *TrivyScanner) findTrivy() (string, error) {
	// Check common locations
	paths := []string{
		"trivy",
		"/usr/local/bin/trivy",
		"/usr/bin/trivy",
		"/opt/trivy/trivy",
	}

	for _, path := range paths {
		if p, err := exec.LookPath(path); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("trivy executable not found in PATH")
}

// getTrivyVersion gets the Trivy version
func (ts *TrivyScanner) getTrivyVersion() string {
	if ts.trivyPath == "" {
		return "unknown"
	}

	cmd := exec.Command(ts.trivyPath, "version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	// Parse version from output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Version:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}

	return "unknown"
}

// processResults converts Trivy results to our format
func (ts *TrivyScanner) processResults(trivyResult *TrivyResult, scanResult *ScanResult) {
	summary := &scanResult.Summary

	for _, result := range trivyResult.Results {
		for _, vuln := range result.Vulnerabilities {
			// Convert to our vulnerability format
			v := Vulnerability{
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
				v.Layer = vuln.Layer.DiffID
			}

			scanResult.Vulnerabilities = append(scanResult.Vulnerabilities, v)

			// Update summary counts
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
	}
}

// generateRemediationSteps creates actionable remediation guidance
func (ts *TrivyScanner) generateRemediationSteps(result *ScanResult) {
	if result.Summary.Total == 0 {
		result.Remediation = append(result.Remediation, RemediationStep{
			Priority:    1,
			Action:      "No action required",
			Description: "No vulnerabilities found in the image",
		})
		return
	}

	priority := 1

	// Critical and High vulnerabilities
	if result.Summary.Critical > 0 || result.Summary.High > 0 {
		result.Remediation = append(result.Remediation, RemediationStep{
			Priority: priority,
			Action:   "Fix critical vulnerabilities",
			Description: fmt.Sprintf("Found %d CRITICAL and %d HIGH severity vulnerabilities that must be fixed",
				result.Summary.Critical, result.Summary.High),
		})
		priority++

		// Check for base image updates - look for packages that are typically part of base images
		hasBaseImageVulns := false
		baseImagePkgs := []string{"base-image", "alpine-base", "ubuntu-base", "centos-base"}
		for _, vuln := range result.Vulnerabilities {
			if vuln.Severity == "CRITICAL" || vuln.Severity == "HIGH" {
				for _, basePkg := range baseImagePkgs {
					if strings.Contains(vuln.PkgName, basePkg) {
						hasBaseImageVulns = true
						break
					}
				}
				if hasBaseImageVulns {
					break
				}
			}
		}

		if hasBaseImageVulns {
			result.Remediation = append(result.Remediation, RemediationStep{
				Priority:    priority,
				Action:      "Update base image",
				Description: "Many vulnerabilities come from the base image. Consider updating to the latest version",
				Command:     "docker pull <base-image>:latest",
			})
			priority++
		}
	}

	// Fixable vulnerabilities
	if result.Summary.Fixable > 0 {
		result.Remediation = append(result.Remediation, RemediationStep{
			Priority: priority,
			Action:   "Update packages",
			Description: fmt.Sprintf("%d vulnerabilities have fixes available. Update packages in your Dockerfile",
				result.Summary.Fixable),
			Command: "RUN apt-get update && apt-get upgrade -y && rm -rf /var/lib/apt/lists/*",
		})
		priority++
	}

	// General recommendations
	result.Remediation = append(result.Remediation, RemediationStep{
		Priority:    priority,
		Action:      "Regular scanning",
		Description: "Scan images regularly as new vulnerabilities are discovered daily",
	})
}

// CheckTrivyInstalled checks if Trivy is available
func (ts *TrivyScanner) CheckTrivyInstalled() bool {
	_, err := ts.findTrivy()
	return err == nil
}

// FormatScanSummary formats scan results for display
func (ts *TrivyScanner) FormatScanSummary(result *ScanResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Security Scan Results for %s:\n", result.ImageRef))
	sb.WriteString(fmt.Sprintf("Scan completed in %v\n\n", result.Duration.Round(time.Millisecond)))

	// Summary
	sb.WriteString("Vulnerability Summary:\n")
	sb.WriteString(fmt.Sprintf("  CRITICAL: %d\n", result.Summary.Critical))
	sb.WriteString(fmt.Sprintf("  HIGH:     %d\n", result.Summary.High))
	sb.WriteString(fmt.Sprintf("  MEDIUM:   %d\n", result.Summary.Medium))
	sb.WriteString(fmt.Sprintf("  LOW:      %d\n", result.Summary.Low))
	sb.WriteString(fmt.Sprintf("  TOTAL:    %d (Fixable: %d)\n", result.Summary.Total, result.Summary.Fixable))

	// Status
	if result.Success {
		sb.WriteString("\n✅ Image passed security requirements\n")
	} else {
		sb.WriteString(fmt.Sprintf("\n❌ Image has %d CRITICAL and %d HIGH severity vulnerabilities\n",
			result.Summary.Critical, result.Summary.High))
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
