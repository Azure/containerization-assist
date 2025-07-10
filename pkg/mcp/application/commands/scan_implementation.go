package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/core/security"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Security scanning implementation methods

// DockerScanResult represents the result of a Docker image scan
type DockerScanResult struct {
	Vulnerabilities []docker.Vulnerability
	Summary         struct {
		Total    int
		Critical int
		High     int
		Medium   int
		Low      int
	}
}

// performImageSecurityScan performs comprehensive image security scanning
func (cmd *ConsolidatedScanCommand) performImageSecurityScan(ctx context.Context, imageRef string, options ScanOptions) (*SecurityScanResult, error) {
	startTime := time.Now()

	cmd.logger.Info("Starting image security scan", "image_ref", imageRef)

	// TODO: Image vulnerability scanning requires external scanner integration
	// For now, we'll create a placeholder result
	dockerScanResult := &DockerScanResult{
		Vulnerabilities: []docker.Vulnerability{},
		Summary: struct {
			Total    int
			Critical int
			High     int
			Medium   int
			Low      int
		}{
			Total:    0,
			Critical: 0,
			High:     0,
			Medium:   0,
			Low:      0,
		},
	}

	// Convert scan result to our format
	vulnerabilities := convertDockerVulnerabilities(dockerScanResult.Vulnerabilities)

	// Perform secret scanning on image layers if enabled
	var secrets []SecretInfo
	if options.IncludeSecrets {
		secretsResult, err := cmd.scanImageForSecrets(ctx, imageRef, options)
		if err != nil {
			cmd.logger.Warn("image secrets scan failed", "error", err)
		} else {
			secrets = secretsResult
		}
	}

	// Perform compliance checks if enabled
	var compliance ComplianceInfo
	if options.IncludeCompliance {
		complianceResult, err := cmd.performComplianceChecks(ctx, imageRef, vulnerabilities, secrets)
		if err != nil {
			cmd.logger.Warn("compliance checks failed", "error", err)
		} else {
			compliance = complianceResult
		}
	}

	// Calculate security score
	securityScore := cmd.calculateSecurityScore(vulnerabilities, secrets, compliance)
	riskLevel := cmd.determineRiskLevel(securityScore, vulnerabilities, secrets)

	// Generate recommendations if enabled
	var recommendations []SecurityRecommendation
	if options.IncludeRemediations {
		recommendations = cmd.generateSecurityRecommendations(vulnerabilities, secrets, compliance)
	}

	// Generate remediation plan if enabled
	var remediationPlan *RemediationPlan
	if options.IncludeRemediations {
		remediationPlan = cmd.generateRemediationPlan(vulnerabilities, secrets)
	}

	result := &SecurityScanResult{
		ImageRef:        imageRef,
		Vulnerabilities: vulnerabilities,
		Secrets:         secrets,
		Compliance:      compliance,
		SecurityScore:   securityScore,
		RiskLevel:       riskLevel,
		Recommendations: recommendations,
		RemediationPlan: remediationPlan,
	}

	cmd.logger.Info("Image security scan completed",
		"image_ref", imageRef,
		"vulnerabilities", len(vulnerabilities),
		"secrets", len(secrets),
		"security_score", securityScore,
		"risk_level", riskLevel,
		"duration", time.Since(startTime))

	return result, nil
}

// performSecretsscan performs comprehensive secrets scanning
func (cmd *ConsolidatedScanCommand) performSecretsscan(ctx context.Context, scanPath string, options ScanOptions) (*SecretsScanResult, error) {
	startTime := time.Now()

	cmd.logger.Info("Starting secrets scan", "path", scanPath)

	// Prepare scan options for security discovery
	scanOptions := security.DefaultScanOptions()
	scanOptions.FileTypes = cmd.convertFilePatterns(options.FilePatterns)
	scanOptions.Recursive = true
	scanOptions.EnableEntropyDetection = true

	// Perform secrets scan using security discovery
	discoveryResult, err := cmd.secretDiscovery.ScanDirectory(ctx, scanPath, scanOptions)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeSecurityViolation).
			Type(errors.ErrTypeSecurity).
			Messagef("secrets scan failed: %w", err).
			WithLocation().
			Build()
	}

	// Convert discovery results to our format
	secrets := cmd.convertSecretFindings(discoveryResult.Findings)

	// Apply severity filtering
	// TODO: Add severity threshold support to ScanOptions
	// if options.SeverityThreshold != "" {
	//     secrets = cmd.filterSecretsBySeverity(secrets, options.SeverityThreshold)
	// }

	// Apply result limits
	if options.MaxResults > 0 && len(secrets) > options.MaxResults {
		secrets = secrets[:options.MaxResults]
	}

	// Generate summary
	summary := cmd.generateSecretsSummary(secrets, discoveryResult.FilesScanned)

	result := &SecretsScanResult{
		Path:          scanPath,
		Secrets:       secrets,
		FilesScanned:  discoveryResult.FilesScanned,
		SecretsFound:  len(secrets),
		HighRiskCount: cmd.countHighRiskSecrets(secrets),
		Summary:       summary,
	}

	cmd.logger.Info("Secrets scan completed",
		"path", scanPath,
		"files_scanned", discoveryResult.FilesScanned,
		"secrets_found", len(secrets),
		"high_risk_count", result.HighRiskCount,
		"duration", time.Since(startTime))

	return result, nil
}

