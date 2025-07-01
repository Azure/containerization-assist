package scan

import (
	"strings"

	"github.com/rs/zerolog"
)

// ResultProcessor handles processing and analysis of scan results
type ResultProcessor struct {
	logger zerolog.Logger
}

// NewResultProcessor creates a new result processor
func NewResultProcessor(logger zerolog.Logger) *ResultProcessor {
	return &ResultProcessor{
		logger: logger,
	}
}

// CalculateSeverityBreakdown calculates the breakdown of secrets by severity
func (rp *ResultProcessor) CalculateSeverityBreakdown(secrets []ScannedSecret) map[string]int {
	breakdown := make(map[string]int)

	for _, secret := range secrets {
		breakdown[secret.Severity]++
	}

	return breakdown
}

// CalculateSecurityScore calculates an overall security score based on found secrets
func (rp *ResultProcessor) CalculateSecurityScore(secrets []ScannedSecret) int {
	if len(secrets) == 0 {
		return 100
	}

	score := 100
	for _, secret := range secrets {
		switch secret.Severity {
		case "critical":
			score -= 25
		case "high":
			score -= 15
		case "medium":
			score -= 8
		case "low":
			score -= 3
		}
	}

	if score < 0 {
		score = 0
	}

	return score
}

// DetermineRiskLevel determines the overall risk level based on security score
func (rp *ResultProcessor) DetermineRiskLevel(score int, secrets []ScannedSecret) string {
	if score >= 80 {
		return "low"
	}
	if score >= 60 {
		return "medium"
	}
	if score >= 30 {
		return "high"
	}

	return "critical"
}

// GenerateRecommendations generates actionable recommendations based on scan results
func (rp *ResultProcessor) GenerateRecommendations(secrets []ScannedSecret, args AtomicScanSecretsArgs) []string {
	var recommendations []string

	if len(secrets) == 0 {
		recommendations = append(recommendations, "✅ No secrets detected in scanned files")
		recommendations = append(recommendations, "Continue following security best practices")
		return recommendations
	}

	// Count by severity
	severityCount := rp.CalculateSeverityBreakdown(secrets)

	if severityCount["critical"] > 0 {
		recommendations = append(recommendations, "🔴 CRITICAL: Remove or secure critical secrets immediately")
		recommendations = append(recommendations, "Move critical secrets to secure secret management systems")
	}

	if severityCount["high"] > 0 {
		recommendations = append(recommendations, "🟠 HIGH: Review and secure high-severity secrets")
	}

	if severityCount["medium"] > 0 || severityCount["low"] > 0 {
		recommendations = append(recommendations, "🟡 Review and potentially secure medium/low severity secrets")
	}

	// File-type specific recommendations
	hasDockerfiles := false
	hasKubernetesFiles := false
	hasEnvFiles := false

	for _, secret := range secrets {
		if strings.Contains(strings.ToLower(secret.File), "dockerfile") {
			hasDockerfiles = true
		}
		if strings.Contains(strings.ToLower(secret.File), ".yaml") || strings.Contains(strings.ToLower(secret.File), ".yml") {
			hasKubernetesFiles = true
		}
		if strings.Contains(strings.ToLower(secret.File), ".env") {
			hasEnvFiles = true
		}
	}

	if hasDockerfiles {
		recommendations = append(recommendations, "📦 Use Docker build args for secrets in Dockerfiles")
		recommendations = append(recommendations, "Consider using multi-stage builds to avoid exposing secrets")
	}

	if hasKubernetesFiles {
		recommendations = append(recommendations, "☸️ Use Kubernetes Secrets or ConfigMaps for sensitive values")
		recommendations = append(recommendations, "Consider external secret management (e.g., HashiCorp Vault, Azure Key Vault)")
	}

	if hasEnvFiles {
		recommendations = append(recommendations, "🔧 Add .env files to .gitignore")
		recommendations = append(recommendations, "Use .env.example files with placeholder values")
	}

	// General recommendations
	recommendations = append(recommendations, "🔐 Implement secret scanning in CI/CD pipelines")
	recommendations = append(recommendations, "📋 Create incident response plan for exposed secrets")
	recommendations = append(recommendations, "🔄 Rotate any potentially exposed secrets")

	if args.GenerateSecrets {
		recommendations = append(recommendations, "📝 Review generated Kubernetes Secret manifests")
		recommendations = append(recommendations, "Apply generated secrets to your cluster securely")
	}

	return recommendations
}

