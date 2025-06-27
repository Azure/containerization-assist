package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	coresecurity "github.com/Azure/container-kit/pkg/core/security"
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
	Success         bool                              `json:"success"`
	ImageRef        string                            `json:"image_ref"`
	ScanTime        time.Time                         `json:"scan_time"`
	Duration        time.Duration                     `json:"duration"`
	Vulnerabilities []coresecurity.Vulnerability      `json:"vulnerabilities"`
	Summary         coresecurity.VulnerabilitySummary `json:"summary"`
	Remediation     []coresecurity.RemediationStep    `json:"remediation"`
	Context         map[string]interface{}            `json:"context"`
}

// TrivyResult represents the raw JSON output from Trivy
type TrivyResult struct {
	SchemaVersion int `json:"SchemaVersion"`
	Results       []struct {
		Target   string `json:"Target"`
		Class    string `json:"Class,omitempty"`
		Type     string `json:"Type,omitempty"`
		Packages []struct {
			ID         string   `json:"ID"`
			Name       string   `json:"Name"`
			Version    string   `json:"Version"`
			Arch       string   `json:"Arch,omitempty"`
			SrcName    string   `json:"SrcName,omitempty"`
			SrcVersion string   `json:"SrcVersion,omitempty"`
			Licenses   []string `json:"Licenses,omitempty"`
			DependsOn  []string `json:"DependsOn,omitempty"`
			Layer      struct {
				Digest string `json:"Digest"`
				DiffID string `json:"DiffID"`
			} `json:"Layer,omitempty"`
			FilePath string `json:"FilePath,omitempty"`
		} `json:"Packages,omitempty"`
		Vulnerabilities []struct {
			VulnerabilityID  string   `json:"VulnerabilityID"`
			PkgID            string   `json:"PkgID,omitempty"`
			PkgName          string   `json:"PkgName"`
			InstalledVersion string   `json:"InstalledVersion"`
			FixedVersion     string   `json:"FixedVersion,omitempty"`
			Status           string   `json:"Status,omitempty"`
			Severity         string   `json:"Severity"`
			Title            string   `json:"Title"`
			Description      string   `json:"Description"`
			References       []string `json:"References,omitempty"`
			PublishedDate    string   `json:"PublishedDate,omitempty"`
			LastModifiedDate string   `json:"LastModifiedDate,omitempty"`
			CweIDs           []string `json:"CweIDs,omitempty"`
			CVSS             map[string]struct {
				V2Vector string  `json:"V2Vector,omitempty"`
				V3Vector string  `json:"V3Vector,omitempty"`
				V2Score  float64 `json:"V2Score,omitempty"`
				V3Score  float64 `json:"V3Score,omitempty"`
			} `json:"CVSS,omitempty"`
			VendorSeverity map[string]interface{} `json:"VendorSeverity,omitempty"`
			CVSSV30        struct {
				Vector                string  `json:"vectorString,omitempty"`
				BaseScore             float64 `json:"baseScore,omitempty"`
				ExploitabilityScore   float64 `json:"exploitabilityScore,omitempty"`
				ImpactScore           float64 `json:"impactScore,omitempty"`
				AttackVector          string  `json:"attackVector,omitempty"`
				AttackComplexity      string  `json:"attackComplexity,omitempty"`
				PrivilegesRequired    string  `json:"privilegesRequired,omitempty"`
				UserInteraction       string  `json:"userInteraction,omitempty"`
				Scope                 string  `json:"scope,omitempty"`
				ConfidentialityImpact string  `json:"confidentialityImpact,omitempty"`
				IntegrityImpact       string  `json:"integrityImpact,omitempty"`
				AvailabilityImpact    string  `json:"availabilityImpact,omitempty"`
			} `json:"CVSS:3.0,omitempty"`
			CVSSV31 struct {
				Vector                string  `json:"vectorString,omitempty"`
				BaseScore             float64 `json:"baseScore,omitempty"`
				ExploitabilityScore   float64 `json:"exploitabilityScore,omitempty"`
				ImpactScore           float64 `json:"impactScore,omitempty"`
				AttackVector          string  `json:"attackVector,omitempty"`
				AttackComplexity      string  `json:"attackComplexity,omitempty"`
				PrivilegesRequired    string  `json:"privilegesRequired,omitempty"`
				UserInteraction       string  `json:"userInteraction,omitempty"`
				Scope                 string  `json:"scope,omitempty"`
				ConfidentialityImpact string  `json:"confidentialityImpact,omitempty"`
				IntegrityImpact       string  `json:"integrityImpact,omitempty"`
				AvailabilityImpact    string  `json:"availabilityImpact,omitempty"`
			} `json:"CVSS:3.1,omitempty"`
			DataSource struct {
				ID   string `json:"ID,omitempty"`
				Name string `json:"Name,omitempty"`
				URL  string `json:"URL,omitempty"`
			} `json:"DataSource,omitempty"`
			Layer struct {
				Digest string `json:"Digest"`
				DiffID string `json:"DiffID"`
			} `json:"Layer,omitempty"`
			PkgPath    string `json:"PkgPath,omitempty"`
			PrimaryURL string `json:"PrimaryURL,omitempty"`
		} `json:"Vulnerabilities"`
	} `json:"Results"`
	Metadata struct {
		OS struct {
			Family string `json:"Family"`
			Name   string `json:"Name"`
		} `json:"OS,omitempty"`
		ImageID     string   `json:"ImageID,omitempty"`
		DiffIDs     []string `json:"DiffIDs,omitempty"`
		RepoTags    []string `json:"RepoTags,omitempty"`
		RepoDigests []string `json:"RepoDigests,omitempty"`
		ImageConfig struct {
			Architecture string `json:"architecture,omitempty"`
			Created      string `json:"created,omitempty"`
			History      []struct {
				Created    string `json:"created,omitempty"`
				CreatedBy  string `json:"created_by,omitempty"`
				EmptyLayer bool   `json:"empty_layer,omitempty"`
			} `json:"history,omitempty"`
			OS     string `json:"os,omitempty"`
			RootFS struct {
				Type    string   `json:"type,omitempty"`
				DiffIDs []string `json:"diff_ids,omitempty"`
			} `json:"rootfs,omitempty"`
			Config struct {
				Env        []string `json:"Env,omitempty"`
				Cmd        []string `json:"Cmd,omitempty"`
				WorkingDir string   `json:"WorkingDir,omitempty"`
				Entrypoint []string `json:"Entrypoint,omitempty"`
			} `json:"config,omitempty"`
		} `json:"ImageConfig,omitempty"`
	} `json:"Metadata,omitempty"`
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
		Vulnerabilities: make([]coresecurity.Vulnerability, 0),
		Context:         make(map[string]interface{}),
		Remediation:     make([]coresecurity.RemediationStep, 0),
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

	// nolint:gosec // trivy path is validated and args are controlled
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

	// nolint:gosec // trivy path is validated
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

	// Store metadata if available
	if trivyResult.Metadata.ImageID != "" {
		scanResult.Context["image_id"] = trivyResult.Metadata.ImageID
	}
	if trivyResult.Metadata.OS.Family != "" {
		scanResult.Context["os_family"] = trivyResult.Metadata.OS.Family
		scanResult.Context["os_name"] = trivyResult.Metadata.OS.Name
	}
	if len(trivyResult.Metadata.RepoTags) > 0 {
		scanResult.Context["repo_tags"] = trivyResult.Metadata.RepoTags
	}
	if trivyResult.SchemaVersion > 0 {
		scanResult.Context["schema_version"] = trivyResult.SchemaVersion
	}

	// Create package lookup map for enhanced data
	packageMap := make(map[string]interface{})
	for _, result := range trivyResult.Results {
		for _, pkg := range result.Packages {
			packageMap[pkg.ID] = pkg
		}
	}

	for _, result := range trivyResult.Results {
		for _, vuln := range result.Vulnerabilities {
			// Convert to our vulnerability format with enhanced data
			v := coresecurity.Vulnerability{
				VulnerabilityID:  vuln.VulnerabilityID,
				PkgName:          vuln.PkgName,
				PkgID:            vuln.PkgID,
				PkgPath:          vuln.PkgPath,
				InstalledVersion: vuln.InstalledVersion,
				FixedVersion:     vuln.FixedVersion,
				Severity:         vuln.Severity,
				Title:            vuln.Title,
				Description:      vuln.Description,
				References:       vuln.References,
				PublishedDate:    vuln.PublishedDate,
				LastModifiedDate: vuln.LastModifiedDate,
				CWE:              vuln.CweIDs,
				Status:           vuln.Status,
				PrimaryURL:       vuln.PrimaryURL,
			}

			// Extract layer information
			if vuln.Layer.DiffID != "" {
				v.Layer = vuln.Layer.DiffID
			}

			// Extract CVSS information
			if len(vuln.CVSS) > 0 {
				for vendor, cvssData := range vuln.CVSS {
					if cvssData.V3Score > 0 {
						v.CVSS = coresecurity.CVSSInfo{
							Version: "3.0",
							Vector:  cvssData.V3Vector,
							Score:   cvssData.V3Score,
						}
						break
					} else if cvssData.V2Score > 0 {
						v.CVSS = coresecurity.CVSSInfo{
							Version: "2.0",
							Vector:  cvssData.V2Vector,
							Score:   cvssData.V2Score,
						}
					}
					// Store vendor for reference
					if v.VendorSeverity == nil {
						v.VendorSeverity = make(map[string]string)
					}
					if severityStr, ok := vuln.VendorSeverity[vendor].(string); ok {
						v.VendorSeverity[vendor] = severityStr
					}
				}
			}

			// Extract detailed CVSS v3 information (prefer 3.1 over 3.0)
			if vuln.CVSSV31.BaseScore > 0 {
				v.CVSSV3 = coresecurity.CVSSV3Info{
					Vector:                vuln.CVSSV31.Vector,
					Score:                 vuln.CVSSV31.BaseScore,
					ExploitabilityScore:   vuln.CVSSV31.ExploitabilityScore,
					ImpactScore:           vuln.CVSSV31.ImpactScore,
					AttackVector:          vuln.CVSSV31.AttackVector,
					AttackComplexity:      vuln.CVSSV31.AttackComplexity,
					PrivilegesRequired:    vuln.CVSSV31.PrivilegesRequired,
					UserInteraction:       vuln.CVSSV31.UserInteraction,
					Scope:                 vuln.CVSSV31.Scope,
					ConfidentialityImpact: vuln.CVSSV31.ConfidentialityImpact,
					IntegrityImpact:       vuln.CVSSV31.IntegrityImpact,
					AvailabilityImpact:    vuln.CVSSV31.AvailabilityImpact,
				}
			} else if vuln.CVSSV30.BaseScore > 0 {
				v.CVSSV3 = coresecurity.CVSSV3Info{
					Vector:                vuln.CVSSV30.Vector,
					Score:                 vuln.CVSSV30.BaseScore,
					ExploitabilityScore:   vuln.CVSSV30.ExploitabilityScore,
					ImpactScore:           vuln.CVSSV30.ImpactScore,
					AttackVector:          vuln.CVSSV30.AttackVector,
					AttackComplexity:      vuln.CVSSV30.AttackComplexity,
					PrivilegesRequired:    vuln.CVSSV30.PrivilegesRequired,
					UserInteraction:       vuln.CVSSV30.UserInteraction,
					Scope:                 vuln.CVSSV30.Scope,
					ConfidentialityImpact: vuln.CVSSV30.ConfidentialityImpact,
					IntegrityImpact:       vuln.CVSSV30.IntegrityImpact,
					AvailabilityImpact:    vuln.CVSSV30.AvailabilityImpact,
				}
			}

			// Extract data source information
			if vuln.DataSource.ID != "" {
				v.DataSource = coresecurity.VulnDataSource{
					ID:   vuln.DataSource.ID,
					Name: vuln.DataSource.Name,
					URL:  vuln.DataSource.URL,
				}
			}

			// Enhanced package information from package map
			if _, exists := packageMap[vuln.PkgID]; exists {
				// Use reflection or type assertion to extract package data
				// For now, store basic package type
				v.PkgType = result.Type
				if v.PkgIdentifier.Licenses == nil {
					v.PkgIdentifier.Licenses = make([]string, 0)
				}
				// Additional package metadata can be added here
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

	// Store enhanced context information
	scanResult.Context["trivy_schema_version"] = trivyResult.SchemaVersion
	scanResult.Context["total_results"] = len(trivyResult.Results)
}

// generateRemediationSteps creates actionable remediation guidance
func (ts *TrivyScanner) generateRemediationSteps(result *ScanResult) {
	if result.Summary.Total == 0 {
		result.Remediation = append(result.Remediation, coresecurity.RemediationStep{
			Priority:    1,
			Action:      "No action required",
			Description: "No vulnerabilities found in the image",
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
			result.Remediation = append(result.Remediation, coresecurity.RemediationStep{
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
		result.Remediation = append(result.Remediation, coresecurity.RemediationStep{
			Priority: priority,
			Action:   "Update packages",
			Description: fmt.Sprintf("%d vulnerabilities have fixes available. Update packages in your Dockerfile",
				result.Summary.Fixable),
			Command: "RUN apt-get update && apt-get upgrade -y && rm -rf /var/lib/apt/lists/*",
		})
		priority++
	}

	// General recommendations
	result.Remediation = append(result.Remediation, coresecurity.RemediationStep{
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