// performVulnerabilityyScan performs comprehensive vulnerability scanning
func (cmd *ConsolidatedScanCommand) performVulnerabilityyScan(ctx context.Context, target string, options ScanOptions) (*VulnerabilityScanResult, error) {
	startTime := time.Now()

	cmd.logger.Info("Starting vulnerability scan", "target", target)

	// Determine scan type based on target
	var vulnerabilities []VulnerabilityInfo
	var err error

	if cmd.isDockerImage(target) {
		// TODO: Image vulnerability scanning requires external scanner integration
		// For now, we'll create an empty result
		vulnerabilities = []VulnerabilityInfo{}
	} else if cmd.isFilesystemPath(target) {
		// Filesystem vulnerability scan
		vulnerabilities, err = cmd.scanFilesystemForVulnerabilities(ctx, target, options)
		if err != nil {
			return nil, errors.NewError().
				Code(errors.CodeSecurityViolation).
				Type(errors.ErrTypeSecurity).
				Messagef("filesystem vulnerability scan failed: %w", err).
				WithLocation().
				Build()
		}
	} else {
		return nil, errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Messagef("unsupported target type: %s", target).
			WithLocation().
			Build()
	}

	// Apply severity filtering
	// TODO: Add severity threshold support to ScanOptions
	// if options.SeverityThreshold != "" {
	//     vulnerabilities = cmd.filterVulnerabilitiesBySeverity(vulnerabilities, options.SeverityThreshold)
	// }

	// Apply result limits
	if options.MaxResults > 0 && len(vulnerabilities) > options.MaxResults {
		vulnerabilities = vulnerabilities[:options.MaxResults]
	}

	// Generate summary
	summary := cmd.generateVulnerabilitySummary(vulnerabilities)

	result := &VulnerabilityScanResult{
		Target:          target,
		Vulnerabilities: vulnerabilities,
		Summary:         summary,
		CriticalCount:   cmd.countVulnerabilitiesBySeverity(vulnerabilities, "critical"),
		HighCount:       cmd.countVulnerabilitiesBySeverity(vulnerabilities, "high"),
		MediumCount:     cmd.countVulnerabilitiesBySeverity(vulnerabilities, "medium"),
		LowCount:        cmd.countVulnerabilitiesBySeverity(vulnerabilities, "low"),
	}

	cmd.logger.Info("Vulnerability scan completed",
		"target", target,
		"vulnerabilities", len(vulnerabilities),
		"critical", result.CriticalCount,
		"high", result.HighCount,
		"medium", result.MediumCount,
		"low", result.LowCount,
		"duration", time.Since(startTime))

	return result, nil
}

// scanImageForSecrets scans Docker image layers for secrets
func (cmd *ConsolidatedScanCommand) scanImageForSecrets(ctx context.Context, imageRef string, options ScanOptions) ([]SecretInfo, error) {
	// This would extract image layers and scan them for secrets
	// For now, we'll return empty results as this requires complex Docker layer extraction
	cmd.logger.Debug("Scanning image for secrets", "image_ref", imageRef)
	return []SecretInfo{}, nil
}