// GenerateScanContext creates contextual information about the scan
func (rp *ResultProcessor) GenerateScanContext(secrets []ScannedSecret, fileResults []FileSecretScanResult, args AtomicScanSecretsArgs) map[string]interface{} {
	context := make(map[string]interface{})

	// File type analysis
	fileTypes := make(map[string]int)
	for _, result := range fileResults {
		fileTypes[result.FileType]++
	}
	context["file_types_scanned"] = fileTypes

	// Secret type analysis
	secretTypes := make(map[string]int)
	for _, secret := range secrets {
		secretTypes[secret.Type]++
	}
	context["secret_types_found"] = secretTypes

	// Scan configuration
	context["scan_configuration"] = map[string]interface{}{
		"scan_dockerfiles": args.ScanDockerfiles,
		"scan_manifests":   args.ScanManifests,
		"scan_source_code": args.ScanSourceCode,
		"scan_env_files":   args.ScanEnvFiles,
	}

	// Files with secrets
	filesWithSecrets := make([]string, 0)
	for _, result := range fileResults {
		if result.SecretsFound > 0 {
			filesWithSecrets = append(filesWithSecrets, result.FilePath)
		}
	}
	context["files_with_secrets"] = filesWithSecrets

	// Risk assessment
	riskFactors := rp.identifyRiskFactors(secrets, fileResults)
	context["risk_factors"] = riskFactors

	return context
}

// identifyRiskFactors identifies specific risk factors in the scan results
func (rp *ResultProcessor) identifyRiskFactors(secrets []ScannedSecret, fileResults []FileSecretScanResult) []string {
	var riskFactors []string

	// Check for multiple secrets in same file
	fileSecretCount := make(map[string]int)
	for _, secret := range secrets {
		fileSecretCount[secret.File]++
	}

	for file, count := range fileSecretCount {
		if count > 3 {
			riskFactors = append(riskFactors, "Multiple secrets found in "+file)
		}
	}

	// Check for production-related secrets
	hasProductionSecrets := false
	for _, secret := range secrets {
		if strings.Contains(strings.ToLower(secret.Pattern), "prod") ||
			strings.Contains(strings.ToLower(secret.Pattern), "production") {
			hasProductionSecrets = true
			break
		}
	}
	if hasProductionSecrets {
		riskFactors = append(riskFactors, "Production secrets detected")
	}

	// Check for database credentials
	hasDatabaseCreds := false
	for _, secret := range secrets {
		if strings.Contains(strings.ToLower(secret.Type), "database") ||
			strings.Contains(strings.ToLower(secret.Pattern), "db") {
			hasDatabaseCreds = true
			break
		}
	}
	if hasDatabaseCreds {
		riskFactors = append(riskFactors, "Database credentials found")
	}

	// Check for API keys
	hasAPIKeys := false
	for _, secret := range secrets {
		if strings.Contains(strings.ToLower(secret.Type), "api") {
			hasAPIKeys = true
			break
		}
	}
	if hasAPIKeys {
		riskFactors = append(riskFactors, "API keys detected")
	}

	// Check for secrets in version-controlled files
	hasVCSecrets := false
	for _, secret := range secrets {
		if !strings.Contains(secret.File, ".env") &&
			!strings.Contains(secret.File, "temp") &&
			!strings.Contains(secret.File, "tmp") {
			hasVCSecrets = true
			break
		}
	}
	if hasVCSecrets {
		riskFactors = append(riskFactors, "Secrets in version-controlled files")
	}

	if len(riskFactors) == 0 {
		riskFactors = append(riskFactors, "No major risk factors identified")
	}

	return riskFactors
}
