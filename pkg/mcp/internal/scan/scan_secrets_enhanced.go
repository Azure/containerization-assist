package scan

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	coresecurity "github.com/Azure/container-kit/pkg/core/security"
	"github.com/Azure/container-kit/pkg/mcp/internal"
	sessiontypes "github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// AtomicScanSecretsEnhancedArgs defines arguments for enhanced secret scanning
type AtomicScanSecretsEnhancedArgs struct {
	types.BaseToolArgs

	// Target specification
	TargetPath string   `json:"target_path" description:"Path to scan (file or directory)"`
	Recursive  bool     `json:"recursive,omitempty" description:"Recursively scan directories"`
	FileTypes  []string `json:"file_types,omitempty" description:"File types to scan (e.g., .env, .yaml)"`

	// Scanning options
	EnableEntropyDetection bool     `json:"enable_entropy_detection,omitempty" description:"Enable entropy-based detection"`
	MinConfidence          float64  `json:"min_confidence,omitempty" description:"Minimum confidence threshold (0.0-1.0)"`
	CustomPatterns         []string `json:"custom_patterns,omitempty" description:"Additional regex patterns to search for"`
	MaxFileSize            int64    `json:"max_file_size,omitempty" description:"Maximum file size to scan in bytes"`

	// Output options
	VerifyFindings        bool   `json:"verify_findings,omitempty" description:"Attempt to verify if findings are real secrets"`
	ExcludeFalsePositives bool   `json:"exclude_false_positives,omitempty" description:"Exclude likely false positives from results"`
	GenerateReport        bool   `json:"generate_report,omitempty" description:"Generate detailed security report"`
	OutputFormat          string `json:"output_format,omitempty" description:"Output format (json, sarif, markdown)"`
}

// AtomicScanSecretsEnhancedResult represents enhanced secret scan results
type AtomicScanSecretsEnhancedResult struct {
	types.BaseToolResponse
	internal.BaseAIContextResult

	// Scan metadata
	SessionID  string        `json:"session_id"`
	TargetPath string        `json:"target_path"`
	ScanTime   time.Time     `json:"scan_time"`
	Duration   time.Duration `json:"duration"`

	// Results
	Success      bool                    `json:"success"`
	FilesScanned int                     `json:"files_scanned"`
	Findings     []EnhancedSecretFinding `json:"findings"`
	Summary      SecretScanSummary       `json:"summary"`
	RiskScore    int                     `json:"risk_score"`

	// Analysis
	SecurityAnalysis SecurityAnalysis                 `json:"security_analysis"`
	Recommendations  []EnhancedSecurityRecommendation `json:"recommendations"`
	RemediationPlan  *EnhancedSecretRemediationPlan   `json:"remediation_plan,omitempty"`

	// Reports
	GeneratedReport string      `json:"generated_report,omitempty"`
	SARIFReport     interface{} `json:"sarif_report,omitempty"`
}

// EnhancedSecretFinding represents a discovered secret with enhanced metadata
type EnhancedSecretFinding struct {
	coresecurity.SecretFinding
	RemediationSteps []string       `json:"remediation_steps"`
	RelatedFindings  []string       `json:"related_findings,omitempty"`
	RiskAssessment   RiskAssessment `json:"risk_assessment"`
}

// SecretScanSummary provides comprehensive scan summary
type SecretScanSummary struct {
	TotalFindings    int            `json:"total_findings"`
	UniqueSecrets    int            `json:"unique_secrets"`
	BySeverity       map[string]int `json:"by_severity"`
	ByType           map[string]int `json:"by_type"`
	ByFile           map[string]int `json:"by_file"`
	VerifiedFindings int            `json:"verified_findings"`
	FalsePositives   int            `json:"false_positives"`
	TopRiskFiles     []FileRisk     `json:"top_risk_files"`
}

// FileRisk represents risk assessment for a file
type FileRisk struct {
	FilePath    string `json:"file_path"`
	RiskScore   int    `json:"risk_score"`
	SecretCount int    `json:"secret_count"`
	HighestRisk string `json:"highest_risk"`
}

// RiskAssessment provides risk analysis for a finding
type RiskAssessment struct {
	Score      int      `json:"score"`
	Level      string   `json:"level"`
	Factors    []string `json:"factors"`
	Impact     string   `json:"impact"`
	Likelihood string   `json:"likelihood"`
}

// SecurityAnalysis provides overall security analysis
type SecurityAnalysis struct {
	OverallRisk      string            `json:"overall_risk"`
	CriticalFindings int               `json:"critical_findings"`
	ExposureVectors  []string          `json:"exposure_vectors"`
	ComplianceIssues []ComplianceIssue `json:"compliance_issues"`
	SecurityPosture  string            `json:"security_posture"`
}