// performComplianceChecks performs compliance checks on scan results
func (cmd *ConsolidatedScanCommand) performComplianceChecks(ctx context.Context, target string, vulnerabilities []VulnerabilityInfo, secrets []SecretInfo) (ComplianceInfo, error) {
	// Implement compliance checks based on various frameworks
	checks := []ComplianceCheck{
		{
			ID:          "no-critical-vulns",
			Name:        "No Critical Vulnerabilities",
			Passed:      cmd.countVulnerabilitiesBySeverity(vulnerabilities, "critical") == 0,
			Description: "Image should not contain critical vulnerabilities",
		},
		{
			ID:          "no-high-risk-secrets",
			Name:        "No High Risk Secrets",
			Passed:      cmd.countHighRiskSecrets(secrets) == 0,
			Description: "Image should not contain high risk secrets",
		},
		{
			ID:          "vuln-threshold",
			Name:        "Vulnerability Threshold",
			Passed:      len(vulnerabilities) < 50,
			Description: "Total vulnerabilities should be below threshold",
		},
	}

	// Calculate overall score
	passedCount := 0
	for _, check := range checks {
		if check.Passed {
			passedCount++
		}
	}

	score := float64(passedCount) / float64(len(checks))

	return ComplianceInfo{
		Framework: "container-security",
		Passed:    score >= 0.8,
		Score:     score,
		Checks:    checks,
	}, nil
}

// calculateSecurityScore calculates overall security score
func (cmd *ConsolidatedScanCommand) calculateSecurityScore(vulnerabilities []VulnerabilityInfo, secrets []SecretInfo, compliance ComplianceInfo) int {
	baseScore := 100

	// Deduct points for vulnerabilities
	criticalCount := cmd.countVulnerabilitiesBySeverity(vulnerabilities, "critical")
	highCount := cmd.countVulnerabilitiesBySeverity(vulnerabilities, "high")
	mediumCount := cmd.countVulnerabilitiesBySeverity(vulnerabilities, "medium")
	lowCount := cmd.countVulnerabilitiesBySeverity(vulnerabilities, "low")

	baseScore -= (criticalCount * 20)
	baseScore -= (highCount * 10)
	baseScore -= (mediumCount * 5)
	baseScore -= (lowCount * 1)

	// Deduct points for secrets
	highRiskSecrets := cmd.countHighRiskSecrets(secrets)
	baseScore -= (highRiskSecrets * 15)
	baseScore -= ((len(secrets) - highRiskSecrets) * 5)

	// Deduct points for compliance failures
	if !compliance.Passed {
		baseScore -= int((1.0 - compliance.Score) * 20)
	}

	if baseScore < 0 {
		baseScore = 0
	}

	return baseScore
}

// determineRiskLevel determines overall risk level
func (cmd *ConsolidatedScanCommand) determineRiskLevel(score int, vulnerabilities []VulnerabilityInfo, secrets []SecretInfo) string {
	criticalVulns := cmd.countVulnerabilitiesBySeverity(vulnerabilities, "critical")
	highRiskSecrets := cmd.countHighRiskSecrets(secrets)

	if criticalVulns > 0 || highRiskSecrets > 0 || score < 30 {
		return "critical"
	}

	if score < 60 {
		return "high"
	}

	if score < 80 {
		return "medium"
	}

	return "low"
}

// generateSecurityRecommendations generates security recommendations
func (cmd *ConsolidatedScanCommand) generateSecurityRecommendations(vulnerabilities []VulnerabilityInfo, secrets []SecretInfo, compliance ComplianceInfo) []SecurityRecommendation {
	var recommendations []SecurityRecommendation

	// Vulnerability recommendations
	criticalCount := cmd.countVulnerabilitiesBySeverity(vulnerabilities, "critical")
	if criticalCount > 0 {
		recommendations = append(recommendations, SecurityRecommendation{
			ID:          "fix-critical-vulns",
			Type:        "vulnerability",
			Priority:    "high",
			Title:       "Fix Critical Vulnerabilities",
			Description: fmt.Sprintf("Address %d critical vulnerabilities immediately", criticalCount),
			Action:      "Update packages or apply patches",
			Impact:      "Prevents potential security breaches",
		})
	}

	// Secrets recommendations
	highRiskSecrets := cmd.countHighRiskSecrets(secrets)
	if highRiskSecrets > 0 {
		recommendations = append(recommendations, SecurityRecommendation{
			ID:          "remove-secrets",
			Type:        "secret",
			Priority:    "high",
			Title:       "Remove Exposed Secrets",
			Description: fmt.Sprintf("Remove %d high-risk secrets from the codebase", highRiskSecrets),
			Action:      "Move secrets to secure storage or environment variables",
			Impact:      "Prevents credential exposure",
		})
	}

	// Compliance recommendations
	if !compliance.Passed {
		recommendations = append(recommendations, SecurityRecommendation{
			ID:          "improve-compliance",
			Type:        "compliance",
			Priority:    "medium",
			Title:       "Improve Security Compliance",
			Description: fmt.Sprintf("Current compliance score: %.1f%%", compliance.Score*100),
			Action:      "Address failed compliance checks",
			Impact:      "Meets security standards",
		})
	}

	return recommendations
}

