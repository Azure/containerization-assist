package container

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/containerization-assist/pkg/domain/errors"
	coresecurity "github.com/Azure/containerization-assist/pkg/infrastructure/security"
	"github.com/rs/zerolog"
)

// TrivyScanner provides container image security scanning using Trivy
type TrivyScanner struct {
	logger    zerolog.Logger
	trivyPath string
}

// Vulnerability represents a general security vulnerability
type Vulnerability struct {
	ID          string                 `json:"id"`
	Severity    string                 `json:"severity"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Package     string                 `json:"package"`
	Version     string                 `json:"version"`
	FixedIn     string                 `json:"fixed_in,omitempty"`
	CVSS        map[string]interface{} `json:"cvss,omitempty"`
	References  []string               `json:"references,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// TrivyVulnerability represents a vulnerability from Trivy scan results
type TrivyVulnerability = struct {
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
	CVSSV30        TrivyCVSSV3            `json:"CVSS:3.0,omitempty"`
	CVSSV31        TrivyCVSSV3            `json:"CVSS:3.1,omitempty"`
	DataSource     TrivyDataSource        `json:"DataSource,omitempty"`
	Layer          TrivyLayer             `json:"Layer,omitempty"`
	PkgPath        string                 `json:"PkgPath,omitempty"`
	PrimaryURL     string                 `json:"PrimaryURL,omitempty"`
}

// TrivyCVSSV3 represents CVSS v3 data
type TrivyCVSSV3 = struct {
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
}

// TrivyDataSource represents data source information
type TrivyDataSource = struct {
	ID   string `json:"ID,omitempty"`
	Name string `json:"Name,omitempty"`
	URL  string `json:"URL,omitempty"`
}

// TrivyLayer represents layer information
type TrivyLayer = struct {
	Digest string `json:"Digest"`
	DiffID string `json:"DiffID"`
}

// TrivyResultEntry represents a single result entry from Trivy scan
type TrivyResultEntry = struct {
	Target          string               `json:"Target"`
	Class           string               `json:"Class,omitempty"`
	Type            string               `json:"Type,omitempty"`
	Packages        []TrivyPackage       `json:"Packages,omitempty"`
	Vulnerabilities []TrivyVulnerability `json:"Vulnerabilities"`
}

// TrivyPackage represents package information
type TrivyPackage = struct {
	ID         string     `json:"ID"`
	Name       string     `json:"Name"`
	Version    string     `json:"Version"`
	Arch       string     `json:"Arch,omitempty"`
	SrcName    string     `json:"SrcName,omitempty"`
	SrcVersion string     `json:"SrcVersion,omitempty"`
	Licenses   []string   `json:"Licenses,omitempty"`
	DependsOn  []string   `json:"DependsOn,omitempty"`
	Layer      TrivyLayer `json:"Layer,omitempty"`
	FilePath   string     `json:"FilePath,omitempty"`
}

// NewTrivyScanner creates a new Trivy scanner
func NewTrivyScanner(logger zerolog.Logger) *TrivyScanner {
	return &TrivyScanner{}
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
	SchemaVersion int                `json:"SchemaVersion"`
	Results       []TrivyResultEntry `json:"Results"`
	Metadata      struct {
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
		return nil, errors.New(errors.CodeInternalError, "docker", "trivy not available", nil)
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

	// Run Trivy scan with JSON output
	args := []string{
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
		} else {
			return result, errors.New(errors.CodeOperationFailed, "trivy", fmt.Sprintf("trivy scan failed: %v", err), err)
		}
	}

	var trivyResult TrivyResult
	if err := json.Unmarshal(output, &trivyResult); err != nil {
		return result, errors.New(errors.CodeOperationFailed, "trivy", fmt.Sprintf("failed to parse trivy output: %v", err), err)
	}

	ts.processResults(&trivyResult, result)

	// Generate remediation steps
	ts.generateRemediationSteps(result)

	result.Success = result.Summary.Critical == 0 && result.Summary.High == 0

	return result, nil
}

// findTrivy locates the Trivy executable
func (ts *TrivyScanner) findTrivy() (string, error) {
	// Check common locations
	paths := []string{}

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
	ts.setMetadataContext(trivyResult, scanResult)
	packageMap := ts.buildPackageMap(trivyResult)

	for _, result := range trivyResult.Results {
		for _, vuln := range result.Vulnerabilities {
			v := ts.convertVulnerability(vuln, result, packageMap)
			scanResult.Vulnerabilities = append(scanResult.Vulnerabilities, v)
			ts.updateSummary(&scanResult.Summary, vuln)
		}
	}

	ts.setContextInfo(trivyResult, scanResult)
}

// setMetadataContext sets metadata context from Trivy results
func (ts *TrivyScanner) setMetadataContext(trivyResult *TrivyResult, scanResult *ScanResult) {
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
}

// buildPackageMap creates a lookup map for packages
func (ts *TrivyScanner) buildPackageMap(trivyResult *TrivyResult) map[string]interface{} {
	packageMap := make(map[string]interface{})
	for _, result := range trivyResult.Results {
		for _, pkg := range result.Packages {
			packageMap[pkg.ID] = pkg
		}
	}
	return packageMap
}

// convertVulnerability converts a Trivy vulnerability to our format
func (ts *TrivyScanner) convertVulnerability(vuln TrivyVulnerability, result TrivyResultEntry, packageMap map[string]interface{}) coresecurity.Vulnerability {
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

	ts.setLayerInfo(&v, vuln)
	ts.setCVSSInfo(&v, vuln)
	ts.setCVSSV3Info(&v, vuln)
	ts.setDataSourceInfo(&v, vuln)
	ts.setPackageInfo(&v, vuln, result, packageMap)

	return v
}

// setLayerInfo sets layer information for the vulnerability
func (ts *TrivyScanner) setLayerInfo(v *coresecurity.Vulnerability, vuln TrivyVulnerability) {
	if vuln.Layer.DiffID != "" {
		v.Layer = vuln.Layer.DiffID
	}
}

// setCVSSInfo sets CVSS information for the vulnerability
func (ts *TrivyScanner) setCVSSInfo(v *coresecurity.Vulnerability, vuln TrivyVulnerability) {
	if len(vuln.CVSS) == 0 {
		return
	}

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

		ts.setVendorSeverity(v, vuln, vendor)
	}
}