// ComplianceIssue represents a compliance violation
type ComplianceIssue struct {
	Standard    string `json:"standard"`
	Requirement string `json:"requirement"`
	Violation   string `json:"violation"`
	Severity    string `json:"severity"`
}

// EnhancedSecurityRecommendation represents a security recommendation
type EnhancedSecurityRecommendation struct {
	Priority    int    `json:"priority"` // 1-5 (1 highest)
	Category    string `json:"category"` // patterns, entropy, verification, remediation
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Impact      string `json:"impact"`
	Effort      string `json:"effort"` // low, medium, high
}

// EnhancedSecretRemediationPlan provides a plan to fix discovered secrets
type EnhancedSecretRemediationPlan struct {
	Priority           string              `json:"priority"`
	EstimatedEffort    string              `json:"estimated_effort"`
	Steps              []RemediationStep   `json:"steps"`
	ToolingSuggestions []ToolingSuggestion `json:"tooling_suggestions"`
	PreventionMeasures []string            `json:"prevention_measures"`
}

// RemediationStep represents a step in the remediation plan
type RemediationStep struct {
	Order       int      `json:"order"`
	Action      string   `json:"action"`
	Description string   `json:"description"`
	Commands    []string `json:"commands,omitempty"`
	Resources   []string `json:"resources,omitempty"`
}

// ToolingSuggestion suggests tools to help with remediation
type ToolingSuggestion struct {
	Tool        string `json:"tool"`
	Purpose     string `json:"purpose"`
	Integration string `json:"integration"`
}

// AtomicScanSecretsEnhancedTool implements enhanced secret scanning
type AtomicScanSecretsEnhancedTool struct {
	logger         zerolog.Logger
	sessionManager mcptypes.ToolSessionManager
	scanner        *coresecurity.SecretDiscovery
}

// NewAtomicScanSecretsEnhancedTool creates a new enhanced secret scanning tool
func NewAtomicScanSecretsEnhancedTool(
	sessionManager mcptypes.ToolSessionManager,
	logger zerolog.Logger,
) *AtomicScanSecretsEnhancedTool {
	return &AtomicScanSecretsEnhancedTool{
		logger:         logger.With().Str("tool", "atomic_scan_secrets_enhanced").Logger(),
		sessionManager: sessionManager,
		scanner:        coresecurity.NewSecretDiscovery(logger),
	}
}

// GetName returns the tool name
func (t *AtomicScanSecretsEnhancedTool) GetName() string {
	return "atomic_scan_secrets_enhanced"
}

// GetDescription returns the tool description
func (t *AtomicScanSecretsEnhancedTool) GetDescription() string {
	return "Enhanced secret scanning with pattern and entropy detection"
}