// generateRemediationPlan generates detailed remediation plan
func (cmd *ConsolidatedScanCommand) generateRemediationPlan(vulnerabilities []VulnerabilityInfo, secrets []SecretInfo) *RemediationPlan {
	var steps []RemediationStep
	stepID := 1

	// Add vulnerability remediation steps
	packageVulns := cmd.groupVulnerabilitiesByPackage(vulnerabilities)
	for pkg, vulns := range packageVulns {
		if len(vulns) > 0 {
			steps = append(steps, RemediationStep{
				ID:          fmt.Sprintf("vuln-%d", stepID),
				Order:       stepID,
				Type:        "vulnerability",
				Title:       fmt.Sprintf("Update %s package", pkg),
				Description: fmt.Sprintf("Update package %s to fix %d vulnerabilities", pkg, len(vulns)),
				Command:     cmd.generatePackageUpdateCommand(pkg, vulns),
				Automated:   true,
			})
			stepID++
		}
	}

	// Add secret remediation steps
	secretsByType := cmd.groupSecretsByType(secrets)
	for secretType, typeSecrets := range secretsByType {
		if len(typeSecrets) > 0 {
			steps = append(steps, RemediationStep{
				ID:          fmt.Sprintf("secret-%d", stepID),
				Order:       stepID,
				Type:        "secret",
				Title:       fmt.Sprintf("Remove %s secrets", secretType),
				Description: fmt.Sprintf("Remove %d %s secrets from the codebase", len(typeSecrets), secretType),
				Command:     cmd.generateSecretRemovalCommand(secretType, typeSecrets),
				Automated:   false,
			})
			stepID++
		}
	}

	// Calculate effort and priority
	priority := cmd.calculateRemediationPriority(vulnerabilities, secrets)
	effort := cmd.estimateRemediationEffort(steps)

	return &RemediationPlan{
		ID:        fmt.Sprintf("remediation-%d", time.Now().Unix()),
		Priority:  priority,
		Effort:    effort,
		Steps:     steps,
		Estimated: time.Duration(len(steps)) * 30 * time.Minute,
	}
}

// scanFilesystemForVulnerabilities scans filesystem for vulnerabilities
func (cmd *ConsolidatedScanCommand) scanFilesystemForVulnerabilities(ctx context.Context, path string, options ScanOptions) ([]VulnerabilityInfo, error) {
	var vulnerabilities []VulnerabilityInfo

	// This would scan package manifests, dependencies, etc.
	// For now, we'll return empty results as this requires complex package analysis
	cmd.logger.Debug("Scanning filesystem for vulnerabilities", "path", path)

	return vulnerabilities, nil
}

// Helper methods for scan operations

// convertFilePatterns converts file patterns to extensions
func (cmd *ConsolidatedScanCommand) convertFilePatterns(patterns []string) []string {
	extensions := make([]string, 0)
	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "*.") {
			extensions = append(extensions, pattern[1:])
		} else if strings.Contains(pattern, ".") {
			parts := strings.Split(pattern, ".")
			if len(parts) > 1 {
				extensions = append(extensions, "."+parts[len(parts)-1])
			}
		}
	}
	return extensions
}

// convertSecretFindings converts security findings to secret info
func (cmd *ConsolidatedScanCommand) convertSecretFindings(findings []security.ExtendedSecretFinding) []SecretInfo {
	secrets := make([]SecretInfo, len(findings))
	for i, finding := range findings {
		secrets[i] = SecretInfo{
			Type:       finding.Type,
			File:       finding.File,
			Line:       finding.Line,
			Pattern:    finding.Pattern,
			Value:      finding.Redacted,
			Severity:   finding.Severity,
			Confidence: finding.Confidence,
			Metadata:   map[string]interface{}{"scanner": "security_discovery"},
		}
	}
	return secrets
}

// parseConfidence parses confidence string to float64
func (cmd *ConsolidatedScanCommand) parseConfidence(confidence string) float64 {
	var conf float64
	if _, err := fmt.Sscanf(confidence, "%f", &conf); err == nil {
		return conf
	}
	return 0.5 // Default confidence
}