// setVendorSeverity sets vendor severity information
func (ts *TrivyScanner) setVendorSeverity(v *coresecurity.Vulnerability, vuln TrivyVulnerability, vendor string) {
	if v.VendorSeverity == nil {
		v.VendorSeverity = make(map[string]string)
	}
	if severityStr, ok := vuln.VendorSeverity[vendor].(string); ok {
		v.VendorSeverity[vendor] = severityStr
	}
}

// setCVSSV3Info sets detailed CVSS v3 information
func (ts *TrivyScanner) setCVSSV3Info(v *coresecurity.Vulnerability, vuln TrivyVulnerability) {
	if vuln.CVSSV31.BaseScore > 0 {
		v.CVSSV3 = ts.buildCVSSV3Info(vuln.CVSSV31)
	} else if vuln.CVSSV30.BaseScore > 0 {
		v.CVSSV3 = ts.buildCVSSV3Info(vuln.CVSSV30)
	}
}

// buildCVSSV3Info builds CVSS v3 info from Trivy CVSS data
func (ts *TrivyScanner) buildCVSSV3Info(cvss TrivyCVSSV3) coresecurity.CVSSV3Info {
	return coresecurity.CVSSV3Info{
		Vector:                cvss.Vector,
		Score:                 cvss.BaseScore,
		ExploitabilityScore:   cvss.ExploitabilityScore,
		ImpactScore:           cvss.ImpactScore,
		AttackVector:          cvss.AttackVector,
		AttackComplexity:      cvss.AttackComplexity,
		PrivilegesRequired:    cvss.PrivilegesRequired,
		UserInteraction:       cvss.UserInteraction,
		Scope:                 cvss.Scope,
		ConfidentialityImpact: cvss.ConfidentialityImpact,
		IntegrityImpact:       cvss.IntegrityImpact,
		AvailabilityImpact:    cvss.AvailabilityImpact,
	}
}

// setDataSourceInfo sets data source information
func (ts *TrivyScanner) setDataSourceInfo(v *coresecurity.Vulnerability, vuln TrivyVulnerability) {
	if vuln.DataSource.ID != "" {
		v.DataSource = coresecurity.VulnDataSource{
			ID:   vuln.DataSource.ID,
			Name: vuln.DataSource.Name,
			URL:  vuln.DataSource.URL,
		}
	}
}

// setPackageInfo sets enhanced package information
func (ts *TrivyScanner) setPackageInfo(v *coresecurity.Vulnerability, vuln TrivyVulnerability, result TrivyResultEntry, packageMap map[string]interface{}) {
	if _, exists := packageMap[vuln.PkgID]; exists {
		v.PkgType = result.Type
		if v.PkgIdentifier.Licenses == nil {
			v.PkgIdentifier.Licenses = make([]string, 0)
		}
	}
}

// updateSummary updates vulnerability summary counts
func (ts *TrivyScanner) updateSummary(summary *coresecurity.VulnerabilitySummary, vuln TrivyVulnerability) {
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

// setContextInfo sets additional context information
func (ts *TrivyScanner) setContextInfo(trivyResult *TrivyResult, scanResult *ScanResult) {
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