// Execute performs enhanced secret scanning
func (t *AtomicScanSecretsEnhancedTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	typedArgs, ok := args.(AtomicScanSecretsEnhancedArgs)
	if !ok {
		return nil, fmt.Errorf("invalid argument type: expected AtomicScanSecretsEnhancedArgs, got %T", args)
	}

	t.logger.Info().
		Str("session_id", typedArgs.SessionID).
		Str("target_path", typedArgs.TargetPath).
		Bool("recursive", typedArgs.Recursive).
		Msg("Starting enhanced secret scan")

	startTime := time.Now()

	// Get session
	sessionInterface, err := t.sessionManager.GetSession(typedArgs.SessionID)
	if err != nil {
		return t.createErrorResult(typedArgs, startTime, fmt.Errorf("failed to get session: %w", err)), nil
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	// Create base result
	result := &AtomicScanSecretsEnhancedResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_scan_secrets_enhanced", session.SessionID, typedArgs.DryRun),
		BaseAIContextResult: internal.NewBaseAIContextResult("scan", false, 0),
		SessionID:           session.SessionID,
		TargetPath:          typedArgs.TargetPath,
		ScanTime:            startTime,
		Findings:            make([]EnhancedSecretFinding, 0),
		Summary: SecretScanSummary{
			BySeverity: make(map[string]int),
			ByType:     make(map[string]int),
			ByFile:     make(map[string]int),
		},
		Recommendations: make([]EnhancedSecurityRecommendation, 0),
	}

	// Resolve target path
	targetPath := typedArgs.TargetPath
	if targetPath == "" {
		// Default to repository root from session
		if repoPath, ok := session.Metadata["repository_path"].(string); ok {
			targetPath = repoPath
		} else {
			targetPath = "."
		}
	}

	// Make path absolute
	if !filepath.IsAbs(targetPath) {
		if repoPath, ok := session.Metadata["repository_path"].(string); ok {
			targetPath = filepath.Join(repoPath, targetPath)
		}
	}

	result.TargetPath = targetPath

	// Handle dry-run
	if typedArgs.DryRun {
		result.Duration = time.Since(startTime)
		result.Success = true
		result.Recommendations = append(result.Recommendations, EnhancedSecurityRecommendation{
			Priority:    1,
			Category:    "scanning",
			Title:       "Dry Run - Enhanced Secret Scan",
			Description: fmt.Sprintf("Would scan %s for secrets with enhanced detection", targetPath),
			Action:      "Run without dry_run flag to perform actual scan",
			Impact:      "Comprehensive secret detection with entropy analysis",
			Effort:      types.SeverityLow,
		})
		return result, nil
	}

	// Configure scan options
	scanOptions := coresecurity.ScanOptions{
		Recursive:              typedArgs.Recursive,
		FileTypes:              typedArgs.FileTypes,
		MaxFileSize:            typedArgs.MaxFileSize,
		MaxConcurrency:         4,
		EnableEntropyDetection: typedArgs.EnableEntropyDetection,
		MinConfidence:          typedArgs.MinConfidence,
		VerifyFindings:         typedArgs.VerifyFindings,
		ExcludeFalsePositives:  typedArgs.ExcludeFalsePositives,
		CustomPatterns:         typedArgs.CustomPatterns,
	}

	// Set defaults
	if scanOptions.MaxFileSize == 0 {
		scanOptions.MaxFileSize = 10 * 1024 * 1024 // 10MB
	}
	if scanOptions.MinConfidence == 0 {
		scanOptions.MinConfidence = 0.7
	}
	// Enable defaults if not explicitly disabled
	if typedArgs.EnableEntropyDetection {
		scanOptions.EnableEntropyDetection = true
	}
	if typedArgs.VerifyFindings {
		scanOptions.VerifyFindings = true
	}

	// Perform scan
	scanResult, err := t.scanner.ScanDirectory(ctx, targetPath, scanOptions)
	if err != nil {
		t.logger.Error().Err(err).Msg("Secret scan failed")
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Process results
	result.FilesScanned = scanResult.FilesScanned
	result.RiskScore = scanResult.RiskScore
	result.Duration = scanResult.Duration

	// Convert findings to enhanced format
	t.processFindings(scanResult, result)

	// Perform security analysis
	t.analyzeSecurityPosture(result)

	// Generate recommendations
	t.generateRecommendations(result)

	// Create remediation plan if needed
	if len(result.Findings) > 0 {
		result.RemediationPlan = t.createRemediationPlan(result)
	}

	// Generate report if requested
	if typedArgs.GenerateReport {
		result.GeneratedReport = t.generateReport(result, typedArgs.OutputFormat)
	}

	// Determine success
	result.Success = result.RiskScore < 50 && result.Summary.BySeverity["critical"] == 0
	result.IsSuccessful = result.Success
	result.BaseAIContextResult.Duration = result.Duration

	// Set error/warning counts
	result.ErrorCount = result.Summary.BySeverity["critical"] + result.Summary.BySeverity["high"]
	result.WarningCount = result.Summary.BySeverity["medium"] + result.Summary.BySeverity["low"]

	// Update session metadata
	session.Metadata["last_secret_scan"] = map[string]interface{}{
		"timestamp":     result.ScanTime,
		"files_scanned": result.FilesScanned,
		"findings":      result.Summary.TotalFindings,
		"risk_score":    result.RiskScore,
	}

	t.logger.Info().
		Bool("success", result.Success).
		Int("files_scanned", result.FilesScanned).
		Int("findings", len(result.Findings)).
		Int("risk_score", result.RiskScore).
		Dur("duration", result.Duration).
		Msg("Enhanced secret scan completed")

	return result, nil
}

// processFindings converts and enhances findings
func (t *AtomicScanSecretsEnhancedTool) processFindings(scanResult *coresecurity.DiscoveryResult, result *AtomicScanSecretsEnhancedResult) {
	// Copy summary
	result.Summary.TotalFindings = scanResult.Summary.TotalFindings
	result.Summary.UniqueSecrets = scanResult.Summary.UniqueSecrets
	result.Summary.BySeverity = scanResult.Summary.BySeverity
	result.Summary.ByType = scanResult.Summary.ByType
	result.Summary.ByFile = scanResult.Summary.ByFile
	result.Summary.VerifiedFindings = scanResult.Summary.VerifiedFindings
	result.Summary.FalsePositives = scanResult.Summary.FalsePositives

	// Convert findings
	for _, finding := range scanResult.Findings {
		enhanced := EnhancedSecretFinding{
			SecretFinding:    finding,
			RemediationSteps: t.getRemediationSteps(finding),
			RiskAssessment:   t.assessRisk(finding),
		}

		// Find related findings
		enhanced.RelatedFindings = t.findRelatedFindings(finding, scanResult.Findings)

		result.Findings = append(result.Findings, enhanced)
	}

	// Calculate top risk files
	fileRisks := make(map[string]*FileRisk)
	for _, finding := range result.Findings {
		if _, exists := fileRisks[finding.FilePath]; !exists {
			fileRisks[finding.FilePath] = &FileRisk{
				FilePath: finding.FilePath,
			}
		}

		risk := fileRisks[finding.FilePath]
		risk.SecretCount++
		risk.RiskScore += finding.RiskAssessment.Score

		if finding.Severity == "critical" || (risk.HighestRisk != "critical" && finding.Severity == "high") {
			risk.HighestRisk = finding.Severity
		}
	}

	// Get top 5 risk files
	for _, risk := range fileRisks {
		result.Summary.TopRiskFiles = append(result.Summary.TopRiskFiles, *risk)
	}
}

// getRemediationSteps provides specific remediation steps for a finding
func (t *AtomicScanSecretsEnhancedTool) getRemediationSteps(finding coresecurity.SecretFinding) []string {
	steps := []string{}

	// Common first step
	steps = append(steps, fmt.Sprintf("Remove the secret from %s", finding.FilePath))

	// Type-specific steps
	switch finding.SecretType {
	case "aws_access_key", "aws_secret_key":
		steps = append(steps,
			"Rotate the AWS access key immediately in AWS IAM console",
			"Use AWS Secrets Manager or environment variables instead",
			"Configure AWS CLI with proper credential management",
			"Add .aws/credentials to .gitignore",
		)
	case "github_token":
		steps = append(steps,
			"Revoke the token at https://github.com/settings/tokens",
			"Generate a new token with minimal required scopes",
			"Use GitHub Actions secrets for CI/CD",
			"Consider using GitHub Apps instead of personal tokens",
		)
	case "private_key":
		steps = append(steps,
			"Generate a new key pair immediately",
			"Update all systems using the compromised key",
			"Store private keys in secure key management systems",
			"Use SSH agent forwarding instead of copying keys",
		)
	case "database_url":
		steps = append(steps,
			"Change database passwords immediately",
			"Use environment variables for database configuration",
			"Implement connection pooling with encrypted credentials",
			"Consider using cloud-native secret management",
		)
	default:
		steps = append(steps,
			"Rotate or revoke the compromised credential",
			"Use environment variables or secret management tools",
			"Update .gitignore to prevent future commits",
		)
	}

	// Add git cleanup step if in git repo
	steps = append(steps, "If committed to git, use 'git filter-branch' or BFG Repo-Cleaner to remove from history")

	return steps
}

// assessRisk calculates risk assessment for a finding
func (t *AtomicScanSecretsEnhancedTool) assessRisk(finding coresecurity.SecretFinding) RiskAssessment {
	assessment := RiskAssessment{
		Factors: []string{},
	}

	// Base score on severity
	switch finding.Severity {
	case "critical":
		assessment.Score = 90
		assessment.Level = "critical"
	case "high":
		assessment.Score = 70
		assessment.Level = "high"
	case "medium":
		assessment.Score = 40
		assessment.Level = "medium"
	case "low":
		assessment.Score = 20
		assessment.Level = "low"
	default:
		assessment.Score = 10
		assessment.Level = "info"
	}

	// Adjust based on verification
	if finding.Verified {
		assessment.Score += 10
		assessment.Factors = append(assessment.Factors, "Verified as real secret")
		assessment.Likelihood = "confirmed"
	} else if finding.FalsePositive {
		assessment.Score -= 20
		assessment.Factors = append(assessment.Factors, "Likely false positive")
		assessment.Likelihood = "unlikely"
	} else {
		assessment.Likelihood = "probable"
	}

	// Adjust based on secret type
	switch finding.SecretType {
	case "private_key", "aws_secret_key":
		assessment.Score += 10
		assessment.Factors = append(assessment.Factors, "High-value credential type")
		assessment.Impact = "critical"
	case "database_url":
		assessment.Score += 5
		assessment.Factors = append(assessment.Factors, "Database access credential")
		assessment.Impact = "high"
	default:
		assessment.Impact = "medium"
	}

	// Consider entropy
	if finding.Entropy > 5.0 {
		assessment.Factors = append(assessment.Factors, fmt.Sprintf("High entropy: %.2f", finding.Entropy))
	}

	// Cap score
	if assessment.Score > 100 {
		assessment.Score = 100
	}
	if assessment.Score < 0 {
		assessment.Score = 0
	}

	return assessment
}

// findRelatedFindings finds other findings that might be related
func (t *AtomicScanSecretsEnhancedTool) findRelatedFindings(finding coresecurity.SecretFinding, allFindings []coresecurity.SecretFinding) []string {
	related := []string{}

	// Look for findings in the same file
	for _, other := range allFindings {
		if other.ID == finding.ID {
			continue
		}

		// Same file
		if other.FilePath == finding.FilePath {
			related = append(related, other.ID)
			continue
		}

		// Same secret value (indicates reuse)
		if other.Match == finding.Match {
			related = append(related, other.ID)
		}
	}

	return related
}

// analyzeSecurityPosture performs overall security analysis
func (t *AtomicScanSecretsEnhancedTool) analyzeSecurityPosture(result *AtomicScanSecretsEnhancedResult) {
	analysis := &result.SecurityAnalysis

	// Count critical findings
	analysis.CriticalFindings = result.Summary.BySeverity["critical"]

	// Determine overall risk
	if analysis.CriticalFindings > 0 || result.RiskScore > 80 {
		analysis.OverallRisk = "critical"
		analysis.SecurityPosture = "poor"
	} else if result.Summary.BySeverity["high"] > 0 || result.RiskScore > 60 {
		analysis.OverallRisk = "high"
		analysis.SecurityPosture = "needs improvement"
	} else if result.Summary.BySeverity["medium"] > 0 || result.RiskScore > 40 {
		analysis.OverallRisk = "medium"
		analysis.SecurityPosture = "fair"
	} else if result.Summary.TotalFindings > 0 {
		analysis.OverallRisk = "low"
		analysis.SecurityPosture = "good"
	} else {
		analysis.OverallRisk = "minimal"
		analysis.SecurityPosture = "excellent"
	}

	// Identify exposure vectors
	exposureVectors := make(map[string]bool)
	for _, finding := range result.Findings {
		switch finding.SecretType {
		case "aws_access_key", "aws_secret_key":
			exposureVectors["AWS Infrastructure"] = true
		case "github_token":
			exposureVectors["Source Code Repositories"] = true
		case "database_url":
			exposureVectors["Database Systems"] = true
		case "api_key":
			exposureVectors["External APIs"] = true
		case "private_key":
			exposureVectors["SSH/TLS Infrastructure"] = true
		}

		// Check file types for exposure vectors
		if strings.HasSuffix(finding.FilePath, ".env") {
			exposureVectors["Environment Configuration"] = true
		}
		if strings.Contains(finding.FilePath, "config") {
			exposureVectors["Application Configuration"] = true
		}
	}

	for vector := range exposureVectors {
		analysis.ExposureVectors = append(analysis.ExposureVectors, vector)
	}

	// Check compliance issues
	if result.Summary.TotalFindings > 0 {
		analysis.ComplianceIssues = append(analysis.ComplianceIssues, ComplianceIssue{
			Standard:    "PCI-DSS",
			Requirement: "Requirement 8.2.1",
			Violation:   "Credentials stored in plain text",
			Severity:    "high",
		})

		if analysis.CriticalFindings > 0 {
			analysis.ComplianceIssues = append(analysis.ComplianceIssues, ComplianceIssue{
				Standard:    "SOC 2",
				Requirement: "CC6.1",
				Violation:   "Inadequate protection of sensitive information",
				Severity:    "critical",
			})
		}
	}
}

// generateRecommendations creates security recommendations
func (t *AtomicScanSecretsEnhancedTool) generateRecommendations(result *AtomicScanSecretsEnhancedResult) {
	// High priority recommendations based on findings
	if result.Summary.BySeverity["critical"] > 0 {
		result.Recommendations = append(result.Recommendations, EnhancedSecurityRecommendation{
			Priority:    1,
			Category:    "immediate_action",
			Title:       "Rotate Critical Credentials Immediately",
			Description: fmt.Sprintf("Found %d critical secrets that require immediate rotation", result.Summary.BySeverity["critical"]),
			Action:      "Follow the remediation steps for each critical finding to rotate credentials",
			Impact:      "Prevents unauthorized access to critical systems",
			Effort:      types.SeverityHigh,
		})
	}

	// Secret management recommendations
	if result.Summary.TotalFindings > 5 {
		result.Recommendations = append(result.Recommendations, EnhancedSecurityRecommendation{
			Priority:    2,
			Category:    "secret_management",
			Title:       "Implement Centralized Secret Management",
			Description: "Multiple secrets found across the codebase indicate need for centralized management",
			Action:      "Deploy HashiCorp Vault, AWS Secrets Manager, or Kubernetes Secrets",
			Impact:      "Centralized control and audit of all secrets",
			Effort:      types.SeverityMedium,
		})
	}

	// Git history cleanup
	if result.Summary.TotalFindings > 0 {
		result.Recommendations = append(result.Recommendations, EnhancedSecurityRecommendation{
			Priority:    3,
			Category:    "repository_cleanup",
			Title:       "Clean Git History",
			Description: "Secrets may exist in git history even after removal from current files",
			Action:      "Use BFG Repo-Cleaner or git filter-branch to remove secrets from history",
			Impact:      "Prevents exposure through git history",
			Effort:      types.SeverityMedium,
		})
	}

	// Prevention recommendations
	result.Recommendations = append(result.Recommendations, EnhancedSecurityRecommendation{
		Priority:    4,
		Category:    "prevention",
		Title:       "Implement Pre-commit Hooks",
		Description: "Prevent secrets from being committed in the first place",
		Action:      "Install pre-commit hooks with secret scanning (e.g., detect-secrets, gitleaks)",
		Impact:      "Proactive prevention of secret exposure",
		Effort:      types.SeverityLow,
	})

	// CI/CD integration
	result.Recommendations = append(result.Recommendations, EnhancedSecurityRecommendation{
		Priority:    5,
		Category:    "ci_cd",
		Title:       "Add Secret Scanning to CI/CD Pipeline",
		Description: "Automated scanning ensures no secrets slip through code review",
		Action:      "Integrate this secret scanner or tools like TruffleHog into CI/CD",
		Impact:      "Continuous monitoring for secret exposure",
		Effort:      types.SeverityLow,
	})
}

// createRemediationPlan creates a comprehensive remediation plan
func (t *AtomicScanSecretsEnhancedTool) createRemediationPlan(result *AtomicScanSecretsEnhancedResult) *EnhancedSecretRemediationPlan {
	plan := &EnhancedSecretRemediationPlan{
		Steps:              []RemediationStep{},
		ToolingSuggestions: []ToolingSuggestion{},
		PreventionMeasures: []string{},
	}

	// Determine priority and effort
	if result.Summary.BySeverity["critical"] > 0 {
		plan.Priority = "critical"
		plan.EstimatedEffort = "1-2 days"
	} else if result.Summary.BySeverity["high"] > 0 {
		plan.Priority = "high"
		plan.EstimatedEffort = "2-3 days"
	} else {
		plan.Priority = "medium"
		plan.EstimatedEffort = "3-5 days"
	}

	// Create remediation steps
	stepOrder := 1

	// Step 1: Immediate actions
	if result.Summary.BySeverity["critical"] > 0 || result.Summary.BySeverity["high"] > 0 {
		plan.Steps = append(plan.Steps, RemediationStep{
			Order:       stepOrder,
			Action:      "Rotate Compromised Credentials",
			Description: "Immediately rotate all critical and high severity credentials",
			Commands: []string{
				"# AWS: Use AWS CLI or Console to rotate keys",
				"# GitHub: Revoke tokens at https://github.com/settings/tokens",
				"# Databases: Update passwords through admin interfaces",
			},
			Resources: []string{
				"https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_access-keys.html",
				"https://docs.github.com/en/authentication/keeping-your-account-and-data-secure",
			},
		})
		stepOrder++
	}

	// Step 2: Remove secrets from code
	plan.Steps = append(plan.Steps, RemediationStep{
		Order:       stepOrder,
		Action:      "Remove Secrets from Source Code",
		Description: "Remove all discovered secrets from source files",
		Commands: []string{
			"# Review each finding and remove the secret",
			"# Update code to use environment variables or secret management",
			"git add -A && git commit -m 'Remove exposed secrets'",
		},
	})
	stepOrder++

	// Step 3: Clean git history
	if result.Summary.TotalFindings > 0 {
		plan.Steps = append(plan.Steps, RemediationStep{
			Order:       stepOrder,
			Action:      "Clean Git History",
			Description: "Remove secrets from git history to prevent exposure",
			Commands: []string{
				"# Using BFG Repo-Cleaner",
				"bfg --delete-files '*.env' --delete-folders '.aws'",
				"git reflog expire --expire=now --all",
				"git gc --prune=now --aggressive",
			},
			Resources: []string{
				"https://rtyley.github.io/bfg-repo-cleaner/",
			},
		})
		stepOrder++
	}

	// Step 4: Implement secret management
	plan.Steps = append(plan.Steps, RemediationStep{
		Order:       stepOrder,
		Action:      "Implement Secret Management Solution",
		Description: "Deploy and configure a secret management system",
		Commands: []string{
			"# Example: Using Docker secrets",
			"docker secret create db_password -",
			"# Example: Using Kubernetes secrets",
			"kubectl create secret generic app-secrets --from-literal=api-key=<value>",
		},
	})
	stepOrder++

	// Step 5: Set up prevention
	plan.Steps = append(plan.Steps, RemediationStep{
		Order:       stepOrder,
		Action:      "Set Up Prevention Mechanisms",
		Description: "Install pre-commit hooks and CI/CD scanning",
		Commands: []string{
			"# Install pre-commit",
			"pip install pre-commit",
			"# Add secret scanning hooks",
			"pre-commit install",
		},
	})

	// Tooling suggestions
	plan.ToolingSuggestions = []ToolingSuggestion{
		{
			Tool:        "HashiCorp Vault",
			Purpose:     "Enterprise secret management with audit logging",
			Integration: "SDK libraries available for all major languages",
		},
		{
			Tool:        "AWS Secrets Manager",
			Purpose:     "Cloud-native secret management for AWS workloads",
			Integration: "Native integration with AWS services",
		},
		{
			Tool:        "detect-secrets",
			Purpose:     "Pre-commit hook for secret detection",
			Integration: "Integrates with git pre-commit framework",
		},
		{
			Tool:        "SOPS",
			Purpose:     "Encrypted secrets in git repositories",
			Integration: "Encrypts secrets while keeping them in version control",
		},
	}

	// Prevention measures
	plan.PreventionMeasures = []string{
		"Implement mandatory code review for all changes",
		"Use environment variables for all configuration",
		"Enable branch protection rules requiring security scans",
		"Conduct regular security training for developers",
		"Implement least-privilege access controls",
		"Use temporary credentials where possible",
		"Enable audit logging for all secret access",
		"Regular automated scanning of repositories",
	}

	return plan
}

// generateReport generates a formatted report
func (t *AtomicScanSecretsEnhancedTool) generateReport(result *AtomicScanSecretsEnhancedResult, format string) string {
	switch format {
	case "markdown":
		return t.generateMarkdownReport(result)
	case "sarif":
		// SARIF format would be generated here
		return "SARIF report generation not implemented"
	default:
		return t.generateMarkdownReport(result)
	}
}

// generateMarkdownReport creates a markdown formatted report
func (t *AtomicScanSecretsEnhancedTool) generateMarkdownReport(result *AtomicScanSecretsEnhancedResult) string {
	var sb strings.Builder

	sb.WriteString("# Secret Discovery Security Report\n\n")
	sb.WriteString(fmt.Sprintf("**Scan Date:** %s\n", result.ScanTime.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**Target Path:** %s\n", result.TargetPath))
	sb.WriteString(fmt.Sprintf("**Files Scanned:** %d\n", result.FilesScanned))
	sb.WriteString(fmt.Sprintf("**Duration:** %v\n\n", result.Duration))

	// Executive Summary
	sb.WriteString("## Executive Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Risk Score:** %d/100\n", result.RiskScore))
	sb.WriteString(fmt.Sprintf("- **Overall Risk:** %s\n", result.SecurityAnalysis.OverallRisk))
	sb.WriteString(fmt.Sprintf("- **Security Posture:** %s\n", result.SecurityAnalysis.SecurityPosture))
	sb.WriteString(fmt.Sprintf("- **Total Findings:** %d\n", result.Summary.TotalFindings))
	sb.WriteString(fmt.Sprintf("- **Critical Findings:** %d\n", result.Summary.BySeverity["critical"]))
	sb.WriteString(fmt.Sprintf("- **Verified Secrets:** %d\n\n", result.Summary.VerifiedFindings))

	// Findings by Severity
	sb.WriteString("## Findings by Severity\n\n")
	sb.WriteString("| Severity | Count |\n")
	sb.WriteString("|----------|-------|\n")
	for _, sev := range []string{"critical", "high", "medium", "low"} {
		if count, ok := result.Summary.BySeverity[sev]; ok && count > 0 {
			sb.WriteString(fmt.Sprintf("| %s | %d |\n", strings.ToUpper(sev[:1])+strings.ToLower(sev[1:]), count))
		}
	}
	sb.WriteString("\n")

	// Top Risk Files
	if len(result.Summary.TopRiskFiles) > 0 {
		sb.WriteString("## High Risk Files\n\n")
		sb.WriteString("| File | Secrets | Risk Score |\n")
		sb.WriteString("|------|---------|------------|\n")
		for _, file := range result.Summary.TopRiskFiles {
			sb.WriteString(fmt.Sprintf("| %s | %d | %d |\n", file.FilePath, file.SecretCount, file.RiskScore))
		}
		sb.WriteString("\n")
	}

	// Detailed Findings
	if len(result.Findings) > 0 {
		sb.WriteString("## Detailed Findings\n\n")

		// Group by severity
		for _, severity := range []string{"critical", "high", "medium", "low"} {
			findings := t.filterFindingsBySeverity(result.Findings, severity)
			if len(findings) == 0 {
				continue
			}

			sb.WriteString(fmt.Sprintf("### %s Severity\n\n", strings.ToUpper(severity[:1])+strings.ToLower(severity[1:])))

			for _, finding := range findings {
				sb.WriteString(fmt.Sprintf("#### %s\n", finding.SecretType))
				sb.WriteString(fmt.Sprintf("- **File:** %s (line %d)\n", finding.FilePath, finding.LineNumber))
				sb.WriteString(fmt.Sprintf("- **Type:** %s\n", finding.SecretType))
				sb.WriteString(fmt.Sprintf("- **Confidence:** %.2f\n", finding.Confidence))
				if finding.Verified {
					sb.WriteString("- **Status:** ✓ Verified\n")
				}
				if finding.FalsePositive {
					sb.WriteString("- **Status:** ⚠️ Likely False Positive\n")
				}
				sb.WriteString(fmt.Sprintf("- **Risk Assessment:** %s (Score: %d)\n",
					finding.RiskAssessment.Level, finding.RiskAssessment.Score))
				sb.WriteString("\n")
			}
		}
	}

	// Recommendations
	if len(result.Recommendations) > 0 {
		sb.WriteString("## Recommendations\n\n")
		for _, rec := range result.Recommendations {
			sb.WriteString(fmt.Sprintf("### %d. %s\n", rec.Priority, rec.Title))
			sb.WriteString(fmt.Sprintf("%s\n", rec.Description))
			sb.WriteString(fmt.Sprintf("**Action:** %s\n", rec.Action))
			sb.WriteString(fmt.Sprintf("**Impact:** %s\n", rec.Impact))
			sb.WriteString(fmt.Sprintf("**Effort:** %s\n\n", rec.Effort))
		}
	}

	// Remediation Plan
	if result.RemediationPlan != nil {
		sb.WriteString("## Remediation Plan\n\n")
		sb.WriteString(fmt.Sprintf("**Priority:** %s\n", result.RemediationPlan.Priority))
		sb.WriteString(fmt.Sprintf("**Estimated Effort:** %s\n\n", result.RemediationPlan.EstimatedEffort))

		sb.WriteString("### Steps\n\n")
		for _, step := range result.RemediationPlan.Steps {
			sb.WriteString(fmt.Sprintf("%d. **%s**\n", step.Order, step.Action))
			sb.WriteString(fmt.Sprintf("   %s\n", step.Description))
			if len(step.Commands) > 0 {
				sb.WriteString("   ```bash\n")
				for _, cmd := range step.Commands {
					sb.WriteString(fmt.Sprintf("   %s\n", cmd))
				}
				sb.WriteString("   ```\n")
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// filterFindingsBySeverity filters findings by severity level
func (t *AtomicScanSecretsEnhancedTool) filterFindingsBySeverity(findings []EnhancedSecretFinding, severity string) []EnhancedSecretFinding {
	filtered := []EnhancedSecretFinding{}
	for _, finding := range findings {
		if finding.Severity == severity {
			filtered = append(filtered, finding)
		}
	}
	return filtered
}

// createErrorResult creates an error result
func (t *AtomicScanSecretsEnhancedTool) createErrorResult(args AtomicScanSecretsEnhancedArgs, startTime time.Time, err error) *AtomicScanSecretsEnhancedResult {
	return &AtomicScanSecretsEnhancedResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_scan_secrets_enhanced", args.SessionID, args.DryRun),
		BaseAIContextResult: internal.NewBaseAIContextResult("scan", false, time.Since(startTime)),
		SessionID:           args.SessionID,
		TargetPath:          args.TargetPath,
		ScanTime:            startTime,
		Duration:            time.Since(startTime),
		Success:             false,
		Recommendations: []EnhancedSecurityRecommendation{
			{
				Priority:    1,
				Category:    "error",
				Title:       "Scan Failed",
				Description: fmt.Sprintf("Failed to perform secret scan: %v", err),
				Action:      "Check error message and resolve the issue",
				Impact:      "Cannot detect exposed secrets",
				Effort:      types.SeverityLow,
			},
		},
	}
}

// DisableDefaultOptions is a helper method for the args
func (args *AtomicScanSecretsEnhancedArgs) DisableDefaultOptions() bool {
	// Allow disabling defaults if explicitly set to false
	return !args.EnableEntropyDetection && !args.VerifyFindings
}
