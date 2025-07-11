// Package security provides integration between SBOM generation and vulnerability scanning
package security

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/rs/zerolog"
)

// SBOMVulnerabilityIntegrator integrates SBOM generation with vulnerability scanning
type SBOMVulnerabilityIntegrator struct {
	logger           zerolog.Logger
	sbomGenerator    *SBOMGenerator
	cveDatabase      *CVEDatabase
	policyEngine     *PolicyEngine
	metricsCollector *MetricsCollector
}

// NewSBOMVulnerabilityIntegrator creates a new SBOM vulnerability integrator
func NewSBOMVulnerabilityIntegrator(
	logger zerolog.Logger,
	sbomGenerator *SBOMGenerator,
	cveDatabase *CVEDatabase,
	policyEngine *PolicyEngine,
	metricsCollector *MetricsCollector,
) *SBOMVulnerabilityIntegrator {
	return &SBOMVulnerabilityIntegrator{
		logger:           logger.With().Str("component", "sbom_vuln_integrator").Logger(),
		sbomGenerator:    sbomGenerator,
		cveDatabase:      cveDatabase,
		policyEngine:     policyEngine,
		metricsCollector: metricsCollector,
	}
}

// EnrichedSBOMResult contains SBOM with vulnerability information
type EnrichedSBOMResult struct {
	// Basic information
	Source       string        `json:"source"`
	Format       SBOMFormat    `json:"format"`
	GeneratedAt  time.Time     `json:"generated_at"`
	ScanDuration time.Duration `json:"scan_duration"`

	// SBOM data
	SBOM interface{} `json:"sbom"`

	// Vulnerability data
	VulnerabilityReport VulnerabilityReport `json:"vulnerability_report"`

	// Policy evaluation
	PolicyResults []PolicyEvaluationResult `json:"policy_results,omitempty"`

	// Metrics
	Metrics SBOMScanMetrics `json:"metrics"`
}

// VulnerabilityReport contains vulnerability analysis of SBOM components
type VulnerabilityReport struct {
	Summary                  VulnerabilitySummary         `json:"summary"`
	ComponentVulnerabilities []ComponentVulnerabilityInfo `json:"component_vulnerabilities"`
	CVEEnrichment            map[string]CVEInfo           `json:"cve_enrichment,omitempty"`
}