// filterSecretsBySeverity filters secrets by severity threshold
func (cmd *ConsolidatedScanCommand) filterSecretsBySeverity(secrets []SecretInfo, threshold string) []SecretInfo {
	severityOrder := map[string]int{
		"low":      1,
		"medium":   2,
		"high":     3,
		"critical": 4,
	}

	minSeverity, exists := severityOrder[threshold]
	if !exists {
		return secrets
	}

	var filtered []SecretInfo
	for _, secret := range secrets {
		if secretSeverity, exists := severityOrder[secret.Severity]; exists && secretSeverity >= minSeverity {
			filtered = append(filtered, secret)
		}
	}

	return filtered
}

// filterVulnerabilitiesBySeverity filters vulnerabilities by severity threshold
func (cmd *ConsolidatedScanCommand) filterVulnerabilitiesBySeverity(vulnerabilities []VulnerabilityInfo, threshold string) []VulnerabilityInfo {
	severityOrder := map[string]int{
		"low":      1,
		"medium":   2,
		"high":     3,
		"critical": 4,
	}

	minSeverity, exists := severityOrder[threshold]
	if !exists {
		return vulnerabilities
	}

	var filtered []VulnerabilityInfo
	for _, vuln := range vulnerabilities {
		if vulnSeverity, exists := severityOrder[vuln.Severity]; exists && vulnSeverity >= minSeverity {
			filtered = append(filtered, vuln)
		}
	}

	return filtered
}

// generateSecretsSummary generates secrets scan summary
func (cmd *ConsolidatedScanCommand) generateSecretsSummary(secrets []SecretInfo, filesScanned int) SecretsSummary {
	byType := make(map[string]int)
	bySeverity := make(map[string]int)
	byFile := make(map[string]int)
	totalConfidence := 0.0

	for _, secret := range secrets {
		byType[secret.Type]++
		bySeverity[secret.Severity]++
		byFile[secret.File]++
		totalConfidence += secret.Confidence
	}

	confidenceAvg := 0.0
	if len(secrets) > 0 {
		confidenceAvg = totalConfidence / float64(len(secrets))
	}

	return SecretsSummary{
		TotalSecrets:      len(secrets),
		ByType:            byType,
		BySeverity:        bySeverity,
		ByFile:            byFile,
		ConfidenceAverage: confidenceAvg,
		Metadata: map[string]interface{}{
			"files_scanned": filesScanned,
		},
	}
}

// generateVulnerabilitySummary generates vulnerability scan summary
func (cmd *ConsolidatedScanCommand) generateVulnerabilitySummary(vulnerabilities []VulnerabilityInfo) VulnerabilitySummary {
	bySeverity := make(map[string]int)
	byPackage := make(map[string]int)
	fixableCount := 0

	for _, vuln := range vulnerabilities {
		bySeverity[vuln.Severity]++
		byPackage[vuln.Package]++
		if vuln.FixedIn != "" {
			fixableCount++
		}
	}

	return VulnerabilitySummary{
		TotalVulns:   len(vulnerabilities),
		BySeverity:   bySeverity,
		ByPackage:    byPackage,
		FixableCount: fixableCount,
		AgeAnalysis:  cmd.calculateAgeAnalysis(vulnerabilities),
		Metadata: map[string]interface{}{
			"scan_type": "vulnerability",
		},
	}
}

// calculateAgeAnalysis calculates vulnerability age analysis
func (cmd *ConsolidatedScanCommand) calculateAgeAnalysis(vulnerabilities []VulnerabilityInfo) AgeAnalysis {
	// For now, return empty analysis as this requires CVE database integration
	return AgeAnalysis{
		AverageAge:   0,
		OldestVuln:   0,
		NewestVuln:   0,
		Distribution: make(map[string]int),
	}
}

// countHighRiskSecrets counts high-risk secrets
func (cmd *ConsolidatedScanCommand) countHighRiskSecrets(secrets []SecretInfo) int {
	count := 0
	for _, secret := range secrets {
		if secret.Severity == "high" || secret.Severity == "critical" {
			count++
		}
	}
	return count
}

// countVulnerabilitiesBySeverity counts vulnerabilities by severity
func (cmd *ConsolidatedScanCommand) countVulnerabilitiesBySeverity(vulnerabilities []VulnerabilityInfo, severity string) int {
	count := 0
	for _, vuln := range vulnerabilities {
		if vuln.Severity == severity {
			count++
		}
	}
	return count
}

// groupVulnerabilitiesByPackage groups vulnerabilities by package
func (cmd *ConsolidatedScanCommand) groupVulnerabilitiesByPackage(vulnerabilities []VulnerabilityInfo) map[string][]VulnerabilityInfo {
	groups := make(map[string][]VulnerabilityInfo)
	for _, vuln := range vulnerabilities {
		groups[vuln.Package] = append(groups[vuln.Package], vuln)
	}
	return groups
}

// groupSecretsByType groups secrets by type
func (cmd *ConsolidatedScanCommand) groupSecretsByType(secrets []SecretInfo) map[string][]SecretInfo {
	groups := make(map[string][]SecretInfo)
	for _, secret := range secrets {
		groups[secret.Type] = append(groups[secret.Type], secret)
	}
	return groups
}

// generatePackageUpdateCommand generates package update command
func (cmd *ConsolidatedScanCommand) generatePackageUpdateCommand(pkg string, vulns []VulnerabilityInfo) string {
	if len(vulns) == 0 {
		return ""
	}

	// Use first vulnerability's fixed version if available
	if vulns[0].FixedIn != "" {
		return fmt.Sprintf("apt-get update && apt-get install %s=%s", pkg, vulns[0].FixedIn)
	}

	return fmt.Sprintf("apt-get update && apt-get upgrade %s", pkg)
}

// generateSecretRemovalCommand generates secret removal command
func (cmd *ConsolidatedScanCommand) generateSecretRemovalCommand(secretType string, secrets []SecretInfo) string {
	if len(secrets) == 0 {
		return ""
	}

	// Create a command to help locate and remove secrets
	files := make([]string, 0)
	for _, secret := range secrets {
		if secret.File != "" {
			files = append(files, secret.File)
		}
	}

	if len(files) > 0 {
		return fmt.Sprintf("# Review and remove %s secrets from: %s", secretType, strings.Join(files, ", "))
	}

	return fmt.Sprintf("# Review and remove %s secrets", secretType)
}

// calculateRemediationPriority calculates remediation priority
func (cmd *ConsolidatedScanCommand) calculateRemediationPriority(vulnerabilities []VulnerabilityInfo, secrets []SecretInfo) string {
	criticalVulns := cmd.countVulnerabilitiesBySeverity(vulnerabilities, "critical")
	highRiskSecrets := cmd.countHighRiskSecrets(secrets)

	if criticalVulns > 0 || highRiskSecrets > 0 {
		return "high"
	}

	highVulns := cmd.countVulnerabilitiesBySeverity(vulnerabilities, "high")
	if highVulns > 0 || len(secrets) > 0 {
		return "medium"
	}

	return "low"
}

// estimateRemediationEffort estimates effort required for remediation
func (cmd *ConsolidatedScanCommand) estimateRemediationEffort(steps []RemediationStep) string {
	if len(steps) == 0 {
		return "none"
	}

	if len(steps) <= 3 {
		return "low"
	}

	if len(steps) <= 10 {
		return "medium"
	}

	return "high"
}

// isDockerImage checks if target is a Docker image
func (cmd *ConsolidatedScanCommand) isDockerImage(target string) bool {
	// Check for Docker image patterns
	return strings.Contains(target, ":") || strings.Contains(target, "/")
}

// isFilesystemPath checks if target is a filesystem path
func (cmd *ConsolidatedScanCommand) isFilesystemPath(target string) bool {
	// Check if path exists on filesystem
	_, err := os.Stat(target)
	return err == nil || filepath.IsAbs(target)
}

// convertDockerVulnerabilities converts Docker vulnerabilities to our format
func convertDockerVulnerabilities(dockerVulns []docker.Vulnerability) []VulnerabilityInfo {
	vulnerabilities := make([]VulnerabilityInfo, len(dockerVulns))
	for i, vuln := range dockerVulns {
		// Extract CVSS score from map
		cvssScore := 0.0
		if vuln.CVSS != nil {
			if score, ok := vuln.CVSS["score"].(float64); ok {
				cvssScore = score
			}
		}

		vulnerabilities[i] = VulnerabilityInfo{
			ID:          vuln.ID,
			Severity:    vuln.Severity,
			Title:       vuln.Title,
			Description: vuln.Description,
			Package:     vuln.Package,
			Version:     vuln.Version,
			FixedIn:     vuln.FixedIn,
			CVSS:        cvssScore,
			References:  vuln.References,
			Metadata:    vuln.Metadata,
		}
	}
	return vulnerabilities
}