// ComponentVulnerabilityInfo contains vulnerability information for a specific component
type ComponentVulnerabilityInfo struct {
	Component       Package         `json:"component"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	RiskScore       float64         `json:"risk_score"`
	Recommendations []string        `json:"recommendations"`
}

// SBOMScanMetrics contains metrics for SBOM scanning
type SBOMScanMetrics struct {
	TotalComponents      int            `json:"total_components"`
	VulnerableComponents int            `json:"vulnerable_components"`
	CVEsFound            int            `json:"cves_found"`
	HighRiskComponents   int            `json:"high_risk_components"`
	PolicyViolations     int            `json:"policy_violations"`
	ScanDuration         time.Duration  `json:"scan_duration"`
	EnrichmentDuration   time.Duration  `json:"enrichment_duration"`
	ComponentsByType     map[string]int `json:"components_by_type"`
	VulnsBySeverity      map[string]int `json:"vulns_by_severity"`
}

// ScanWithSBOM performs a comprehensive scan that includes SBOM generation and vulnerability analysis
func (s *SBOMVulnerabilityIntegrator) ScanWithSBOM(
	ctx context.Context,
	source string,
	format SBOMFormat,
	options SBOMScanOptions,
) (*EnrichedSBOMResult, error) {
	startTime := time.Now()

	s.logger.Info().
		Str("source", source).
		Str("format", string(format)).
		Msg("Starting SBOM vulnerability scan")

	// Generate SBOM
	sbomStartTime := time.Now()
	sbom, err := s.sbomGenerator.GenerateSBOM(ctx, source, format)
	if err != nil {
		return nil, mcperrors.New(mcperrors.CodeOperationFailed, "core", "failed to generate SBOM", err)
	}
	sbomDuration := time.Since(sbomStartTime)

	// Extract packages from SBOM for vulnerability analysis
	packages, err := s.extractPackagesFromSBOM(sbom, format)
	if err != nil {
		return nil, mcperrors.New(mcperrors.CodeOperationFailed, "security", "failed to extract packages from SBOM", err)
	}

	enrichStartTime := time.Now()
	vulnReport, err := s.scanPackagesForVulnerabilities(ctx, packages, options)
	if err != nil {
		return nil, mcperrors.New(mcperrors.CodeOperationFailed, "core", "failed to scan packages for vulnerabilities", err)
	}
	enrichDuration := time.Since(enrichStartTime)

	// Evaluate policies if policy engine is available
	var policyResults []PolicyEvaluationResult
	if s.policyEngine != nil {
		policyResults = s.evaluatePolicies(ctx, vulnReport)
	}

	// Calculate metrics
	metrics := s.calculateMetrics(packages, vulnReport, policyResults, sbomDuration, enrichDuration)

	// Record metrics if collector is available
	if s.metricsCollector != nil {
		s.recordMetrics(source, metrics)
	}

	totalDuration := time.Since(startTime)

	result := &EnrichedSBOMResult{
		Source:              source,
		Format:              format,
		GeneratedAt:         startTime,
		ScanDuration:        totalDuration,
		SBOM:                sbom,
		VulnerabilityReport: *vulnReport,
		PolicyResults:       policyResults,
		Metrics:             metrics,
	}

	s.logger.Info().
		Dur("duration", totalDuration).
		Int("components", metrics.TotalComponents).
		Int("vulnerabilities", vulnReport.Summary.Total).
		Int("policy_violations", metrics.PolicyViolations).
		Msg("SBOM vulnerability scan completed")

	return result, nil
}

// SBOMScanOptions provides configuration for SBOM vulnerability scanning
type SBOMScanOptions struct {
	IncludeCVEEnrichment   bool     `json:"include_cve_enrichment"`
	SeverityThreshold      string   `json:"severity_threshold"` // "LOW", "MEDIUM", "HIGH", "CRITICAL"
	IncludeDevDependencies bool     `json:"include_dev_dependencies"`
	ExcludeTypes           []string `json:"exclude_types"`
	MaxConcurrentScans     int      `json:"max_concurrent_scans"`
}

// extractPackagesFromSBOM extracts packages from SBOM for vulnerability analysis
func (s *SBOMVulnerabilityIntegrator) extractPackagesFromSBOM(sbom interface{}, format SBOMFormat) ([]Package, error) {
	var packages []Package

	switch format {
	case SBOMFormatSPDX:
		spdxDoc, ok := sbom.(*SPDXDocument)
		if !ok {
			return nil, fmt.Errorf("invalid SPDX document type")
		}
		packages = s.extractFromSPDX(spdxDoc)

	case SBOMFormatCycloneDX:
		cycloneDXBOM, ok := sbom.(*CycloneDXBOM)
		if !ok {
			return nil, fmt.Errorf("invalid CycloneDX BOM type")
		}
		packages = s.extractFromCycloneDX(cycloneDXBOM)

	default:
		return nil, fmt.Errorf("unsupported SBOM format: %s", format)
	}

	return packages, nil
}

// extractFromSPDX extracts packages from SPDX document
func (s *SBOMVulnerabilityIntegrator) extractFromSPDX(doc *SPDXDocument) []Package {
	var packages []Package

	for _, spdxPkg := range doc.Packages {
		pkg := Package{
			Name:    spdxPkg.Name,
			Version: spdxPkg.Version,
			License: spdxPkg.LicenseDeclared,
		}

		// Extract type and PURL from external references
		for _, ref := range spdxPkg.ExternalRefs {
			switch ref.Type {
			case "purl":
				pkg.PURL = ref.Locator
				// Extract type from PURL
				if purl := parsePURL(ref.Locator); purl != nil {
					pkg.Type = purl.Type
				}
			case "cpe23Type":
				pkg.CPE = ref.Locator
			}
		}

		// Extract checksums
		if len(spdxPkg.Checksums) > 0 {
			pkg.Checksums = make(map[string]string)
			for _, checksum := range spdxPkg.Checksums {
				pkg.Checksums[checksum.Algorithm] = checksum.Value
			}
		}

		packages = append(packages, pkg)
	}

	return packages
}

// extractFromCycloneDX extracts packages from CycloneDX BOM
func (s *SBOMVulnerabilityIntegrator) extractFromCycloneDX(bom *CycloneDXBOM) []Package {
	var packages []Package

	for _, comp := range bom.Components {
		pkg := Package{
			Name:    comp.Name,
			Version: comp.Version,
			Type:    s.mapCycloneDXTypeToPackageType(comp.Type),
			PURL:    comp.PURL,
			CPE:     comp.CPE,
		}

		// Extract license
		if len(comp.Licenses) > 0 && comp.Licenses[0].License != nil {
			if comp.Licenses[0].License.ID != "" {
				pkg.License = comp.Licenses[0].License.ID
			} else {
				pkg.License = comp.Licenses[0].License.Name
			}
		}

		// Extract checksums
		if len(comp.Hashes) > 0 {
			pkg.Checksums = make(map[string]string)
			for _, hash := range comp.Hashes {
				pkg.Checksums[hash.Algorithm] = hash.Content
			}
		}

		// Extract supplier from properties
		for _, prop := range comp.Properties {
			if prop.Name == "supplier" {
				pkg.Supplier = prop.Value
			}
		}

		packages = append(packages, pkg)
	}

	return packages
}

// mapCycloneDXTypeToPackageType maps CycloneDX component type to package type
func (s *SBOMVulnerabilityIntegrator) mapCycloneDXTypeToPackageType(cycloneDXType string) string {
	switch cycloneDXType {
	case "library":
		return "library"
	case "container":
		return "container"
	case "operating-system":
		return "os"
	case "application":
		return "application"
	default:
		return "unknown"
	}
}

// PURLInfo represents parsed PURL information
type PURLInfo struct {
	Type      string
	Namespace string
	Name      string
	Version   string
}

// parsePURL parses a Package URL (PURL) string
func parsePURL(purl string) *PURLInfo {
	// Simple PURL parsing - in production, use a proper PURL library
	// Format: pkg:type/namespace/name@version

	if len(purl) < 4 || purl[:4] != "pkg:" {
		return nil
	}

	// Remove "pkg:" prefix
	remainder := purl[4:]

	// Split by first slash to get type
	parts := strings.SplitN(remainder, "/", 2)
	if len(parts) != 2 {
		return nil
	}

	result := &PURLInfo{
		Type: parts[0],
	}

	// Parse the rest
	nameVersion := parts[1]

	// Split by @ to separate name and version
	if atIndex := strings.LastIndex(nameVersion, "@"); atIndex > 0 {
		result.Name = nameVersion[:atIndex]
		result.Version = nameVersion[atIndex+1:]
	} else {
		result.Name = nameVersion
	}

	// Handle namespace (if name contains slash)
	if slashIndex := strings.Index(result.Name, "/"); slashIndex > 0 {
		result.Namespace = result.Name[:slashIndex]
		result.Name = result.Name[slashIndex+1:]
	}

	return result
}

// scanPackagesForVulnerabilities scans packages for vulnerabilities
func (s *SBOMVulnerabilityIntegrator) scanPackagesForVulnerabilities(
	ctx context.Context,
	packages []Package,
	options SBOMScanOptions,
) (*VulnerabilityReport, error) {
	report := &VulnerabilityReport{
		ComponentVulnerabilities: make([]ComponentVulnerabilityInfo, 0, len(packages)),
		CVEEnrichment:            make(map[string]CVEInfo),
	}

	var totalVulns []Vulnerability

	for _, pkg := range packages {
		// Skip packages based on options
		if s.shouldSkipPackage(pkg, options) {
			continue
		}

		// Simulate vulnerability scanning for this package
		// In a real implementation, this would call external scanners
		vulns := s.findVulnerabilitiesForPackage(ctx, pkg)

		if len(vulns) > 0 {
			// Calculate risk score
			riskScore := s.calculateRiskScore(vulns)

			// Generate recommendations
			recommendations := s.generateRecommendations(pkg, vulns)

			componentInfo := ComponentVulnerabilityInfo{
				Component:       pkg,
				Vulnerabilities: vulns,
				RiskScore:       riskScore,
				Recommendations: recommendations,
			}

			report.ComponentVulnerabilities = append(report.ComponentVulnerabilities, componentInfo)
			totalVulns = append(totalVulns, vulns...)

			// Enrich CVE information if requested
			if options.IncludeCVEEnrichment && s.cveDatabase != nil {
				s.enrichCVEInformation(ctx, vulns, report.CVEEnrichment)
			}
		}
	}

	// Calculate summary
	report.Summary = s.calculateVulnerabilitySummary(totalVulns)

	return report, nil
}

// shouldSkipPackage determines if a package should be skipped based on options
func (s *SBOMVulnerabilityIntegrator) shouldSkipPackage(pkg Package, options SBOMScanOptions) bool {
	// Skip if type is in exclude list
	for _, excludeType := range options.ExcludeTypes {
		if pkg.Type == excludeType {
			return true
		}
	}

	// Add more skip logic based on options
	return false
}

// findVulnerabilitiesForPackage finds vulnerabilities for a specific package
func (s *SBOMVulnerabilityIntegrator) findVulnerabilitiesForPackage(_ context.Context, pkg Package) []Vulnerability {
	// This is a simplified implementation
	// In production, this would integrate with vulnerability databases or scanners

	var vulnerabilities []Vulnerability

	// Simulate finding vulnerabilities based on package name and version
	// This is just for demonstration - real implementation would query vulnerability databases
	if strings.Contains(pkg.Name, "vulnerable") || pkg.Version == "1.0.0" {
		vuln := Vulnerability{
			VulnerabilityID:  fmt.Sprintf("CVE-2023-%04d", len(pkg.Name)%9999),
			InstalledVersion: pkg.Version,
			Severity:         "HIGH",
			Title:            fmt.Sprintf("Vulnerability in %s", pkg.Name),
			Description:      fmt.Sprintf("A security vulnerability was found in %s version %s", pkg.Name, pkg.Version),
			CVSS: CVSSInfo{
				Version: "3.1",
				Score:   7.5,
			},
			References: []string{
				fmt.Sprintf("https://nvd.nist.gov/vuln/detail/CVE-2023-%04d", len(pkg.Name)%9999),
			},
		}
		vulnerabilities = append(vulnerabilities, vuln)
	}

	return vulnerabilities
}

// calculateRiskScore calculates a risk score for a component based on its vulnerabilities
func (s *SBOMVulnerabilityIntegrator) calculateRiskScore(vulns []Vulnerability) float64 {
	if len(vulns) == 0 {
		return 0.0
	}

	var totalScore float64
	var maxScore float64

	for _, vuln := range vulns {
		score := vuln.CVSS.Score
		if score > maxScore {
			maxScore = score
		}
		totalScore += score
	}

	// Risk score is combination of highest score and average score
	avgScore := totalScore / float64(len(vulns))
	riskScore := (maxScore * 0.7) + (avgScore * 0.3)

	return riskScore
}

// generateRecommendations generates security recommendations for a component
func (s *SBOMVulnerabilityIntegrator) generateRecommendations(pkg Package, vulns []Vulnerability) []string {
	var recommendations []string

	if len(vulns) == 0 {
		return recommendations
	}

	// Generic recommendations
	recommendations = append(recommendations, fmt.Sprintf("Update %s to the latest version", pkg.Name))

	// Severity-based recommendations
	for _, vuln := range vulns {
		switch vuln.Severity {
		case "CRITICAL":
			recommendations = append(recommendations, fmt.Sprintf("URGENT: Address %s immediately", vuln.VulnerabilityID))
		case "HIGH":
			recommendations = append(recommendations, fmt.Sprintf("HIGH PRIORITY: Review and fix %s", vuln.VulnerabilityID))
		case "MEDIUM":
			recommendations = append(recommendations, fmt.Sprintf("Consider patching %s in next release cycle", vuln.VulnerabilityID))
		}
	}

	return recommendations
}

// enrichCVEInformation enriches CVE information using the CVE database
func (s *SBOMVulnerabilityIntegrator) enrichCVEInformation(
	ctx context.Context,
	vulns []Vulnerability,
	enrichment map[string]CVEInfo,
) {
	for _, vuln := range vulns {
		if strings.HasPrefix(vuln.VulnerabilityID, "CVE-") {
			if _, exists := enrichment[vuln.VulnerabilityID]; !exists {
				if cveInfo, err := s.cveDatabase.GetCVE(ctx, vuln.VulnerabilityID); err == nil {
					enrichment[vuln.VulnerabilityID] = *cveInfo
				}
			}
		}
	}
}

// calculateVulnerabilitySummary calculates vulnerability summary
func (s *SBOMVulnerabilityIntegrator) calculateVulnerabilitySummary(vulns []Vulnerability) VulnerabilitySummary {
	summary := VulnerabilitySummary{}

	for _, vuln := range vulns {
		summary.Total++

		switch vuln.Severity {
		case "CRITICAL":
			summary.Critical++
		case "HIGH":
			summary.High++
		case "MEDIUM":
			summary.Medium++
		case "LOW":
			summary.Low++
		}

		if vuln.FixedVersion != "" {
			summary.Fixable++
		}
	}

	return summary
}

// evaluatePolicies evaluates security policies against the vulnerability report
func (s *SBOMVulnerabilityIntegrator) evaluatePolicies(ctx context.Context, report *VulnerabilityReport) []PolicyEvaluationResult {
	// Create security scan context for policy evaluation
	scanCtx := &ScanContext{
		VulnSummary:    report.Summary,
		SecretFindings: []ExtendedSecretFinding{}, // No secrets found in SBOM scan
	}

	results, err := s.policyEngine.EvaluatePolicies(ctx, scanCtx)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to evaluate policies")
		return []PolicyEvaluationResult{}
	}

	return results
}

// calculateMetrics calculates scan metrics
func (s *SBOMVulnerabilityIntegrator) calculateMetrics(
	packages []Package,
	report *VulnerabilityReport,
	policyResults []PolicyEvaluationResult,
	sbomDuration, enrichDuration time.Duration,
) SBOMScanMetrics {
	metrics := SBOMScanMetrics{
		TotalComponents:      len(packages),
		VulnerableComponents: len(report.ComponentVulnerabilities),
		CVEsFound:            report.Summary.Total,
		ScanDuration:         sbomDuration,
		EnrichmentDuration:   enrichDuration,
		ComponentsByType:     make(map[string]int),
		VulnsBySeverity:      make(map[string]int),
	}

	// Count components by type
	for _, pkg := range packages {
		metrics.ComponentsByType[pkg.Type]++
	}

	// Count vulnerabilities by severity
	metrics.VulnsBySeverity["critical"] = report.Summary.Critical
	metrics.VulnsBySeverity["high"] = report.Summary.High
	metrics.VulnsBySeverity["medium"] = report.Summary.Medium
	metrics.VulnsBySeverity["low"] = report.Summary.Low

	// Count high-risk components (risk score > 7.0)
	for _, comp := range report.ComponentVulnerabilities {
		if comp.RiskScore > 7.0 {
			metrics.HighRiskComponents++
		}
	}

	// Count policy violations
	for _, result := range policyResults {
		if !result.Passed {
			metrics.PolicyViolations++
		}
	}

	return metrics
}

// recordMetrics records metrics using the metrics collector
func (s *SBOMVulnerabilityIntegrator) recordMetrics(source string, metrics SBOMScanMetrics) {
	// Record SBOM-specific metrics
	s.metricsCollector.RecordScanDuration("sbom", source, "", metrics.ScanDuration)
	s.metricsCollector.RecordScanTotal("sbom", "success")

	// Record component metrics
	for compType, count := range metrics.ComponentsByType {
		// Use a custom metric name for SBOM components
		s.metricsCollector.RecordVulnerabilities(source, fmt.Sprintf("sbom-%s", compType), count)
	}

	// Record vulnerability metrics by severity
	for severity, count := range metrics.VulnsBySeverity {
		s.metricsCollector.RecordVulnerabilitiesBySeverity(source, severity, "sbom", count)
	}
}

// WriteEnrichedResult writes enriched SBOM result to JSON
func (s *SBOMVulnerabilityIntegrator) WriteEnrichedResult(result *EnrichedSBOMResult, filename string) error {
	// nolint:gosec // Filename is controlled by the function caller
	file, err := os.Create(filename)
	if err != nil {
		return mcperrors.New(mcperrors.CodeOperationFailed, "core", "failed to create file", err)
	}
	defer func() { _ = file.Close() }()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	return encoder.Encode(result)
}
